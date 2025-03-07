package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/go-pg/pg/v10"
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
		Status     string     `pg:"crp_status" json:"status"`
	}
	CryptoPaymentWithTotalCount struct {
		Total int64 `pg:"total_count" json:"total_count"`
		CryptoPayment
	}

	Volume struct {
		Volume int64
	}

	DailyVolume struct {
		Volume int64     `pg:"volume" json:"volume"`
		Day    time.Time `pg:"day" json:"day"`
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

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginTx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, transfer := range transfers {
		var payment CryptoPayment
		selectQuery := `SELECT * FROM crypto_payments WHERE crp_memo = ? AND crp_paid_at IS NULL AND crp_status = 'unpaid' FOR UPDATE SKIP LOCKED`
		_, err := tx.db.QueryOneContext(ctx, &payment, selectQuery, transfer.Memo.String())
		if err != nil {
			return fmt.Errorf("QueryOneContext (select): %w", err)
		}

		updateQuery := `UPDATE crypto_payments SET crp_signature = ?, crp_mplx_amount = ?, crp_status = 'paid', crp_paid_at = NOW() WHERE crp_id = ?`
		_, err = tx.db.ExecOneContext(ctx, updateQuery, transfer.Signature.String(), transfer.Amount, payment.ID)
		if err != nil {
			return fmt.Errorf("ExecOneContext (update): %w", err)
		}

		balanceUpdateQuery := `UPDATE users SET usr_mplx_balance = usr_mplx_balance + ? WHERE usr_id = ?`
		_, err = tx.db.ExecOneContext(ctx, balanceUpdateQuery, transfer.Amount, payment.UserID)
		if err != nil {
			return fmt.Errorf("ExecOneContext (balance update): %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit: %w", err)
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
	query := `SELECT crp_memo FROM crypto_payments WHERE crp_paid_at IS NULL AND crp_status = 'unpaid'`
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
	query := `SELECT crp_paid_at FROM crypto_payments WHERE crp_status = 'paid' AND crp_memo = ?`
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
	query := `SELECT COUNT(*) OVER() AS total_count, crp_id, crp_memo, crp_signature, usr_id, crp_mplx_amount, crp_created_at, crp_paid_at, crp_status
					FROM crypto_payments WHERE usr_id = ? ORDER BY crp_created_at DESC LIMIT ? OFFSET ?`
	_, err = s.db.QueryContext(ctx, &paymentHistory, query, userID, limit, (page-1)*limit)
	if err != nil {
		return paymentHistory, fmt.Errorf("QueryContext: %w", err)
	}

	return paymentHistory, nil
}
func (s *Storage) CancelUnpaidPayments(ctx context.Context) error {
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var paymentIDs []int
	selectQuery := `
		SELECT crp_id FROM crypto_payments 
		WHERE crp_status = 'unpaid' 
		AND NOW() > crp_created_at + INTERVAL '3 hours'
		FOR UPDATE SKIP LOCKED;`

	_, err = tx.db.QueryContext(ctx, &paymentIDs, selectQuery)
	if err != nil {
		return fmt.Errorf("failed to select rows: %w", err)
	}

	if len(paymentIDs) == 0 {
		return nil
	}

	updateQuery := `
		UPDATE crypto_payments 
		SET crp_status = 'cancelled' 
		WHERE crp_id IN (?);`

	_, err = tx.db.ExecContext(ctx, updateQuery, pg.In(paymentIDs))
	if err != nil {
		return fmt.Errorf("failed to update rows: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *Storage) GetTotalMPLXVolume(ctx context.Context) (int64, error) {
	query := `SELECT SUM(crp_mplx_amount) FROM crypto_payments WHERE crp_status = 'paid';`
	var result Volume
	_, err := s.db.QueryOneContext(ctx, &result, query)
	if err != nil {
		return result.Volume, err
	}

	return result.Volume, nil
}

func (s *Storage) GetDailyMPLXVolume(ctx context.Context, startDay time.Time, endDay time.Time) (result []DailyVolume, err error) {
	if startDay.After(endDay) {
		return result, fmt.Errorf("failed to get daily MPLX volume because start day cannot be gibber than end data: %w and %w", startDay, endDay)
	}

	query := `SELECT SUM(crp_mplx_amount) as volume, crp_paid_at::date as day FROM crypto_payments WHERE crp_status = 'paid' AND crp_paid_at::date BETWEEN ? AND ? GROUP BY crp_paid_at::date
ORDER BY day;`
	_, err = s.db.QueryContext(ctx, &result, query, startDay.Format("2006-01-02"), endDay.Format("2006-01-02"))
	if err != nil {
		return result, err
	}

	return result, nil
}
