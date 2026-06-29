package usecase

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type ITenantAnalyticsUseCase interface {
	GetOverview(ctx context.Context, clientID uuid.UUID) (*model.TenantAnalyticsOverviewResponse, error)
}

type tenantAnalyticsUseCase struct {
	db            *gorm.DB
	analyticsRepo repository.ITenantAnalyticsRepository
	log           *zap.Logger
}

func NewTenantAnalyticsUseCase(
	db *gorm.DB,
	analyticsRepo repository.ITenantAnalyticsRepository,
	log *zap.Logger,
) ITenantAnalyticsUseCase {
	return &tenantAnalyticsUseCase{
		db:            db,
		analyticsRepo: analyticsRepo,
		log:           log,
	}
}

func (uc *tenantAnalyticsUseCase) GetOverview(ctx context.Context, clientID uuid.UUID) (*model.TenantAnalyticsOverviewResponse, error) {
	var overview model.TenantAnalyticsOverviewResponse

	g, ctx := errgroup.WithContext(ctx)

	// 1. Total Active Members (Subscriptions status = 'active')
	g.Go(func() error {
		count, err := uc.analyticsRepo.CountActiveMembers(ctx, uc.db, clientID)
		if err != nil {
			uc.log.Error("failed to count active members", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.TotalActiveMembers = count
		return nil
	})

	// 2. Total Revenue This Month (orders status = 'paid' and paid_at is in current month)
	g.Go(func() error {
		now := time.Now()
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

		revenue, err := uc.analyticsRepo.SumRevenueThisMonth(ctx, uc.db, clientID, startOfMonth)
		if err != nil {
			uc.log.Error("failed to calculate revenue this month", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.TotalRevenueThisMonth = revenue
		return nil
	})

	// 3. Total Groups
	g.Go(func() error {
		count, err := uc.analyticsRepo.CountGroups(ctx, uc.db, clientID)
		if err != nil {
			uc.log.Error("failed to count groups", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.TotalGroups = count
		return nil
	})

	// 4. Total Members in Groups
	g.Go(func() error {
		total, err := uc.analyticsRepo.SumMembersInGroups(ctx, uc.db, clientID)
		if err != nil {
			uc.log.Error("failed to calculate total members in groups", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.TotalMembersInGroups = total
		return nil
	})

	// 5. Success Transactions (Count orders status = 'paid')
	g.Go(func() error {
		count, err := uc.analyticsRepo.CountSuccessTransactions(ctx, uc.db, clientID)
		if err != nil {
			uc.log.Error("failed to count success transactions", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.SuccessTransactions = count
		return nil
	})

	// 6. Revenue Chart (last 30 days)
	g.Go(func() error {
		thirtyDaysAgo := time.Now().AddDate(0, 0, -29)
		startDate := time.Date(thirtyDaysAgo.Year(), thirtyDaysAgo.Month(), thirtyDaysAgo.Day(), 0, 0, 0, 0, thirtyDaysAgo.Location())

		chartData, err := uc.analyticsRepo.GetDailyRevenueTrend(ctx, uc.db, clientID, startDate)
		if err != nil {
			uc.log.Error("failed to fetch revenue chart data", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.RevenueChart = chartData
		return nil
	})

	// 7. Recent Orders (last 5 orders)
	g.Go(func() error {
		recentOrders, err := uc.analyticsRepo.GetRecentOrders(ctx, uc.db, clientID, 5)
		if err != nil {
			uc.log.Error("failed to fetch recent orders", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.RecentOrders = recentOrders
		return nil
	})

	// 8. Package Popularity (Pie Chart)
	g.Go(func() error {
		popularity, err := uc.analyticsRepo.GetPackagePopularity(ctx, uc.db, clientID)
		if err != nil {
			uc.log.Error("failed to fetch package popularity", zap.Error(err), zap.String("client_id", clientID.String()))
			return err
		}
		overview.PackagePopularity = popularity
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &overview, nil
}


