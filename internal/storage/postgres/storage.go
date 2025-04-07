package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adm-metaex/aura-api/internal/models"
	log "github.com/adm-metaex/aura-api/pkg/log"
	auraProto "github.com/adm-metaex/aura-api/pkg/proto"
	"github.com/go-pg/migrations/v8"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/adm-metaex/aura-api/pkg/configtypes"
)

type UsrUpdateNotifier interface {
	NotifyUserUpdate(UsrIDs models.UsrIDs) error
}

// In case we need a DB object which should not notify anyone about updates
type NullNotifier struct{}

func (n *NullNotifier) NotifyUserUpdate(UsrIDs models.UsrIDs) error {
	// Do nothing
	return nil
}

type Storage struct {
	db           orm.DB
	isTx         bool
	userNotifier UsrUpdateNotifier
	updatedUsers chan auraProto.GetUserInfoResp
}

var (
	ErrNotTx          = errors.New("not tx")
	ErrEmptyDynamicID = errors.New("empty dynamicID")
	ErrEmptyUserID    = errors.New("empty userID")
)

const (
	APIKeysLimitReachedErrorText         = "API keys limit reached"
	SubscriptionsChangeCooldownErrorText = "Subscription changes are allowed once per 24 hours."
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
		log.Logger.Postgre.Infof("PG migrated from version %d to %d", oldVersion, newVersion)
	} else {
		log.Logger.Postgre.Infof("PG migration version is %d", oldVersion)
	}

	s = Storage{
		db:           db,
		userNotifier: &NullNotifier{},
	}

	return s, nil
}

func (s *Storage) IsNotifierNil() bool {
	return s.userNotifier == nil
}

func (s *Storage) SetupUserNotifier(userNotifier UsrUpdateNotifier, updatedUsers chan auraProto.GetUserInfoResp) {
	s.userNotifier = userNotifier
	s.updatedUsers = updatedUsers
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
		db:           tx,
		isTx:         true,
		userNotifier: s.userNotifier,
		updatedUsers: s.updatedUsers,
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

func (s *Storage) NotifyUserUpdate(userIDs models.UsrIDs) (err error) {
	var u UserWithDeletedAPIKeys

	if userIDs.DynamicId != "" {
		u, err = s.GetUserWithKeysByDynamicId(context.TODO(), userIDs.DynamicId)
		if err != nil {
			log.Logger.Postgre.Error("NotifyUserUpdate GetUserWithKeysByDynamicId error: ", err)
			return err
		}
	} else if userIDs.DBId > 0 {
		u, err = s.GetUserWithKeysById(context.TODO(), userIDs.DBId)
		if err != nil {
			log.Logger.Postgre.Error("NotifyUserUpdate GetUserWithKeysByDynamicId error: ", err)
			return err
		}
	} else {
		return nil
	}

	var subscriptionEndsOn *timestamppb.Timestamp
	if u.SubscriptionEndsOn != nil {
		subscriptionEndsOn = timestamppb.New(*u.SubscriptionEndsOn)
	}

	w := auraProto.GetUserInfoResp{
		User: &auraProto.UserWithTokens{
			User:               u.DynamicID,
			SubscriptionId:     u.SubscriptionID,
			MplxBalance:        u.MplxBalance,
			SubscriptionEndsOn: subscriptionEndsOn,
			Tokens:             u.ActiveAPIKeys,
			DeletedTokens:      u.DeletedAPIKeys,
			DeprecatedTokens:   u.DeprecatedAPIKeys,
		},
	}

	s.updatedUsers <- w

	return err
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
