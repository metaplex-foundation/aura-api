package tests

import (
	"context"
	"testing"
	"time"

	"github.com/adm-metaex/aura-api/internal/models"
	"github.com/adm-metaex/aura-api/pkg/rewards"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock structures
type MockPGStorage struct {
	mock.Mock
}

type MockCHStorage struct {
	mock.Mock
}

// Mock implementations for PG Storage
func (m *MockPGStorage) GetCountOfSubscriptionUsersByDay(ctx context.Context, subscriptionId int, day time.Time) (int64, error) {
	args := m.Called(ctx, subscriptionId, day)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPGStorage) GetSubscrPriceAndDurationById(ctx context.Context, subscriptionId int) (models.SubscrPriceAndDuration, error) {
	args := m.Called(ctx, subscriptionId)
	return args.Get(0).(models.SubscrPriceAndDuration), args.Error(1)
}

func (m *MockPGStorage) SaveProvidersRewards(ctx context.Context, rewards map[string]int64, day time.Time) error {
	args := m.Called(ctx, rewards, day)
	return args.Error(0)
}

func (m *MockPGStorage) GetMaxCalculatedRewardsData(ctx context.Context) (day *time.Time, err error) {
	args := m.Called(ctx)
	return args.Get(0).(*time.Time), args.Error(1)
}

// Mock implementations for CH Storage
func (m *MockCHStorage) GetDailyPayAsYouGoRequests(ctx context.Context, day time.Time) ([]models.DailyAggregatedRequests, error) {
	args := m.Called(ctx, day)
	return args.Get(0).([]models.DailyAggregatedRequests), args.Error(1)
}

func (m *MockCHStorage) GetDailySubscriptionRequests(ctx context.Context, day time.Time) ([]models.DailyAggregatedRequests, error) {
	args := m.Called(ctx, day)
	return args.Get(0).([]models.DailyAggregatedRequests), args.Error(1)
}

func (m *MockCHStorage) GetProvidersRequestsServed(ctx context.Context, day time.Time) ([]models.DailyProvidersStat, error) {
	args := m.Called(ctx, day)
	return args.Get(0).([]models.DailyProvidersStat), args.Error(1)
}

func TestCalculateRewardsForDay(t *testing.T) {
	const (
		advancedPlanId = 4
		proPlanId      = 3
	)

	ctx := context.Background()
	testDay := time.Date(2025, 2, 25, 0, 0, 0, 0, time.UTC)

	advancedPlanDetails := models.SubscrPriceAndDuration{
		SbsPriceMplx:    500000000,
		SbsDurationDays: 30,
	}

	proPlanDetails := models.SubscrPriceAndDuration{
		SbsPriceMplx:    1500000000,
		SbsDurationDays: 30,
	}

	t.Run("Basic scenario with all data types", func(t *testing.T) {
		mockPG := new(MockPGStorage)
		mockCH := new(MockCHStorage)

		calculator := rewards.New(mockPG, mockCH)

		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, advancedPlanId, testDay).Return(int64(100), nil)
		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, proPlanId, testDay).Return(int64(50), nil)

		mockPG.On("GetSubscrPriceAndDurationById", ctx, advancedPlanId).Return(advancedPlanDetails, nil)
		mockPG.On("GetSubscrPriceAndDurationById", ctx, proPlanId).Return(proPlanDetails, nil)

		payAsYouGoRequests := []models.DailyAggregatedRequests{
			{
				RequestType:  "type1",
				Chain:        "chain1",
				RequestPrice: 13,
				RequestCount: 1000,
			},

			{
				RequestType:  "type1",
				Chain:        "chain1",
				RequestPrice: 13,
				RequestCount: 500,
			},
			{
				RequestType:  "type2",
				Chain:        "chain2",
				RequestPrice: 107,
				RequestCount: 200,
			},
		}

		subscriptionRequests := []models.DailyAggregatedRequests{
			{
				RequestType:  "type1",
				Chain:        "chain1",
				RequestPrice: 13,
				RequestCount: 3000,
			},
			{
				RequestType:  "type2",
				Chain:        "chain2",
				RequestPrice: 107,
				RequestCount: 500,
			},
		}

		providersRequests := []models.DailyProvidersStat{
			{
				Provider:     "provider1",
				Chain:        "chain1",
				RequestType:  "type1",
				RequestCount: 3000, // 1000 pay-as-you-go + 2000 subscription
			},
			{
				Provider:     "provider2",
				Chain:        "chain1",
				RequestType:  "type1",
				RequestCount: 1500, // 500 pay-as-you-go + 1000 subscription
			},
			{
				Provider:     "provider1",
				Chain:        "chain2",
				RequestType:  "type2",
				RequestCount: 700, // 200 pay-as-you-go + 500 subscription
			},
		}

		mockCH.On("GetDailyPayAsYouGoRequests", ctx, testDay).Return(payAsYouGoRequests, nil)
		mockCH.On("GetDailySubscriptionRequests", ctx, testDay).Return(subscriptionRequests, nil)
		mockCH.On("GetProvidersRequestsServed", ctx, testDay).Return(providersRequests, nil)

		expectedRewards := map[string]int64{
			"provider1": 3581115423,
			"provider2": 585592076,
		}

		rewards, err := calculator.CalculateRewardsForDay(ctx, testDay)

		assert.NoError(t, err)
		assert.NotNil(t, rewards)
		assert.Equal(t, expectedRewards, rewards)
	})

	t.Run("No subscription users", func(t *testing.T) {
		mockPG := new(MockPGStorage)
		mockCH := new(MockCHStorage)
		calculator := rewards.New(mockPG, mockCH)

		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, advancedPlanId, testDay).Return(int64(0), nil)
		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, proPlanId, testDay).Return(int64(0), nil)

		mockPG.On("GetSubscrPriceAndDurationById", ctx, advancedPlanId).Return(advancedPlanDetails, nil)
		mockPG.On("GetSubscrPriceAndDurationById", ctx, proPlanId).Return(proPlanDetails, nil)

		payAsYouGoRequests := []models.DailyAggregatedRequests{
			{
				RequestType:  "type1",
				Chain:        "chain1",
				RequestPrice: 5,
				RequestCount: 1000,
			},
		}

		subscriptionRequests := []models.DailyAggregatedRequests{}

		providersRequests := []models.DailyProvidersStat{
			{
				Provider:     "provider1",
				Chain:        "chain1",
				RequestType:  "type1",
				RequestCount: 1000,
			},
		}

		mockCH.On("GetDailyPayAsYouGoRequests", ctx, testDay).Return(payAsYouGoRequests, nil)
		mockCH.On("GetDailySubscriptionRequests", ctx, testDay).Return(subscriptionRequests, nil)
		mockCH.On("GetProvidersRequestsServed", ctx, testDay).Return(providersRequests, nil)

		expectedRewards := map[string]int64{
			"provider1": 5000, // 1000 * 5
		}

		rewards, err := calculator.CalculateRewardsForDay(ctx, testDay)

		assert.NoError(t, err)
		assert.Equal(t, expectedRewards, rewards)
	})

	t.Run("No pay-as-you-go users", func(t *testing.T) {
		mockPG := new(MockPGStorage)
		mockCH := new(MockCHStorage)
		calculator := rewards.New(mockPG, mockCH)

		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, advancedPlanId, testDay).Return(int64(5), nil)
		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, proPlanId, testDay).Return(int64(5), nil)

		mockPG.On("GetSubscrPriceAndDurationById", ctx, advancedPlanId).Return(advancedPlanDetails, nil)
		mockPG.On("GetSubscrPriceAndDurationById", ctx, proPlanId).Return(proPlanDetails, nil)

		payAsYouGoRequests := []models.DailyAggregatedRequests{}

		subscriptionRequests := []models.DailyAggregatedRequests{
			{
				RequestType:  "type1",
				Chain:        "chain1",
				RequestPrice: 13,
				RequestCount: 1000,
			},
			{
				RequestType:  "type1",
				Chain:        "chain2",
				RequestPrice: 13,
				RequestCount: 1000,
			},
			{
				RequestType:  "type2",
				Chain:        "chain2",
				RequestPrice: 107,
				RequestCount: 500,
			},
			{
				RequestType:  "type2",
				Chain:        "chain1",
				RequestPrice: 107,
				RequestCount: 500,
			},
		}

		providersRequests := []models.DailyProvidersStat{
			{
				Provider:     "provider1",
				Chain:        "chain1",
				RequestType:  "type1",
				RequestCount: 500,
			},
			{
				Provider:     "provider2",
				Chain:        "chain1",
				RequestType:  "type1",
				RequestCount: 500,
			},
			{
				Provider:     "provider3",
				Chain:        "chain2",
				RequestType:  "type1",
				RequestCount: 1000,
			},
			{
				Provider:     "provider4",
				Chain:        "chain2",
				RequestType:  "type2",
				RequestCount: 250,
			},
			{
				Provider:     "provider5",
				Chain:        "chain2",
				RequestType:  "type2",
				RequestCount: 250,
			},
			{
				Provider:     "provider6",
				Chain:        "chain1",
				RequestType:  "type2",
				RequestCount: 500,
			},
		}

		mockCH.On("GetDailyPayAsYouGoRequests", ctx, testDay).Return(payAsYouGoRequests, nil)
		mockCH.On("GetDailySubscriptionRequests", ctx, testDay).Return(subscriptionRequests, nil)
		mockCH.On("GetProvidersRequestsServed", ctx, testDay).Return(providersRequests, nil)

		expectedRewards := map[string]int64{
			"provider1": 16290726,
			"provider2": 16290726,
			"provider3": 32581453,
			"provider4": 67042606,
			"provider5": 67042606,
			"provider6": 134085212,
		}

		rewards, err := calculator.CalculateRewardsForDay(ctx, testDay)

		assert.NoError(t, err)
		assert.Equal(t, expectedRewards, rewards)
	})

	t.Run("No requests served", func(t *testing.T) {
		mockPG := new(MockPGStorage)
		mockCH := new(MockCHStorage)
		calculator := rewards.New(mockPG, mockCH)

		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, advancedPlanId, testDay).Return(int64(10), nil)
		mockPG.On("GetCountOfSubscriptionUsersByDay", ctx, proPlanId, testDay).Return(int64(5), nil)

		mockPG.On("GetSubscrPriceAndDurationById", ctx, advancedPlanId).Return(advancedPlanDetails, nil)
		mockPG.On("GetSubscrPriceAndDurationById", ctx, proPlanId).Return(proPlanDetails, nil)

		mockCH.On("GetDailyPayAsYouGoRequests", ctx, testDay).Return([]models.DailyAggregatedRequests{}, nil)
		mockCH.On("GetDailySubscriptionRequests", ctx, testDay).Return([]models.DailyAggregatedRequests{}, nil)
		mockCH.On("GetProvidersRequestsServed", ctx, testDay).Return([]models.DailyProvidersStat{}, nil)

		expectedRewards := map[string]int64{}

		rewards, err := calculator.CalculateRewardsForDay(ctx, testDay)

		assert.NoError(t, err)
		assert.Equal(t, expectedRewards, rewards)
	})
}
