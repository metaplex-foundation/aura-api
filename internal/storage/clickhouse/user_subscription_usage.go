package clickhouse

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"github.com/adm-metaex/aura-api/pkg/log"
	auraProto "github.com/adm-metaex/aura-api/pkg/proto"
)

const userSubscriptionUsageTable = "aura.user_subscription_usage"

type UserSubscriptionUsage struct {
	Date  Date  `json:"date" swaggertype:"string" format:"date"`
	Value int64 `json:"value"`
}

type (
	CreditsUsageHistory struct {
		Timestamp time.Time        `json:"timestamp"`
		Networks  map[string]int64 `json:"networks"`
	}

	CreditsUsageWithReqType struct {
		Chain       string    `json:"chain"`
		RequestType string    `json:"request_type"`
		UsedCredits int64     `json:"used_credits"`
		Timestamp   time.Time `json:"ts"`
	}
)

func (r CreditsUsageHistory) GetTimestamp() time.Time {
	return r.Timestamp
}

func (r CreditsUsageHistory) BuildDefault(rpcMethod, network *string, token *uuid.UUID, t time.Time) CreditsUsageHistory {
	r.Timestamp = t
	r.Networks = map[string]int64{
		"eclipse":            0,
		"solana":             0,
		"solana-das":         0,
		"eclipse-das":        0,
		"getProgramAccounts": 0,
	}

	return r
}

func (s *Storage) BatchInsertUserSubscriptionUsage(reqs map[string]*auraProto.UserRequestsByChain) error {
	if len(reqs) == 0 {
		return nil
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return fmt.Errorf("tx begin error: %s", err)
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone { //nolint:errorlint
			log.Logger.General.Errorf("tx rollback error: %s", err)
		}
	}()

	stmt, err := tx.Prepare(`INSERT INTO user_subscription_usage (
		time,
		user_uid,
        used_credits,
        chain,
		request_type,
        tkn_uuid,
        is_mainnet
	)`)
	if err != nil {
		return fmt.Errorf("prepare statement error: %s", err)
	}
	defer stmt.Close()

	timeNow := time.Now()
	for userUID, reqChain := range reqs {
		for chain, requestType := range reqChain.GetReqs() {
			for reqType, reqToken := range requestType.GetReqs() {
				for token, reqWithUsage := range reqToken.GetReqs() {
					usage := reqWithUsage.GetUsage()
					if usage > 0 {
						_, err = stmt.Exec(
							timeNow,
							userUID,
							usage,
							chain,
							reqType,
							token,
							reqWithUsage.GetIsMainnet(),
						)
						if err != nil {
							return fmt.Errorf("exec statement error: %s", err)
						}
					}
				}
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("tx commit error: %s", err)
	}

	return nil
}

func (s *Storage) GetCreditsUsageHistory(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
	isMainnet *bool,
) ([]CreditsUsageHistory, error) {
	if userUID == "" {
		return nil, ErrEmptyUserUUID
	}

	startTime = truncateTimeByGranularity(startTime, granularity)

	query, args, err := buildCreditsUsageQuery(userUID, tknUUID, chain, rpcMethod, startTime, granularity, isMainnet)
	if err != nil {
		return nil, fmt.Errorf("buildCreditsUsageQuery: %w", err)
	}

	rawUsageData, err := s.executeCreditsQuery(query, args)
	if err != nil {
		return nil, fmt.Errorf("executeCreditsQuery: %w", err)
	}

	result := processCreditsUsageData(rawUsageData)

	return fillGaps(result, startTime, granularity, rpcMethod, chain, tknUUID), nil
}

func truncateTimeByGranularity(t time.Time, granularity string) time.Time {
	if granularity == HourlyGranularity {
		return t.Truncate(time.Hour)
	}
	return t.Truncate(24 * time.Hour)
}

func buildCreditsUsageQuery(
	userUID string,
	tknUUID *uuid.UUID,
	chain *string,
	rpcMethod *string,
	startTime time.Time,
	granularity string,
	isMainnet *bool,
) (string, []interface{}, error) {
	builder := sq.Select().
		PlaceholderFormat(sq.Question).
		Columns("chain", "request_type", "sum(used_credits) as used_credits").
		From(userSubscriptionUsageTable).
		OrderBy("ts").
		GroupBy("chain, request_type, ts").
		Where("time >= ?", startTime)

	if granularity == HourlyGranularity {
		builder = builder.Columns("toStartOfHour(time) as ts")
	} else {
		builder = builder.Columns("toDateTime(toDate(time)) as ts")
	}

	builder = buildWhereCondition(builder, userUID, tknUUID, chain, rpcMethod, true, isMainnet)

	return builder.ToSql()
}

func (s *Storage) executeCreditsQuery(sqlQuery string, args []interface{}) ([]CreditsUsageWithReqType, error) {
	rows, err := s.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("Query: %w", err)
	}
	defer rows.Close()

	usageData := make([]CreditsUsageWithReqType, 0)
	for rows.Next() {
		var entry CreditsUsageWithReqType
		if err = rows.Scan(&entry.Chain, &entry.RequestType, &entry.UsedCredits, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		usageData = append(usageData, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return usageData, nil
}

// It's required to do this mapping because of the way we display this info on graphs
func getNetworkType(chain, requestType string) string {
	key := chain + ":" + requestType

	networkMap := map[string]string{
		"solana:RPC":        "solana",
		"solana:DAS":        "solana-das",
		"solana:GPA":        "getProgramAccounts",
		"solana:Websocket":  "solana",
		"solana:SWQOS":      "solana",
		"eclipse:RPC":       "eclipse",
		"eclipse:DAS":       "eclipse-das",
		"eclipse:GPA":       "getProgramAccounts",
		"eclipse:Websocket": "eclipse",
		"eclipse:SWQOS":     "eclipse",
	}

	if networkType, ok := networkMap[key]; ok {
		return networkType
	}

	// Default empty string but it should not happen
	// such as all the possible variations are mentioned above
	return ""
}

func processCreditsUsageData(usageData []CreditsUsageWithReqType) []CreditsUsageHistory {
	creditsMap := make(map[time.Time]CreditsUsageHistory)

	for _, row := range usageData {
		networkType := getNetworkType(row.Chain, row.RequestType)

		if entry, exists := creditsMap[row.Timestamp]; exists {
			if credits, ok := entry.Networks[networkType]; ok {
				entry.Networks[networkType] = credits + row.UsedCredits
			} else {
				entry.Networks[networkType] = row.UsedCredits
			}
		} else {
			networks := make(map[string]int64)
			networks[networkType] = row.UsedCredits
			creditsMap[row.Timestamp] = CreditsUsageHistory{
				Timestamp: row.Timestamp,
				Networks:  networks,
			}
		}
	}

	// Convert map to slice
	result := make([]CreditsUsageHistory, 0, len(creditsMap))
	for _, v := range creditsMap {
		result = append(result, v)
	}

	standardNetworks := []string{"solana", "eclipse", "getProgramAccounts", "solana-das", "eclipse-das"}
	for i := range result {
		for _, network := range standardNetworks {
			if _, exists := result[i].Networks[network]; !exists {
				result[i].Networks[network] = 0
			}
		}
	}

	return result
}
