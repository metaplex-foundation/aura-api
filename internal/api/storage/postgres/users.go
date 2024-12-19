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
		ID                 int64     `pg:"usr_id" json:"id"`
		MplxBalance        int64     `pg:"usr_mplx_balance" json:"mplx_balance"`
		DynamicID          string    `pg:"usr_dynamic_id" json:"dynamic_id"`
		CreatedAt          time.Time `pg:"usr_created_at" json:"created_at"`
		LastUpdatedPlanAt  time.Time `pg:"usr_last_updated_plan_at" json:"last_updated_plan_at"`
		SubscriptionEndsOn time.Time `pg:"usr_sbs_ends_on" json:"usr_sbs_ends_on"`
	}
	UserWithCurrentPlan struct {
		User
		Plan
	}
)

const (
	usersTable = "users"
)

func (s *Storage) GetOrCreateUser(ctx context.Context, dynamicID string) (u UserWithCurrentPlan, err error) {
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

func (s *Storage) GetUser(ctx context.Context, dynamicID string) (u UserWithCurrentPlan, err error) {
	if dynamicID == "" {
		return u, ErrEmptyDynamicID
	}

	query := `SELECT usr_id, usr_dynamic_id, usr_created_at, usr_mplx_balance, usr_last_updated_plan_at, sbs_id, usr_sbs_ends_on, sbs_priority, sbs_name, sbs_tokens_limit, sbs_created_at
				FROM users 
    			LEFT JOIN subscriptions USING(sbs_id)
				WHERE usr_dynamic_id = ?`
	_, err = s.db.QueryOneContext(ctx, &u, query, dynamicID)
	if err != nil {
		return u, err
	}

	return u, nil
}

func (s *Storage) UpdateUserSubscriptionPlan(ctx context.Context, usrID int64, subscriptionID int64) (err error) {
	query := `UPDATE users SET sbs_id = ? WHERE usr_id = ?;`
	_, err = s.db.ExecOneContext(ctx, query, subscriptionID, usrID)
	if err != nil {
		return err
	}

	return nil
}
