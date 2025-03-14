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
	query := `INSERT INTO failed_processing_transaction (fpt_signature, fpt_fail_reason) VALUES (?, ?) ON CONFLICT (fpt_signature) DO NOTHING;`
	_, err = s.db.ExecOneContext(ctx, query, sig.String(), failReason)
	if err != nil && err.Error() != "pg: no rows in result set" {
		return fmt.Errorf("ExecOneContext: %w", err)
	}

	return nil
}
