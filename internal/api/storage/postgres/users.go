package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-pg/pg/v10"
)

type (
	User struct {
		ID          int64     `pg:"usr_id" json:"id"`
		MplxBalance int64     `pg:"usr_mplx_balance" json:"mplx_balance"`
		DynamicID   string    `pg:"usr_dynamic_id" json:"dynamic_id"`
		CreatedAt   time.Time `pg:"usr_created_at" json:"created_at"`
	}
	Subscription struct {
		Name              string    `pg:"sbs_name" json:"name"`
		RequestsPerSecond int64     `pg:"sbs_request_per_second" json:"requests_per_second"`
		MinimumBalance    int64     `pg:"sbs_minimum_balance" json:"minimum_balance"`
		TokenLimit        int64     `pg:"sbs_tokens_limit" json:"token_limit"`
		CreatedAt         time.Time `pg:"sbs_created_at" json:"-"`
	}
	UserWithSubscription struct {
		User
		Subscription
	}
)

const (
	usersTable = "users"
)

func (s *Storage) GetOrCreateUser(ctx context.Context, dynamicID string) (u UserWithSubscription, err error) {
	if dynamicID == "" {
		return u, ErrEmptyDynamicID
	}

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return u, fmt.Errorf("beginTx: %s", err)
	}
	defer tx.Rollback() //nolint:errcheck

	u, err = tx.GetUser(ctx, dynamicID)
	if errors.Is(err, pg.ErrNoRows) {
		err = tx.CreateUser(ctx, dynamicID)
		if err != nil {
			return u, fmt.Errorf("CreateUser: %s", err)
		}
		u, err = tx.GetUser(ctx, dynamicID)
	}
	if err != nil {
		return u, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return u, fmt.Errorf("commit: %s", err)
	}

	return u, nil
}

func (s *Storage) CreateUser(ctx context.Context, dynamicID string) error {
	if dynamicID == "" {
		return ErrEmptyDynamicID
	}

	query := `INSERT INTO users (usr_dynamic_id)
			VALUES (?)`
	_, err := s.db.ExecContext(ctx, query, dynamicID)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetUser(ctx context.Context, dynamicID string) (u UserWithSubscription, err error) {
	if dynamicID == "" {
		return u, ErrEmptyDynamicID
	}

	query := `SELECT usr_id, usr_dynamic_id, usr_created_at, usr_mplx_balance, sbs_name, sbs_request_per_second, sbs_minimum_balance, sbs_tokens_limit, sbs_created_at
				FROM users 
    			LEFT JOIN subscriptions USING(sbs_id)
				WHERE usr_dynamic_id = ?`
	_, err = s.db.QueryOneContext(ctx, &u, query, dynamicID)
	if err != nil {
		return u, err
	}

	return u, nil
}
