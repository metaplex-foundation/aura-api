package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/go-pg/pg/v10"
	"github.com/mr-tron/base58"
	"github.com/shopspring/decimal"

	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/log"
)

var metaplexToken = solana.MustPublicKeyFromBase58("METAewgxyPbgwsseH8T16a39CQ5VyVxZi9zXiDPY18m")

const (
	paymentLabel = "Aura Gateway"
)

const (
	expectedMintAccountIndex        = 1
	expectedDestinationAccountIndex = 2

	getSignaturesLimit = 1000
)

type paymentsWatcher struct {
	rpcClient *rpc.Client
	pgStorage *postgres.Storage

	paymentRecipient                       solana.PublicKey
	paymentRecipientAssociatedTokenAddress solana.PublicKey
	lastProcessedSignature                 solana.Signature
	unpaidReferences                       map[string]struct{}
}

func newPaymentsWatcher(rpcAddress string, pgStorage *postgres.Storage, paymentRecipient solana.PublicKey) (p paymentsWatcher, err error) {
	paymentRecipientAssociatedTokenAddress, _, err := solana.FindAssociatedTokenAddress(paymentRecipient, metaplexToken)
	if err != nil {
		return p, fmt.Errorf("FindAssociatedTokenAddress: %s", err)
	}
	return paymentsWatcher{
		rpcClient:                              rpc.New(rpcAddress),
		pgStorage:                              pgStorage,
		paymentRecipient:                       paymentRecipient,
		paymentRecipientAssociatedTokenAddress: paymentRecipientAssociatedTokenAddress,
	}, nil
}

func (p *paymentsWatcher) generateSolanaPayPaymentLink(ctx context.Context, amount, paymentType string, userID int64) (string, error) {
	referenceKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		return "", fmt.Errorf("NewRandomPrivateKey: %w", err)
	}
	u := url.URL{
		Scheme: "solana",
		Opaque: p.paymentRecipient.String(),
	}
	q := u.Query()
	q.Set("amount", amount)
	q.Set("spl-token", metaplexToken.String())
	q.Set("label", paymentLabel)
	q.Set("message", fmt.Sprintf("Payment type: %s", paymentType))
	q.Set("memo", referenceKey.PublicKey().String())

	u.RawQuery = q.Encode()
	amountConverted, err := decimal.NewFromString(amount)
	if err != nil {
		return "", fmt.Errorf("NewFromString: %s", err)
	}
	err = p.pgStorage.CreateUnconfirmedPayment(ctx, referenceKey.PublicKey(), userID, amountConverted.Truncate(metaplexTokenDecimals).Mul(metaplexTokenDecimalsMultiplier).Floor().BigInt().Int64())
	if err != nil {
		return "", fmt.Errorf("CreateUnconfirmedPayment: %w", err)
	}

	return u.String(), nil
}

func (p *paymentsWatcher) watchPayments(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		lastProcessedSig, err := p.pgStorage.FetchLastProcessedSignature(ctx)
		if err != nil && !errors.Is(err, pg.ErrNoRows) {
			log.Logger.API.Errorf("watchPayments: FetchLastProcessedSignature: %s", err)
		}
		p.lastProcessedSignature = lastProcessedSig
		unpaidReferences, err := p.pgStorage.FetchAllUnpaidReferences(ctx)
		if err != nil && !errors.Is(err, pg.ErrNoRows) {
			log.Logger.API.Errorf("watchPayments: FetchAllUnpaidReferences: %s", err)
		}
		p.unpaidReferences = unpaidReferences

		err = p.processNewTransfers(ctx)
		if err != nil {
			log.Logger.API.Errorf("watchPayments: processNewTransfers: %s", err)
		}
		// prevent rate-limit errors
		time.Sleep(250 * time.Millisecond)
	}
}

// TODO: consider user reusing reference in multiple txs
func (p *paymentsWatcher) processNewTransfers(ctx context.Context) (err error) {
	allNewSignatures, err := p.fetchNewTransactionSignatures(ctx)
	if err != nil {
		return fmt.Errorf("fetchNewTransactionSignatures: %s", err)
	}
	for _, sig := range allNewSignatures {
		transfer, err := p.processTransaction(ctx, sig)
		if err != nil {
			log.Logger.API.Errorf("fetchNewTransfers: processTransaction: %s", err)
			err = p.pgStorage.SaveFailTransactionProcessingSignature(ctx, sig, err.Error())
			if err != nil {
				log.Logger.API.Errorf("processNewTransfers: SaveFailTransactionProcessingSignature 1: %s", err)
			}
			continue
		}
		err = p.pgStorage.UpdatePayments(ctx, []postgres.TransferInfo{transfer})
		if err != nil {
			log.Logger.API.Errorf("fetchNewTransfers: UpdatePayments %s: %s", sig, err)
			err = p.pgStorage.SaveFailTransactionProcessingSignature(ctx, sig, err.Error())
			if err != nil {
				log.Logger.API.Errorf("processNewTransfers: SaveFailTransactionProcessingSignature 2: %s", err)
			}
			continue
		}
		// prevent rate-limit errors
		time.Sleep(250 * time.Millisecond)
	}

	return nil
}

