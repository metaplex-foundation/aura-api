package postgres

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

const (
	filedTransactionTable = "failed_processing_transaction"
)

func (s *Storage) SaveFailTransactionProcessingSignature(ctx context.Context, sig solana.Signature, failReason string) (err error) {
	query := `INSERT INTO failed_processing_transaction (fpt_signature, fpt_fail_reason) VALUES (?, ?);`
	_, err = s.db.ExecOneContext(ctx, query, sig.String(), failReason)
	if err != nil {
		return fmt.Errorf("ExecOneContext: %w", err)
	}

	return nil
}
