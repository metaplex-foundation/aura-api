package clickhouse

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/adm-metaex/aura-api/pkg/configtypes"

	"github.com/golang-migrate/migrate/v4"
	chMigrations "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Storage struct {
	conn     *sql.DB
	serverID string
}

func NewAndMigrate(config configtypes.ClickhouseConfig, serverID string) (s Storage, err error) {
	opt, err := clickhouse.ParseDSN(config.DSN)
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

	driver, err := chMigrations.WithInstance(conn, &chMigrations.Config{DatabaseName: opt.Auth.Database, MultiStatementEnabled: true})
	if err != nil {
		return s, fmt.Errorf("WithInstance: %s", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+config.MigrationsPath,
		opt.Auth.Database, driver)
	if err != nil {
		return s, fmt.Errorf("NewWithDatabaseInstance: %s", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return s, fmt.Errorf("applying up ClickHouse migrations error: %s", err)
	}

	return Storage{conn: conn, serverID: serverID}, nil
}

func (s *Storage) Close() error {
	if s.conn == nil {
		return nil
	}

	return s.conn.Close()
}
