package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
)

type (
	APIKey struct {
		ID        int64      `pg:"uak_id" json:"-"`
		UserID    int64      `pg:"usr_id" json:"-"`
		Name      string     `pg:"uak_name" json:"name"`
		Token     uuid.UUID  `pg:"uak_token" json:"token"`
		CreatedAt time.Time  `pg:"uak_created_at" json:"created_at"`
		DeletedAt *time.Time `pg:"uak_deleted_at" json:"deleted_at"`
	}
	APIKeyWithSupportedNetworks struct {
		APIKey
		SupportedNetworks []string `pg:"supported_networks" json:"supported_networks"`
	}
)

const (
	apiKeysTable         = "user_api_keys"
	apiKeysNetworksTable = "user_api_keys_networks"
)

func (s *Storage) CreateAPIKey(ctx context.Context, userID int64, name string, networks []int64) error {
	if userID == 0 {
		return ErrEmptyUserID
	}
	if name == "" {
		return errors.New("empty name")
	}

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginTx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var apiKey APIKey
	query := "INSERT INTO user_api_keys (usr_id, uak_name) VALUES (?, ?) RETURNING uak_id"
	_, err = tx.db.QueryOneContext(ctx, &apiKey, query, userID, name)
	if err != nil {
		return fmt.Errorf("apiKey QueryOneContext: %w", err)
	}

	if err := tx.insertAPIKeyNetworks(ctx, apiKey.ID, networks); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (s *Storage) GetAPIKeysByUser(ctx context.Context, userID int64, notDeleted *bool) (apiKeys []APIKeyWithSupportedNetworks, err error) {
	if userID == 0 {
		return nil, ErrEmptyUserID
	}

	q := sq.Select("uak_id, usr_id, uak_name, uak_token, uak_created_at, uak_deleted_at, JSON_AGG(networks.ntw_name) AS supported_networks").
		From(apiKeysTable).
		LeftJoin("user_api_keys_networks USING(uak_id)").
		LeftJoin("networks USING(ntw_id)").
		Where("usr_id = ?", userID).
		GroupBy("uak_id")
	if notDeleted != nil {
		if *notDeleted {
			q = q.Where("uak_deleted_at IS NULL")
		} else {
			q = q.Where("uak_deleted_at IS NOT NULL")
		}
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	_, err = s.db.QueryContext(ctx, &apiKeys, query, args...)
	if err != nil {
		return apiKeys, fmt.Errorf("select: %w", err)
	}

	return apiKeys, nil
}

func (s *Storage) GetAPIKeyByTokenAndUserDynamicID(ctx context.Context, apiKeyToken uuid.UUID, userDynamicID string) (apiKey APIKeyWithSupportedNetworks, err error) {
	if userDynamicID == "" {
		return apiKey, ErrEmptyDynamicID
	}
	query := `
		SELECT uak_id, usr_id, uak_name, uak_token, uak_created_at, uak_deleted_at, JSON_AGG(networks.ntw_name) AS supported_networks
		FROM user_api_keys
		LEFT JOIN user_api_keys_networks USING(uak_id)
		LEFT JOIN networks USING(ntw_id)
		WHERE usr_id = (SELECT usr_id FROM users WHERE usr_dynamic_id = ?)
		AND uak_token = ?
		GROUP BY uak_id 
	`
	_, err = s.db.QueryOneContext(ctx, &apiKey, query, userDynamicID, apiKeyToken)
	return apiKey, err
}

func (s *Storage) UpdateAPIKey(ctx context.Context, apiKeyToken uuid.UUID, userDynamicID string, name *string, networks []int64) (err error) {
	if userDynamicID == "" {
		return ErrEmptyDynamicID
	}

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("beginTx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	query := sq.Update(apiKeysTable)
	if name != nil {
		query = query.Set("uak_name", *name)
	}
	query = query.
		Where("usr_id = (SELECT usr_id FROM users WHERE usr_dynamic_id = ?)", userDynamicID).
		Where("uak_deleted_at is NULL", apiKeyToken).
		Where("uak_token = ?", apiKeyToken).
		Suffix("RETURNING uak_id, uak_name, uak_token")
	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("ToSql user_api_keys: %w", err)
	}

	var apiKey APIKeyWithSupportedNetworks
	_, err = tx.db.QueryOneContext(ctx, &apiKey, querySQL, args...)
	if err != nil {
		return fmt.Errorf("update user_api_keys: %w", err)
	}

	if len(networks) > 0 {
		if err := tx.updateAPIKeyNetworks(ctx, apiKey.ID, networks); err != nil {
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (s *Storage) DeleteAPIKey(ctx context.Context, apiKeyToken uuid.UUID, userDynamicID string) error {
	if userDynamicID == "" {
		return ErrEmptyDynamicID
	}
	q := `
		UPDATE user_api_keys 
		SET uak_deleted_at = NOW() 
		WHERE usr_id = (SELECT usr_id FROM users WHERE usr_dynamic_id = ?) AND uak_token = ? AND uak_deleted_at IS NULL 
		RETURNING uak_id
	`
	var apiKey APIKeyWithSupportedNetworks
	_, err := s.db.QueryOneContext(ctx, &apiKey, q, userDynamicID, apiKeyToken)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) insertAPIKeyNetworks(ctx context.Context, apiKeyID int64, networks []int64) error {
	if len(networks) == 0 {
		return nil
	}

	qb := sq.Insert(apiKeysNetworksTable).Columns("uak_id", "ntw_id").Suffix("ON CONFLICT (uak_id, ntw_id) DO NOTHING")
	for _, nID := range networks {
		qb = qb.Values(apiKeyID, nID)
	}
	query, args, err := qb.ToSql()
	if err != nil {
		return fmt.Errorf("ToSql: %w", err)
	}
	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("ExecContext: %w", err)
	}

	return nil
}

func (s *Storage) updateAPIKeyNetworks(ctx context.Context, apiKeyID int64, networks []int64) error {
	qb := sq.Delete(apiKeysNetworksTable).Where("uak_id = ? ", apiKeyID)
	query, args, err := qb.ToSql()
	if err != nil {
		return fmt.Errorf("ToSql: %w", err)
	}
	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("ExecContext delete: %w", err)
	}

	return s.insertAPIKeyNetworks(ctx, apiKeyID, networks)
}
