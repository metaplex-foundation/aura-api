package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-pg/migrations/v8"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	log "github.com/sirupsen/logrus"

	"github.com/adm-metaex/aura-api/pkg/configtypes"
)

type Storage struct {
	db   orm.DB
	isTx bool
}

var (
	ErrNotTx          = errors.New("not tx")
	ErrEmptyDynamicID = errors.New("empty dynamicID")
	ErrEmptyUserID    = errors.New("empty userID")
)

const (
	APIKeysLimitReachedErrorText         = "API keys limit reached"
	SubscriptionsChangeCooldawnErrorText = "Subscription changes are allowed once per 24 hours."
	InsufficientBalanceErrorText         = "Insufficient balance to change subscription."
	CannotSwitchSubscriptionErrorText    = "Cannot switch to the selected subscription."
)

func New(ctx context.Context, cfg configtypes.PostgresConfig) (s Storage, err error) { //nolint:gocritic
	// DialTimeout default is 5s
	options, err := pg.ParseURL(cfg.URL)
	if err != nil {
		return s, fmt.Errorf("ParseURL: %w", err)
	}
	db := pg.Connect(options).
		WithContext(ctx).
		WithTimeout(5 * time.Second)

	err = db.Ping(ctx)
	if err != nil {
		return s, fmt.Errorf("ping: %s", err)
	}

	collection := migrations.NewCollection()
	collection.DisableSQLAutodiscover(true)
	err = collection.DiscoverSQLMigrations(cfg.MigrationsPath)
	if err != nil {
		return s, fmt.Errorf("DiscoverSQLMigrations: %s", err)
	}

	err = db.RunInTransaction(ctx, func(_ *pg.Tx) (err error) {
		_, _, err = collection.Run(db, "init")
		if err != nil {
			return err
		}
		return
	})
	if err != nil {
		return s, fmt.Errorf("init migration: %s", err)
	}
	var oldVersion, newVersion int64
	err = db.RunInTransaction(ctx, func(_ *pg.Tx) (err error) {
		oldVersion, newVersion, err = collection.Run(db, "up")
		if err != nil {
			return err
		}
		return
	})
	if err != nil {
		return s, fmt.Errorf("migration: %s", err)
	}

	if newVersion != oldVersion {
		log.Infof("PG migrated from version %d to %d", oldVersion, newVersion)
	} else {
		log.Infof("PG migration version is %d", oldVersion)
	}

	return Storage{
		db: db,
	}, nil
}

func (s *Storage) BeginTx(ctx context.Context) (ss Storage, err error) {
	if s.isTx {
		return ss, errors.New("already tx")
	}

	tx, err := s.db.(*pg.DB).BeginContext(ctx)
	if err != nil {
		return ss, err
	}

	return Storage{
		db:   tx,
		isTx: true,
	}, nil
}

func (s *Storage) Rollback() error {
	if !s.isTx {
		return ErrNotTx
	}

	return s.db.(*pg.Tx).Rollback()
}

func (s *Storage) Commit(ctx context.Context) error {
	if !s.isTx {
		return ErrNotTx
	}

	return s.db.(*pg.Tx).CommitContext(ctx)
}

func IsErrViolateConstraint(err error) bool {
	var pgErr pg.Error
	return errors.As(err, &pgErr) && pgErr.IntegrityViolation()
}

func IsErrAPIKeysLimitReached(err error) bool {
	var pgErr pg.Error
	return errors.As(err, &pgErr) && pgErr.Field(77) == APIKeysLimitReachedErrorText //nolint:revive
}

func IsErrInvalidSubscriptionID(err error) bool {
	var pgErr pg.Error
	return errors.As(err, &pgErr) && pgErr.IntegrityViolation() && strings.Contains(pgErr.Error(), "users_sbs_id_fkey")
}

func UpdateSubscriptionErrorMessage(err error) *string {
	var pgErr pg.Error
	if !errors.As(err, &pgErr) {
		return nil
	}
	errorMessage := pgErr.Field(77)
	if errorMessage == InsufficientBalanceErrorText || errorMessage == SubscriptionsChangeCooldawnErrorText || errorMessage == CannotSwitchSubscriptionErrorText {
		return &errorMessage
	}

	return nil
}
