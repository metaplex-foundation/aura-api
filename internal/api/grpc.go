package api

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	proto2 "github.com/adm-metaex/aura-api/pkg/proto"
)

type auraServer struct {
	proto2.UnsafeAuraServer

	pgStorage *postgres.Storage
	chStorage clickhouse.Storage
}

// ClickHouse

func (s *auraServer) BatchInsertStats(_ context.Context, in *proto2.BatchInsertStatsReq) (*emptypb.Empty, error) {
	// no need to handle context
	err := s.chStorage.BatchInsertStats(in.GetStats())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}

func (s *auraServer) IncreaseUserRequests(_ context.Context, in *proto2.IncreaseUserRequestsReq) (*emptypb.Empty, error) {
	err := s.chStorage.BatchInsertUserSubscriptionUsage(in.GetReqs())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}

func (s *auraServer) BatchInsertDetailedRequests(_ context.Context, in *proto2.BatchInsertDetailedRequestsReq) (*emptypb.Empty, error) {
	// no need to handle context
	err := s.chStorage.BatchInsertDetailedRequests(in.GetReq())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}
