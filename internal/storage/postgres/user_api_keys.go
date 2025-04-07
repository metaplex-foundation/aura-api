package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/adm-metaex/aura-api/internal/models"
	log "github.com/adm-metaex/aura-api/pkg/log"
	customErrors "github.com/adm-metaex/aura-api/pkg/util"
	"github.com/go-pg/pg/v10"
	"github.com/google/uuid"
)

type (
	APIKey struct {
		ID            int64      `pg:"uak_id" json:"-"`
		UserID        int64      `pg:"usr_id" json:"-"`
		TotalRequests int64      `pg:"uak_total_requests" json:"total_requests"`
		Name          string     `pg:"uak_name" json:"name"`
		Token         uuid.UUID  `pg:"uak_token" json:"token"`
		CreatedAt     time.Time  `pg:"uak_created_at" json:"created_at"`
		DeletedAt     *time.Time `pg:"uak_deleted_at" json:"deleted_at"`
		LastUsed      *time.Time `pg:"uak_last_used_at" json:"last_used"`
		Deprecated    bool       `pg:"deprecated" json:"deprecated"`
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

func (s *Storage) CreateAPIKey(ctx context.Context, userID int64, name string, networks []int64) (apiKey APIKeyWithSupportedNetworks, err error) {
	if userID == 0 {
		return apiKey, ErrEmptyUserID
	}
	if name == "" {
		return apiKey, errors.New("empty name")
	}

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return apiKey, fmt.Errorf("beginTx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	query := "INSERT INTO user_api_keys (usr_id, uak_name) VALUES (?, ?) RETURNING uak_id, uak_token"
	_, err = tx.db.QueryOneContext(ctx, &apiKey, query, userID, name)
	if err != nil {
		return apiKey, fmt.Errorf("apiKey QueryOneContext: %w", err)
	}

	if err = tx.insertAPIKeyNetworks(ctx, apiKey.ID, networks); err != nil {
		return apiKey, err
	}
	apiKey, err = tx.GetAPIKeyByTokenAndUserDynamicID(ctx, apiKey.Token)
	if err != nil {
		return apiKey, fmt.Errorf("GetAPIKeyByTokenAndUserDynamicID: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return apiKey, fmt.Errorf("commit: %w", err)
	}

	if s.userNotifier != nil {
		go s.userNotifier.NotifyUserUpdate(models.UsrIDs{DBId: userID})
	} else {
		log.Logger.Postgre.Warn("userNotifier is not declared for storage. Attempt to call it in CreateAPIKey()")
	}

	return apiKey, nil
}

func (s *Storage) GetAPIKeysByUser(ctx context.Context, userID int64, showDeleted *bool) (apiKeys []APIKeyWithSupportedNetworks, err error) {
	if userID == 0 {
		return nil, ErrEmptyUserID
	}

	q := sq.Select("uak_id, usr_id, uak_name, uak_token, uak_created_at, uak_deleted_at, uak_total_requests, uak_last_used_at, deprecated, JSON_AGG(networks.ntw_name) AS supported_networks").
		From(apiKeysTable).
		LeftJoin("user_api_keys_networks USING(uak_id)").
		LeftJoin("networks USING(ntw_id)").
		Where("usr_id = ?", userID).
		GroupBy("uak_id").
		OrderBy("uak_created_at DESC")

	if showDeleted != nil && *showDeleted {
		q = q.Where("uak_deleted_at IS NOT NULL")
	} else if showDeleted == nil {
		q = q.Where("uak_deleted_at IS NULL")
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

func (s *Storage) GetAPIKeyByTokenAndUserDynamicID(ctx context.Context, apiKeyToken uuid.UUID) (apiKey APIKeyWithSupportedNetworks, err error) {
	query := `
		SELECT uak_id, usr_id, uak_name, uak_token, uak_created_at, uak_deleted_at, uak_total_requests, uak_last_used_at, JSON_AGG(networks.ntw_name) AS supported_networks
		FROM user_api_keys
		LEFT JOIN user_api_keys_networks USING(uak_id)
		LEFT JOIN networks USING(ntw_id)
		WHERE uak_token = ? AND deprecated=false
		GROUP BY uak_id
	`
	_, err = s.db.QueryOneContext(ctx, &apiKey, query, apiKeyToken)
	return apiKey, err
}

func (s *Storage) UpdateAPIKey(ctx context.Context, apiKeyToken uuid.UUID, name *string, networks []int64) (apiKey APIKeyWithSupportedNetworks, err error) {
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return apiKey, fmt.Errorf("beginTx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	query := sq.Update(apiKeysTable)
	if name != nil {
		query = query.Set("uak_name", *name)
	}
	query = query.
		Where("uak_deleted_at is NULL", apiKeyToken).
		Where("uak_token = ?", apiKeyToken).
		Suffix("RETURNING uak_id, uak_token")
	querySQL, args, err := query.ToSql()
	if err != nil {
		return apiKey, fmt.Errorf("ToSql user_api_keys: %w", err)
	}

	_, err = tx.db.QueryOneContext(ctx, &apiKey, querySQL, args...)
	if err != nil {
		return apiKey, fmt.Errorf("update user_api_keys: %w", err)
	}

	if len(networks) > 0 {
		if err = tx.updateAPIKeyNetworks(ctx, apiKey.ID, networks); err != nil {
			return apiKey, err
		}
	}
	apiKey, err = tx.GetAPIKeyByTokenAndUserDynamicID(ctx, apiKey.Token)
	if err != nil {
		return apiKey, fmt.Errorf("GetAPIKeyByTokenAndUserDynamicID: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return apiKey, fmt.Errorf("commit: %w", err)
	}

	return apiKey, nil
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

	if s.userNotifier != nil {
		go s.userNotifier.NotifyUserUpdate(models.UsrIDs{DynamicId: userDynamicID})
	} else {
		log.Logger.Postgre.Warn("userNotifier is not declared for storage. Attempt to call it in DeleteAPIKey()")
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

func (s *Storage) DeprecateAPIKeys(ctx context.Context, userID, keysDiff, nextSubscriptionID int64) error {
	if userID == 0 {
		return ErrEmptyUserID
	}

	query := `
		UPDATE user_api_keys 
		SET deprecated = true 
		WHERE uak_deleted_at IS NULL AND uak_id IN (
			SELECT uak_id 
			FROM user_api_keys 
			WHERE usr_id = ? AND deprecated = false AND uak_deleted_at IS NULL
			ORDER BY uak_total_requests ASC
			LIMIT (
				CASE 
					WHEN ? > (SELECT COUNT(*) FROM user_api_keys WHERE usr_id = ? AND deprecated = false AND uak_deleted_at IS NULL) 
					THEN (SELECT COUNT(*) FROM user_api_keys WHERE usr_id = ? AND deprecated = false AND uak_deleted_at IS NULL) - 
						(SELECT next_sub.sbs_tokens_limit 
							FROM subscriptions AS next_sub WHERE next_sub.sbs_id = 
								(SELECT ? FROM users WHERE usr_id = ?))
					ELSE ?
				END
			)
		)
	`

	if _, err := s.db.ExecContext(ctx, query, userID, keysDiff, userID, userID, nextSubscriptionID, userID, keysDiff); err != nil {
		return &customErrors.PgUpdateError{Msg: err.Error()}
	}

	if s.userNotifier != nil {
		go s.userNotifier.NotifyUserUpdate(models.UsrIDs{DBId: userID})
	} else {
		log.Logger.Postgre.Warn("userNotifier is not declared for storage. Attempt to call it in DeprecateAPIKeys()")
	}

	return nil
}

func (s *Storage) RestoreAPIKeys(ctx context.Context, userID, keysDiff int64) error {
	if userID == 0 {
		return ErrEmptyUserID
	}

	query := `
		UPDATE user_api_keys 
		SET deprecated = false 
		WHERE usr_id = ? AND uak_deleted_at IS NULL
		AND deprecated = true 
		AND uak_id IN (
			SELECT uak_id 
			FROM user_api_keys 
			WHERE usr_id = ? 
			AND deprecated = true 
			ORDER BY uak_total_requests DESC 
			LIMIT ?
		)
	`

	if _, err := s.db.ExecContext(ctx, query, userID, userID, keysDiff); err != nil {
		return &customErrors.PgUpdateError{Msg: err.Error()}
	}

	if s.userNotifier != nil {
		go s.userNotifier.NotifyUserUpdate(models.UsrIDs{DBId: userID})
	} else {
		log.Logger.Postgre.Warn("userNotifier is not declared for storage. Attempt to call it in RestoreAPIKeys()")
	}

	return nil
}

func (s *Storage) GetSubscriptionKeysDiff(ctx context.Context, userID, nextSubscription int64) (int64, error) {
	if userID == 0 {
		return 0, ErrEmptyUserID
	}

	query := `
		SELECT COALESCE(next_sub.sbs_tokens_limit - curr_sub.sbs_tokens_limit, 0) AS keys_diff
		FROM users
		LEFT JOIN subscriptions AS curr_sub ON users.sbs_id = curr_sub.sbs_id
		LEFT JOIN subscriptions AS next_sub ON next_sub.sbs_id = ?
		WHERE users.usr_id = ?
	`
	var keysDiff int64
	_, err := s.db.QueryContext(ctx, pg.Scan(&keysDiff), query, nextSubscription, userID)
	if err != nil {
		return 0, &customErrors.PgUpdateError{Msg: err.Error()}
	}
	// positive values mean that we need to add keys
	// negative values mean that we need to remove keys
	return keysDiff, nil
}
