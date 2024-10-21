package clickhouse

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type Storage struct {
	conn     *sql.DB
	serverID string
}

func New(dsn, serverID string) (s Storage, err error) {
	opt, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return s, fmt.Errorf("ParseDSN: %s", err)
	}
	conn := clickhouse.OpenDB(opt)
	conn.SetMaxIdleConns(10)
	conn.SetMaxOpenConns(40) //nolint:revive
	conn.SetConnMaxLifetime(time.Hour)

	// Test connection
	err = conn.Ping()
	if err != nil {
		return s, fmt.Errorf("connection ping error: %s", err)
	}

	return Storage{conn: conn, serverID: serverID}, nil
}

func (s *Storage) Close() error {
	if s.conn == nil {
		return nil
	}

	return s.conn.Close()
}
