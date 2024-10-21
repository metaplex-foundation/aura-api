package api

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"aura-api/internal/api/storage/clickhouse"
	"aura-api/internal/api/storage/postgres"
	"aura-api/internal/pkg/proto"
)

type auraServer struct {
	proto.UnsafeAuraServer

	pgStorage *postgres.Storage
	chStorage clickhouse.Storage
}

// ClickHouse

func (s *auraServer) BatchInsertStats(_ context.Context, in *proto.BatchInsertStatsReq) (*emptypb.Empty, error) {
	// no need to handle context
	err := s.chStorage.BatchInsertStats(in.GetStats())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}

func (s *auraServer) IncreaseUserRequests(_ context.Context, in *proto.IncreaseUserRequestsReq) (*emptypb.Empty, error) {
	err := s.chStorage.BatchInsertUserSubscriptionUsage(in.GetReqs())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}

func (s *auraServer) BatchInsertDetailedRequests(_ context.Context, in *proto.BatchInsertDetailedRequestsReq) (*emptypb.Empty, error) {
	// no need to handle context
	err := s.chStorage.BatchInsertDetailedRequests(in.GetReq())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}
