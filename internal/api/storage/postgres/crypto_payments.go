package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
)

type TransferInfo struct {
	Signature solana.Signature
	Memo      solana.PublicKey
	Amount    int64
}

type (
	CryptoPayment struct {
		ID         int64      `pg:"crp_id" json:"-"`
		Memo       string     `pg:"crp_memo" json:"memo"`
		Signature  *string    `pg:"crp_signature" json:"signature"`
		UserID     int64      `pg:"usr_id" json:"-"`
		MplxAmount *int64     `pg:"crp_mplx_amount" json:"mplx_amount"`
		CreatedAt  time.Time  `pg:"crp_created_at" json:"created_at"`
		PaidAt     *time.Time `pg:"crp_paid_at" json:"paid_at"`
	}
	CryptoPaymentWithTotalCount struct {
		Total int64 `pg:"total_count" json:"total_count"`
		CryptoPayment
	}
)

const (
	paymentsTable = "crypto_payments"
)

func (s *Storage) CreateUnconfirmedPayment(ctx context.Context, memo solana.PublicKey, userID, mplxAmount int64) (err error) {
	query := `INSERT INTO crypto_payments (crp_memo, usr_id, crp_mplx_amount) VALUES (?, ?, ?)`
	_, err = s.db.ExecOneContext(ctx, query, memo.String(), userID, mplxAmount)
	if err != nil {
		return fmt.Errorf("QueryOneContext: %w", err)
	}

	return nil
}

func (s *Storage) UpdatePayments(ctx context.Context, transfers []TransferInfo) error {
	if len(transfers) == 0 {
		return nil
	}

	// TODO: consider SELECT FOR UPDATE
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginTx: %s", err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, transfer := range transfers {
		query := `UPDATE crypto_payments SET crp_signature = ?, crp_mplx_amount = ?, crp_paid_at = now() WHERE crp_memo = ? AND crp_paid_at IS NULL AND crp_paid_at IS NULL RETURNING usr_id`
		var payment CryptoPayment
		res, err := tx.db.QueryOneContext(ctx, &payment, query, transfer.Signature.String(), transfer.Amount, transfer.Memo.String())
		if err != nil {
			return fmt.Errorf("QueryOneContext 1: %s", err)
		}
		if res.RowsAffected() != 1 {
			return errors.New("RowsAffected != 1")
		}

		query = `UPDATE users SET usr_mplx_balance = usr_mplx_balance + ? WHERE usr_id = ?`
		_, err = tx.db.ExecOneContext(ctx, query, transfer.Amount, payment.UserID)
		if err != nil {
			return fmt.Errorf("ExecOneContext 2: %s", err)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit: %s", err)
	}

	return nil
}

func (s *Storage) FetchLastProcessedSignature(ctx context.Context) (sig solana.Signature, err error) {
	query := `SELECT crp_signature FROM crypto_payments WHERE crp_signature IS NOT NULL ORDER BY crp_paid_at DESC LIMIT 1`
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

func (s *Storage) FetchAllUnpaidMemos(ctx context.Context) (memos map[string]struct{}, err error) {
	query := `SELECT crp_memo FROM crypto_payments WHERE crp_paid_at IS NULL`
	var payments []CryptoPayment
	_, err = s.db.QueryContext(ctx, &payments, query)
	if err != nil {
		return memos, fmt.Errorf("QueryContext: %w", err)
	}
	memos = make(map[string]struct{}, len(payments))
	for i := range payments {
		memos[payments[i].Memo] = struct{}{}
	}

	return memos, nil
}

func (s *Storage) CheckIfMemoPaid(ctx context.Context, memo string) (isPaid bool, err error) {
	query := `SELECT crp_paid_at FROM crypto_payments WHERE crp_memo = ?`
	var payment CryptoPayment
	_, err = s.db.QueryContext(ctx, &payment, query, memo)
	if err != nil {
		return isPaid, fmt.Errorf("QueryContext: %w", err)
	}
	if payment.PaidAt != nil {
		isPaid = true
	}

	return isPaid, nil
}

func (s *Storage) GetUserPaymentHistory(ctx context.Context, userID, limit, page int64) (paymentHistory []CryptoPaymentWithTotalCount, err error) {
	if page == 0 {
		page = 1
	}
	query := `SELECT COUNT(*) OVER() AS total_count, crp_id, crp_memo, crp_signature, usr_id, crp_mplx_amount, crp_created_at, crp_paid_at
					FROM crypto_payments WHERE usr_id = ? ORDER BY crp_created_at DESC LIMIT ? OFFSET ?`
	_, err = s.db.QueryContext(ctx, &paymentHistory, query, userID, limit, (page-1)*limit)
	if err != nil {
		return paymentHistory, fmt.Errorf("QueryContext: %w", err)
	}

	return paymentHistory, nil
}
