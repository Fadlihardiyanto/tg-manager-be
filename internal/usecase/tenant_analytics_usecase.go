package usecase

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ITenantAnalyticsUseCase interface {
	GetOverview(ctx context.Context, clientID uuid.UUID) (*model.TenantAnalyticsOverviewResponse, error)
}

type tenantAnalyticsUseCase struct {
	db  *gorm.DB
	log *zap.Logger
}

func NewTenantAnalyticsUseCase(db *gorm.DB, log *zap.Logger) ITenantAnalyticsUseCase {
	return &tenantAnalyticsUseCase{
		db:  db,
		log: log,
	}
}

func (uc *tenantAnalyticsUseCase) GetOverview(ctx context.Context, clientID uuid.UUID) (*model.TenantAnalyticsOverviewResponse, error) {
	var overview model.TenantAnalyticsOverviewResponse

	// Total Active Subscriptions
	if err := uc.db.WithContext(ctx).Table("subscriptions").
		Where("client_id = ? AND status = ? AND deleted_at IS NULL", clientID, "active").
		Count(&overview.TotalActiveMembers).Error; err != nil {
		uc.log.Error("failed to count active members", zap.Error(err))
		return nil, err
	}

	// Total Revenue (Orders with status 'success')
	// Using NullDecimal to handle empty sums gracefully
	type RevenueResult struct {
		Total decimal.NullDecimal
	}
	var revResult RevenueResult
	if err := uc.db.WithContext(ctx).Table("orders").
		Select("COALESCE(SUM(final_amount), 0) as total").
		Where("client_id = ? AND status = ? AND deleted_at IS NULL", clientID, "success").
		Scan(&revResult).Error; err != nil {
		uc.log.Error("failed to calculate revenue", zap.Error(err))
		return nil, err
	}

	if revResult.Total.Valid {
		overview.TotalRevenue = revResult.Total.Decimal
	} else {
		overview.TotalRevenue = decimal.Zero
	}

	// Total Groups
	if err := uc.db.WithContext(ctx).Table("groups").
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Count(&overview.TotalGroups).Error; err != nil {
		uc.log.Error("failed to count groups", zap.Error(err))
		return nil, err
	}

	// Total Packages
	if err := uc.db.WithContext(ctx).Table("packages").
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Count(&overview.TotalPackages).Error; err != nil {
		uc.log.Error("failed to count packages", zap.Error(err))
		return nil, err
	}

	return &overview, nil
}