func (p *paymentsWatcher) fetchNewTransactionSignatures(ctx context.Context) (allNewSignatures []solana.Signature, err error) {
	var beforeSig solana.Signature
	unfetchedSignaturesLeft := true

	for unfetchedSignaturesLeft {
		cfg := rpc.GetSignaturesForAddressOpts{
			Commitment: rpc.CommitmentFinalized,
			Until:      p.lastProcessedSignature,
			Before:     beforeSig,
		}
		sigs, err := p.rpcClient.GetSignaturesForAddressWithOpts(ctx, p.paymentRecipientAssociatedTokenAddress, &cfg)
		if err != nil {
			return nil, fmt.Errorf("GetSignaturesForAddressWithConfig: %w", err)
		}
		if len(sigs) == 0 {
			break
		}
		for _, sigInfo := range sigs {
			allNewSignatures = append(allNewSignatures, sigInfo.Signature)
		}

		beforeSig = sigs[len(sigs)-1].Signature
		if len(sigs) < getSignaturesLimit {
			unfetchedSignaturesLeft = false
		}
		// prevent rate-limit errors
		time.Sleep(250 * time.Millisecond)
	}

	return allNewSignatures, nil
}

func (p *paymentsWatcher) processTransaction(ctx context.Context, sig solana.Signature) (transfers postgres.TransferInfo, err error) {
	txResp, err := p.rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding:   solana.EncodingBase64,
		Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		// Node error. Need to retry later
		return transfers, fmt.Errorf("GetTransaction %s: %s", sig, err)
	}
	if txResp == nil || txResp.Transaction == nil {
		// Cannot fetch transaction from node. Maybe need to retry later
		return transfers, fmt.Errorf("empty transaction for signature: %s", sig)
	}
	transfers, err = p.parseTransaction(*txResp)
	if err != nil {
		// Error while parsing tx. Maybe the wrong tx format returned. Need to retry
		return transfers, fmt.Errorf("parseTransaction: %s", err)
	}

	return transfers, nil
}

func (p *paymentsWatcher) parseTransaction(txResp rpc.GetTransactionResult) (result postgres.TransferInfo, err error) {
	parsedTx, err := txResp.Transaction.GetTransaction()
	if err != nil {
		return result, fmt.Errorf("GetTransaction: %s", err)
	}
	if len(parsedTx.Signatures) == 0 {
		return result, fmt.Errorf("empty signature")
	}

	result.Signature = parsedTx.Signatures[0]

	for _, inst := range parsedTx.Message.Instructions {
		accounts, err := inst.ResolveInstructionAccounts(&parsedTx.Message)
		if err != nil {
			return result, fmt.Errorf("ResolveInstructionAccounts: %s", err)
		}

		if int(inst.ProgramIDIndex) < len(parsedTx.Message.AccountKeys) {
			programKey := parsedTx.Message.AccountKeys[inst.ProgramIDIndex]

			switch programKey {
			case token.ProgramID:
				{
					amount, err := p.getTransferredAmount(accounts, inst.Data, parsedTx)
					if err != nil {
						return result, fmt.Errorf("getTransferredAmount: %s", err)
					}
					result.Amount = amount

					// check if there is reference in Transfer instruction
					// in case API started to check old transactions
					// new payment transactions will put reference into memo
					for _, accountKey := range parsedTx.Message.AccountKeys {
						if _, ok := p.unpaidReferences[accountKey.String()]; ok {
							if accountKey != solana.SystemProgramID {
								result.Reference = accountKey
							}
						}
					}
				}
			case memo.ProgramID:
				{
					reference, err := p.getReferenceFromMemoInstr(inst.Data)
					if err != nil {
						return result, fmt.Errorf("getReferenceFromMemoInstr: %s", err)
					}
					result.Reference = reference
				}
			default:
				// unknown program
				continue
			}
		} else {
			// cannot identify program
			continue
		}
	}

	// means we've got invalid transaction with incorrect data
	if result.Amount == 0 || result.Reference == (solana.PublicKey{}) {
		return result, fmt.Errorf("could not extract all the expected information from the transaction")
	}

	return result, nil
}

func (p *paymentsWatcher) getReferenceFromMemoInstr(instrData solana.Base58) (reference solana.PublicKey, err error) {
	decoded, err := base58.Decode(instrData.String())
	if err != nil {
		return reference, fmt.Errorf("could not decode memo instruction data from base58: %s", err)
	}
	reference, err = solana.PublicKeyFromBase58(string(decoded))
	if err != nil {
		return reference, fmt.Errorf("getReferenceFromMemoInstr: %s", err)
	}

	return reference, nil
}

func (p *paymentsWatcher) getTransferredAmount(accounts []*solana.AccountMeta, instrData []byte, parsedTx *solana.Transaction) (amount int64, err error) {
	tokenInst, err := token.DecodeInstruction(accounts, instrData)
	if err != nil {
		return amount, fmt.Errorf("DecodeInstruction: %s", err)
	}

	switch spec := tokenInst.Impl.(type) {
	case *token.TransferChecked:
		{
			mint := spec.Accounts.Get(expectedMintAccountIndex)
			if mint == nil {
				return amount, fmt.Errorf("mint account is missed")
			}
			destination := spec.Accounts.Get(expectedDestinationAccountIndex)
			if destination == nil {
				return amount, fmt.Errorf("destination account is missed")
			}
			if mint.PublicKey != metaplexToken {
				// Invalid mint
				return amount, fmt.Errorf("mint account in transaction doesn't equal to MPLX token account mint. Received %s", mint.PublicKey)
			}
			if destination.PublicKey != p.paymentRecipientAssociatedTokenAddress {
				// Invalid destination
				return amount, fmt.Errorf("destination wallet address doesn't match with expected. Received %s", destination.PublicKey)
			}
			if spec.Amount == nil {
				return amount, fmt.Errorf("amount is missed")
			}

			amount = int64(*spec.Amount)
		}
	default:
		// Another token program instruction
		return amount, nil
	}

	return amount, nil
}
