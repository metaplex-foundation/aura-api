package api

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/adm-metaex/aura-api/internal/api/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/api/storage/postgres"
	auraProto "github.com/adm-metaex/aura-api/pkg/proto"
	"github.com/adm-metaex/aura-api/pkg/util"
)

type auraServer struct {
	auraProto.UnsafeAuraServer

	pgStorage *postgres.Storage
	chStorage clickhouse.Storage

	pricing   PricingPlans
	mplxPrice decimal.Decimal
}

// ClickHouse

func (s *auraServer) BatchInsertStats(_ context.Context, in *auraProto.BatchInsertStatsReq) (*emptypb.Empty, error) {
	// no need to handle context
	err := s.chStorage.BatchInsertStats(in.GetStats())
	if err != nil {
		return nil, err
	}

	return new(emptypb.Empty), nil
}

func (s *auraServer) IncreaseUserRequests(_ context.Context, in *auraProto.IncreaseUserRequestsReq) (*emptypb.Empty, error) {
	err := s.chStorage.BatchInsertUserSubscriptionUsage(in.GetReqs())
	if err != nil {
		return nil, fmt.Errorf("BatchInsertUserSubscriptionUsage: %w", err)
	}
	err = s.pgStorage.UpdateUserBalances(in)
	if err != nil {
		return nil, fmt.Errorf("UpdateUserBalances: %w", err)
	}

	return new(emptypb.Empty), nil
}

func (s *auraServer) GetUserInfo(ctx context.Context, in *auraProto.GetUserInfoReq) (*auraProto.GetUserInfoResp, error) {
	u, err := s.pgStorage.GetUserByAPIKey(ctx, in.GetApiToken())
	if err != nil {
		return nil, err
	}
	var subscriptionEndsOn *timestamppb.Timestamp
	if u.SubscriptionEndsOn != nil {
		subscriptionEndsOn = timestamppb.New(*u.SubscriptionEndsOn)
	}
	return &auraProto.GetUserInfoResp{
		User: &auraProto.UserWithTokens{
			User:               u.DynamicID,
			SubscriptionId:     u.SubscriptionID,
			MplxBalance:        u.MplxBalance,
			SubscriptionEndsOn: subscriptionEndsOn,
			Tokens:             u.APIKeys,
		},
	}, nil
}

func (s *auraServer) GetSubscriptions(ctx context.Context, _ *emptypb.Empty) (*auraProto.GetSubscriptionsResp, error) {
	// no need to handle context
	subscriptionsList, err := s.pgStorage.GetSubscriptionsList(ctx)
	if err != nil {
		return nil, err
	}
	subscriptionsListConverted := getSubscriptionsWithPricingList(subscriptionsList, s.pricing, s.mplxPrice)

	return &auraProto.GetSubscriptionsResp{
		Subscriptions: util.Map(subscriptionsListConverted, func(sub SubscriptionWithPricing) *auraProto.SubscriptionWithPricing {
			var monthlyPriceMplx int64
			if sub.Pricing.MonthlyPriceMPLX != nil {
				monthlyPriceMplx = *sub.Pricing.MonthlyPriceMPLX
			}
			return &auraProto.SubscriptionWithPricing{
				Id:              sub.ID,
				Name:            sub.Name,
				Priority:        sub.Priority,
				ApiTokensLimit:  sub.APITokensLimit,
				PrioritySupport: sub.PrioritySupport,
				Pricing: &auraProto.Pricing{
					AuraDas: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.AuraDAS.RequestsPerSecond,
						PriceMplx:         sub.Pricing.AuraDAS.PriceMPLX,
					},
					EclipseDas: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.EclipseDAS.RequestsPerSecond,
						PriceMplx:         sub.Pricing.EclipseDAS.PriceMPLX,
					},
					EclipseRpc: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.EclipseRPC.RequestsPerSecond,
						PriceMplx:         sub.Pricing.EclipseRPC.PriceMPLX,
					},
					SolanaRpc: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.SolanaRPC.RequestsPerSecond,
						PriceMplx:         sub.Pricing.SolanaRPC.PriceMPLX,
					},
					GetProgramAccounts: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.GetProgramAccounts.RequestsPerSecond,
						PriceMplx:         sub.Pricing.GetProgramAccounts.PriceMPLX,
					},
					SolanaSwqos: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.SolanaSWQOS.RequestsPerSecond,
						PriceMplx:         sub.Pricing.SolanaSWQOS.PriceMPLX,
					},
					Websocket: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.Websocket.RequestsPerSecond,
						PriceMplx:         sub.Pricing.Websocket.PriceMPLX,
					},
					MonthlyPriceMplx: monthlyPriceMplx,
				},
			}
		}),
	}, nil
}
