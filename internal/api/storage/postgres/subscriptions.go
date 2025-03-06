package postgres

import (
	"context"
	"fmt"
	"time"

	customErrors "github.com/adm-metaex/aura-api/pkg/util"
	"github.com/go-pg/pg/v10"
)

type (
	Plan struct {
		PlanID     int64     `pg:"sbs_id" json:"id"`
		Name       string    `pg:"sbs_name" json:"name"`
		TokenLimit int64     `pg:"sbs_tokens_limit" json:"token_limit"`
		Priority   int64     `pg:"sbs_priority" json:"priority"`
		CreatedAt  time.Time `pg:"sbs_created_at" json:"-"`
	}

	SubscrPriceAndDuration struct {
		SbsPriceMplx    int64 `pg:"sbs_price_mplx" json:"sbs_price_mplx"`
		SbsDurationDays int64 `pg:"sbs_period_days" json:"sbs_period_days"`
	}
)

const (
	subscriptionsTable = "subscriptions"
)

func (s *Storage) GetSubscriptionsList(ctx context.Context) (subscriptions []Plan, err error) {
	query := `SELECT sbs_id, sbs_priority, sbs_name, sbs_tokens_limit, sbs_created_at
				FROM subscriptions`
	_, err = s.db.QueryContext(ctx, &subscriptions, query)
	if err != nil {
		return subscriptions, &customErrors.PgSelectError{Msg: fmt.Sprintf("QueryContext: %v", err)}
	}

	return subscriptions, nil
}

func (s *Storage) GetSubscrPriceAndDurationById(ctx context.Context, subscriptionId int) (sbsInfo SubscrPriceAndDuration, err error) {
	query := `SELECT sbs_price_mplx, sbs_period_days FROM subscriptions WHERE sbs_id = ?;`
	_, err = s.db.QueryContext(ctx, &sbsInfo, query, subscriptionId)
	if err != nil {
		return sbsInfo, &customErrors.PgSelectError{Msg: fmt.Sprintf("QueryContext: %v", err)}
	}

	return sbsInfo, nil
}

func (s *Storage) UpgradeUserSubscriptionPlan(ctx context.Context, usrID int64, newSubscriptionID int64) (err error) {
	var (
		newSubscriptionPrice, newSubscriptionPriority, userBalance             int64
		currSubscriptionEndsOn                                                 *time.Time
		oldSubscriptionPriority, newSubscriptionPeriodDays, currSubscriptionID *int64
	)

	tx, err := s.BeginTx(ctx)
	if err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to start transaction: %v", err)}
	}
	defer tx.Rollback()

	selectNewSubscription := `
	SELECT 
		sbs_price_mplx, 
		sbs_period_days, 
		sbs_priority 
	FROM subscriptions WHERE sbs_id = ?`
	if _, err = tx.db.QueryOneContext(
		ctx,
		pg.Scan(&newSubscriptionPrice, &newSubscriptionPeriodDays, &newSubscriptionPriority),
		selectNewSubscription,
		newSubscriptionID,
	); err != nil {
		return &customErrors.PgSelectError{Msg: fmt.Sprintf("failed to select new subscription: %v", err)}
	}

	selectInfoAboutUser := `
	SELECT 
		usr_mplx_balance, 
		sbs_priority, 
		usr_sbs_ends_on, 
		users.sbs_id
	FROM users
	JOIN subscriptions ON users.sbs_id = subscriptions.sbs_id 
	WHERE usr_id = ?
	FOR UPDATE SKIP LOCKED;`
	if _, err = tx.db.QueryOneContext(
		ctx,
		pg.Scan(&userBalance, &oldSubscriptionPriority, &currSubscriptionEndsOn, &currSubscriptionID),
		selectInfoAboutUser, usrID); err != nil {

		return &customErrors.PgSelectError{Msg: fmt.Sprintf("failed to retrieve info about an old subscription: %s", err)}
	}

	if oldSubscriptionPriority != nil && newSubscriptionPriority <= *oldSubscriptionPriority {
		return &customErrors.UpgradeSuscriptionError{Msg: "cannot upgrade to a subscription with lower or the same priority"}
	}

	if userBalance <= 0 || userBalance < newSubscriptionPrice {
		return &customErrors.UpgradeSuscriptionError{Msg: "insufficient balance to change subscription"}
	}

	if newSubscriptionPeriodDays == nil {
		defaultValue := int64(0)
		newSubscriptionPeriodDays = &defaultValue
	}
	// Update user's balance, expiration date, last updated plan timestamp, and subscription ID
	updateUserQuery := `
		UPDATE users 
		SET
			usr_mplx_balance = GREATEST(usr_mplx_balance - ?, 0),
			usr_sbs_ends_on = CASE 
				WHEN ? > 0 THEN NOW() + INTERVAL '1 day' * ? 
				ELSE NULL 
			END,
			usr_last_updated_plan_at = NOW(),
			sbs_id = ?,
			usr_next_sbs_id = ?
		WHERE usr_id = ?;
	`
	if _, err = tx.db.ExecContext(
		ctx,
		updateUserQuery,
		newSubscriptionPrice,
		newSubscriptionPeriodDays,
		newSubscriptionPeriodDays,
		newSubscriptionID,
		newSubscriptionID,
		usrID,
	); err != nil {
		return &customErrors.PgUpdateError{Msg: fmt.Sprintf("failed to update user's subscription: %v", err)}
	}

	if err = tx.Commit(ctx); err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to commit transaction: %v", err)}
	}

	return nil
}

