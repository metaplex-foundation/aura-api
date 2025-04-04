package api

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/adm-metaex/aura-api/internal/storage/clickhouse"
	"github.com/adm-metaex/aura-api/internal/storage/postgres"
	"github.com/adm-metaex/aura-api/pkg/configtypes"
	"github.com/adm-metaex/aura-api/pkg/log"
	auraProto "github.com/adm-metaex/aura-api/pkg/proto"
	"github.com/adm-metaex/aura-api/pkg/util"
)

type auraServer struct {
	auraProto.UnsafeAuraServer

	pgStorage *postgres.Storage
	chStorage clickhouse.Storage

	pricing   configtypes.PricingPlans
	mplxPrice decimal.Decimal

	mu      sync.Mutex
	clients map[string]auraProto.Aura_GetUserInfoServer

	// Aura API will have copy of this channel to send updates here
	notifications chan auraProto.GetUserInfoResp
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

func (s *auraServer) GetUserInfo(stream auraProto.Aura_GetUserInfoServer) error {
	clientIdentificator := uuid.New().String()

	s.mu.Lock()
	s.clients[clientIdentificator] = stream
	s.mu.Unlock()

	// once something went wrong with user communication - drop the stream
	defer func() {
		s.mu.Lock()
		delete(s.clients, clientIdentificator)
		s.mu.Unlock()
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		u, err := s.pgStorage.GetUserByAPIKey(stream.Context(), in.GetApiToken())
		if err != nil {
			return err
		}

		var subscriptionEndsOn *timestamppb.Timestamp
		if u.SubscriptionEndsOn != nil {
			subscriptionEndsOn = timestamppb.New(*u.SubscriptionEndsOn)
		}

		err = stream.Send(&auraProto.GetUserInfoResp{
			User: &auraProto.UserWithTokens{
				User:               u.DynamicID,
				SubscriptionId:     u.SubscriptionID,
				MplxBalance:        u.MplxBalance,
				SubscriptionEndsOn: subscriptionEndsOn,
				Tokens:             u.ActiveAPIKeys,
				DeletedTokens:      u.DeletedAPIKeys,
				DeprecatedTokens:   u.DeprecatedAPIKeys,
			},
		})
	}
}

func (s *auraServer) trackNotifications() {
	for update := range s.notifications {
		s.mu.Lock()

		for id, stream := range s.clients {
			err := stream.Send(&update)
			if err != nil {
				log.Logger.API.Error("Error during sending update to the stream: %s", err)
				log.Logger.API.Warn("terminating the stream")
				delete(s.clients, id)
			}
		}
		s.mu.Unlock()
	}
}

func (s *auraServer) GetAllUsers(_ *emptypb.Empty, stream auraProto.Aura_GetAllUsersServer) error {
	var startFrom int64
	startFrom = 1
	limit := 500
	for {
		users, err := s.pgStorage.GetUsersWithKeys(stream.Context(), startFrom, int16(limit))
		if err != nil {
			return err
		}

		if len(users) == 0 {
			break
		}

		for _, usr := range users {
			var subscriptionEndsOn *timestamppb.Timestamp
			if usr.SubscriptionEndsOn != nil {
				subscriptionEndsOn = timestamppb.New(*usr.SubscriptionEndsOn)
			}

			err = stream.Send(&auraProto.GetUserInfoResp{
				User: &auraProto.UserWithTokens{
					User:               usr.DynamicID,
					SubscriptionId:     usr.SubscriptionID,
					MplxBalance:        usr.MplxBalance,
					SubscriptionEndsOn: subscriptionEndsOn,
					Tokens:             usr.APIKeys,
				},
			})
			if err != nil {
				return err
			}
		}

		startFrom = users[len(users)-1].UserID + 1
	}
	return nil
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
					SolanaDas: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.SolanaDAS.RequestsPerSecond,
						PriceMplx:         sub.Pricing.SolanaDAS.PriceMPLX,
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
					SolanaGetProgramAccounts: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.SolanaGetProgramAccounts.RequestsPerSecond,
						PriceMplx:         sub.Pricing.SolanaGetProgramAccounts.PriceMPLX,
					},
					EclipseGetProgramAccounts: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.EclipseGetProgramAccounts.RequestsPerSecond,
						PriceMplx:         sub.Pricing.EclipseGetProgramAccounts.PriceMPLX,
					},
					SolanaSwqos: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.SolanaSWQOS.RequestsPerSecond,
						PriceMplx:         sub.Pricing.SolanaSWQOS.PriceMPLX,
					},
					EclipseSwqos: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.EclipseSWQOS.RequestsPerSecond,
						PriceMplx:         sub.Pricing.EclipseSWQOS.PriceMPLX,
					},
					SolanaWebsocket: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.SolanaWebsocket.RequestsPerSecond,
						PriceMplx:         sub.Pricing.SolanaWebsocket.PriceMPLX,
					},
					EclipseWebsocket: &auraProto.PricingModel{
						RequestsPerSecond: sub.Pricing.EclipseWebsocket.RequestsPerSecond,
						PriceMplx:         sub.Pricing.EclipseWebsocket.PriceMPLX,
					},
					MonthlyPriceMplx: monthlyPriceMplx,
				},
			}
		}),
	}, nil
}
