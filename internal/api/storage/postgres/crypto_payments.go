package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
)

type TransferInfo struct {
	Signature solana.Signature
	Reference solana.PublicKey
	Amount    int64
}

type CryptoPayment struct {
	ID         int64      `pg:"crp_id" json:"crp___id"`
	Reference  string     `pg:"crp_reference" json:"crp___reference"`
	Signature  *string    `pg:"crp_signature" json:"crp___signature"`
	UserID     int64      `pg:"usr_id" json:"usr___id"`
	MplxAmount *int64     `pg:"crp_mplx_amount" json:"crp___mplx___amount"`
	CreatedAt  time.Time  `pg:"crp_created_at" json:"crp___created___at"`
	PaidAt     *time.Time `pg:"crp_paid_at" json:"crp___paid___at"`
}

const (
	paymentsTable = "crypto_payments"
)

func (s *Storage) CreateUnconfirmedPayment(ctx context.Context, reference solana.PublicKey, userID, mplxAmount int64) (err error) {
	query := `INSERT INTO crypto_payments (crp_reference, usr_id, crp_mplx_amount) VALUES (?, ?, ?)`
	_, err = s.db.ExecOneContext(ctx, query, reference.String(), userID, mplxAmount)
	if err != nil {
		return fmt.Errorf("QueryOneContext: %w", err)
	}

	return nil
}

func (s *Storage) UpdatePayments(ctx context.Context, transfers []TransferInfo) error {
	if len(transfers) == 0 {
		return nil
	}

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginTx: %s", err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, transfer := range transfers {
		query := `UPDATE crypto_payments SET crp_signature = ?, crp_mplx_amount = ?, crp_paid_at = now() WHERE crp_reference = ?`
		_, err = tx.db.ExecOneContext(ctx, query, transfer.Signature.String(), transfer.Amount, transfer.Reference.String())
		if err != nil {
			return fmt.Errorf("ExecOneContext: %s", err)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit: %s", err)
	}

	return nil
}

func (s *Storage) FetchLastProcessedSignature(ctx context.Context) (sig solana.Signature, err error) {
	query := `SELECT crp_signature FROM crypto_payments ORDER BY crp_paid_at DESC LIMIT 1`
	var payment CryptoPayment
	_, err = s.db.QueryOneContext(ctx, &payment, query)
	if err != nil {
		return sig, fmt.Errorf("QueryOneContext: %w", err)
	}
	if payment.Signature == nil {
		return sig, nil
	}
	sig, err = solana.SignatureFromBase58(*payment.Signature)
	if err != nil {
		return sig, fmt.Errorf("SignatureFromBase58: %w", err)
	}

	return sig, nil
}