func (s *Storage) DowngradeCurrentSubscription(ctx context.Context, userID, nextSubscriptionId int64) error {
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to start transaction: %v", err)}
	}
	defer tx.Rollback()

	var currentSubscriptionId, currentSubscriptionPriority, nextSubscriptionPriority int64
	var lastChangedAt time.Time
	query := `
		SELECT u.sbs_id, cs.sbs_priority, u.usr_last_updated_plan_at, ns.sbs_priority
		FROM users u
		JOIN subscriptions cs ON u.sbs_id = cs.sbs_id
		JOIN subscriptions ns ON ns.sbs_id = ?
		WHERE u.usr_id = ?`

	if _, err = tx.db.QueryOneContext(
		ctx,
		pg.Scan(
			&currentSubscriptionId,
			&currentSubscriptionPriority,
			&lastChangedAt,
			&nextSubscriptionPriority,
		),
		query,
		nextSubscriptionId,
		userID,
	); err != nil {
		return &customErrors.PgSelectError{Msg: fmt.Sprintf("failed to select subscription priorities: %v", err)}
	}

	if time.Since(lastChangedAt) < 24*time.Hour {
		return &customErrors.DowngradeSubscriptionError{Msg: "downgrade not allowed: last subscription change was less than 24 hours ago"}
	}

	if nextSubscriptionPriority >= currentSubscriptionPriority && !(nextSubscriptionPriority == 0 && currentSubscriptionPriority == 1) {
		return &customErrors.DowngradeSubscriptionError{Msg: "downgrade not allowed: next subscription priority is not lower than current subscription"}
	}

	if nextSubscriptionPriority == 0 && currentSubscriptionPriority == 1 {
		currentSubscriptionId = nextSubscriptionId
	}

	updateQuery := `
		UPDATE users 
		SET 
			usr_next_sbs_id = ?,
			sbs_id = ?
		WHERE usr_id = ?;`
	if _, err = tx.db.ExecContext(ctx, updateQuery, nextSubscriptionId, currentSubscriptionId, userID); err != nil {
		return &customErrors.PgUpdateError{Msg: fmt.Sprintf("failed to update rows: %v", err)}
	}

	if err = tx.Commit(ctx); err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to commit transaction: %v", err)}
	}

	return nil
}

func (s *Storage) UndoSubscriptionDowngrading(ctx context.Context, userID int64) error {
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to start transaction: %v", err)}
	}
	defer tx.Rollback()

	updateQuery := `
		UPDATE users 
		SET usr_next_sbs_id = sbs_id
		WHERE usr_id = ?;`
	if _, err = tx.db.ExecContext(ctx, updateQuery, userID); err != nil {
		return &customErrors.PgUpdateError{Msg: fmt.Sprintf("failed to update rows: %v", err)}
	}
	if err = tx.Commit(ctx); err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to commit transaction: %v", err)}
	}

	return nil
}

func (s *Storage) ResetExpiredPaymentPlans(ctx context.Context) error {
	tx, err := s.BeginTx(ctx)
	if err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to start transaction: %v", err)}
	}
	defer tx.Rollback()

	var usersIDs []int
	selectQuery := `
		SELECT usr_id FROM users 
		WHERE sbs_id != 1 AND NOW() > usr_sbs_ends_on
		FOR UPDATE SKIP LOCKED;
	`

	if _, err = tx.db.QueryContext(ctx, &usersIDs, selectQuery); err != nil {
		return &customErrors.PgSelectError{Msg: fmt.Sprintf("failed to select rows: %v", err)}
	}

	if len(usersIDs) == 0 {
		return nil
	}

	updateQuery := `
		UPDATE users 
		SET sbs_id = CASE 
			WHEN usr_mplx_balance >= (SELECT sbs_price_mplx FROM subscriptions WHERE sbs_id = usr_next_sbs_id) 
			THEN usr_next_sbs_id 
			ELSE 1 
		END, 
		usr_sbs_ends_on = CASE 
			WHEN usr_mplx_balance >= (SELECT sbs_price_mplx FROM subscriptions WHERE sbs_id = usr_next_sbs_id) 
			THEN NOW() + INTERVAL '1 day' * (SELECT sbs_period_days FROM subscriptions WHERE sbs_id = usr_next_sbs_id)
			ELSE NULL 
		END,
		usr_mplx_balance = CASE 
			WHEN usr_mplx_balance >= (SELECT sbs_price_mplx FROM subscriptions WHERE sbs_id = usr_next_sbs_id) 
			THEN usr_mplx_balance - (SELECT sbs_price_mplx FROM subscriptions WHERE sbs_id = usr_next_sbs_id) 
			ELSE usr_mplx_balance 
		END
		WHERE usr_id IN (?);`
	if _, err = tx.db.ExecContext(ctx, updateQuery, pg.In(usersIDs)); err != nil {
		return &customErrors.PgUpdateError{Msg: fmt.Sprintf("failed to update rows: %v", err)}
	}
	if err = tx.Commit(ctx); err != nil {
		return &customErrors.PgTransactionError{Msg: fmt.Sprintf("failed to commit transaction: %v", err)}
	}

	return nil
}
