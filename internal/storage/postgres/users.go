package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-pg/pg/v10"

	auraProto "github.com/adm-metaex/aura-api/pkg/proto"
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
	UserWithAPIKeys struct {
		DynamicID          string     `pg:"usr_dynamic_id"`
		SubscriptionID     int64      `pg:"sbs_id"`
		MplxBalance        int64      `pg:"usr_mplx_balance"`
		SubscriptionEndsOn *time.Time `pg:"usr_sbs_ends_on"`
		APIKeys            []string   `pg:"api_keys"`
	}

	UsersCountByPlans struct {
		SubscriptionId int8  `pg:"subscription"`
		Count          int64 `pg:"users_count"`
	}

	UsersSnapshot struct {
		Day                   time.Time `pg:"urs_day" json:"day"`
		ProSubscriptions      int64     `pg:"urs_pro_subscriptions" json:"pro_subscriptions"`
		AdvancedSubscriptions int64     `pg:"urs_advanced_subscriptions" json:"advanced_subscriptions"`
		PayAsYouGo            int64     `pg:"urs_pay_as_you_go" json:"pay_as_you_go"`
		ActiveUsers           int64     `pg:"urs_active_users" json:"active_users"`
		TotalUsers            int64     `pg:"urs_total_users" json:"total_users"`
	}

	Count struct {
		Count int64
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

func (s *Storage) GetUserByAPIKey(ctx context.Context, apiToken string) (u UserWithAPIKeys, err error) {
	if apiToken == "" {
		return u, errors.New("empty token")
	}

	query := `SELECT 
	    users.usr_dynamic_id,
	    users.sbs_id,
	    users.usr_mplx_balance,
	    users.usr_sbs_ends_on,
	    (SELECT json_agg(user_api_keys.uak_token) FROM user_api_keys WHERE user_api_keys.usr_id = users.usr_id) as api_keys
	FROM 
	    users
	LEFT JOIN 
	    user_api_keys USING(usr_id)
	WHERE 
	    user_api_keys.uak_token = ?
	GROUP BY 
	    users.usr_id, users.usr_dynamic_id, users.sbs_id, users.usr_mplx_balance, users.usr_sbs_ends_on;`
	_, err = s.db.QueryOneContext(ctx, &u, query, apiToken)
	if err != nil {
		return u, err
	}

	return u, nil
}

// UpdateUserBalances use context.Background in order not to cancel queries
func (s *Storage) UpdateUserBalances(req *auraProto.IncreaseUserRequestsReq) error {
	tx, err := s.BeginTx(context.Background())
	if err != nil {
		return fmt.Errorf("beginTx: %s", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// TODO: sort items in map
	for userID, chains := range req.GetReqs() {
		// Sum all credits for the user
		var totalCredits int64
		for _, reqType := range chains.GetReqs() {
			for _, tokens := range reqType.GetReqs() {
				for token, reqWithUsage := range tokens.GetReqs() {
					query := `UPDATE user_api_keys SET uak_total_requests = uak_total_requests + ?, uak_last_used_at = now() WHERE uak_token = ?`
					_, err = tx.db.Exec(query, reqWithUsage.GetReqs(), token)
					if err != nil {
						return fmt.Errorf("exec token %s: %w", token, err)
					}
					totalCredits += reqWithUsage.GetUsage()
				}
			}
		}

		query := `UPDATE users
			SET usr_mplx_balance = GREATEST(usr_mplx_balance - ?, 0)
			WHERE usr_dynamic_id = ?`
		_, err = tx.db.Exec(query, totalCredits, userID)
		if err != nil {
			return fmt.Errorf("exec user %s: %w", userID, err)
		}
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("commit: %s", err)
	}

	return nil
}

func (s *Storage) GetCountOfSubscriptionUsersByDay(ctx context.Context, subscriptionId int, day time.Time) (count int64, err error) {
	query := `SELECT COUNT(*) FROM users WHERE sbs_id = ? and usr_last_updated_plan_at <= ? and usr_sbs_ends_on >= ?;`
	var result Count
	_, err = s.db.QueryOneContext(ctx, &result, query, subscriptionId, day, day)
	if err != nil {
		return count, err
	}

	return result.Count, nil
}

func (s *Storage) GetCurrentUsersCountByPlans(ctx context.Context) (result []UsersCountByPlans, err error) {
	query := `SELECT
				sbs_id as subscription,
				COUNT(*) as users_count
			FROM
				users
			WHERE
				usr_created_at::date < CURRENT_DATE
				AND (usr_last_updated_plan_at < CURRENT_DATE OR usr_last_updated_plan_at IS NULL)
				AND (usr_sbs_ends_on::date >= (CURRENT_DATE - INTERVAL '1 day') OR usr_sbs_ends_on IS NULL)
			GROUP BY
				subscription;`

	_, err = s.db.QueryContext(ctx, &result, query)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (s *Storage) SaveUsersSnapshot(ctx context.Context, snapshot UsersSnapshot) (err error) {
	query := `INSERT INTO users_snapshot (urs_day, urs_pro_subscriptions, urs_advanced_subscriptions, urs_pay_as_you_go, urs_active_users, urs_total_users)
			VALUES (?,?,?,?,?,?);`
	_, err = s.db.ExecContext(ctx, query, snapshot.Day, snapshot.ProSubscriptions, snapshot.AdvancedSubscriptions, snapshot.PayAsYouGo, snapshot.ActiveUsers, snapshot.TotalUsers)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetDailyUserSnapshots(ctx context.Context, startDay time.Time, endDay time.Time) (result []UsersSnapshot, err error) {
	if startDay.After(endDay) {
		return result, fmt.Errorf("failed to get daily users snapshot because start day cannot be gibber than end data: %w and %w", startDay, endDay)
	}

	query := `SELECT
				urs_day,
				urs_pro_subscriptions,
				urs_advanced_subscriptions,
				urs_pay_as_you_go,
				urs_active_users,
				urs_total_users
			FROM
				users_snapshot
			WHERE
				urs_day BETWEEN ? AND ?
			ORDER BY
				urs_day;`

	_, err = s.db.QueryContext(ctx, &result, query, startDay.Truncate(24*time.Hour), endDay.Truncate(24*time.Hour))
	if err != nil {
		return result, err
	}

	return result, nil
}
