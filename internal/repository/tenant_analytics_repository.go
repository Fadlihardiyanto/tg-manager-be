package repository

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ITenantAnalyticsRepository interface {
	CountActiveMembers(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
	SumRevenueThisMonth(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, startOfMonth time.Time) (decimal.Decimal, error)
	CountGroups(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
	SumMembersInGroups(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
	CountSuccessTransactions(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
	GetDailyRevenueTrend(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, startDate time.Time) ([]model.DailyRevenue, error)
	GetPackagePopularity(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]model.PackagePopularity, error)
	GetRecentOrders(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, limit int) ([]model.RecentOrderResponse, error)
}

type TenantAnalyticsRepository struct{}

func NewTenantAnalyticsRepository() ITenantAnalyticsRepository {
	return &TenantAnalyticsRepository{}
}

func (r *TenantAnalyticsRepository) CountActiveMembers(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Table("subscriptions").
		Where("client_id = ? AND status = ? AND deleted_at IS NULL", clientID, "active").
		Count(&count).Error
	return count, err
}

func (r *TenantAnalyticsRepository) SumRevenueThisMonth(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, startOfMonth time.Time) (decimal.Decimal, error) {
	type Result struct {
		Total decimal.NullDecimal
	}
	var res Result
	err := tx.WithContext(ctx).Table("orders").
		Select("COALESCE(SUM(amount), 0) as total").
		Where("client_id = ? AND status = ? AND paid_at >= ? AND deleted_at IS NULL", clientID, "paid", startOfMonth).
		Scan(&res).Error
	if err != nil {
		return decimal.Zero, err
	}
	if res.Total.Valid {
		return res.Total.Decimal, nil
	}
	return decimal.Zero, nil
}

func (r *TenantAnalyticsRepository) CountGroups(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Table("groups").
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Count(&count).Error
	return count, err
}

func (r *TenantAnalyticsRepository) SumMembersInGroups(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var total int64
	err := tx.WithContext(ctx).Table("groups").
		Select("COALESCE(SUM(member_count), 0)").
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Scan(&total).Error
	return total, err
}

func (r *TenantAnalyticsRepository) CountSuccessTransactions(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Table("orders").
		Where("client_id = ? AND status = ? AND deleted_at IS NULL", clientID, "paid").
		Count(&count).Error
	return count, err
}

func (r *TenantAnalyticsRepository) GetDailyRevenueTrend(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, startDate time.Time) ([]model.DailyRevenue, error) {
	type QueryResult struct {
		DateStr string          `gorm:"column:date_str"`
		Revenue decimal.Decimal `gorm:"column:revenue"`
	}
	var queryResults []QueryResult

	err := tx.WithContext(ctx).Table("orders").
		Select("TO_CHAR(paid_at, 'YYYY-MM-DD') as date_str, SUM(amount) as revenue").
		Where("client_id = ? AND status = ? AND paid_at >= ? AND deleted_at IS NULL", clientID, "paid", startDate).
		Group("TO_CHAR(paid_at, 'YYYY-MM-DD')").
		Order("date_str ASC").
		Scan(&queryResults).Error
	if err != nil {
		return nil, err
	}

	revMap := make(map[string]decimal.Decimal)
	for _, q := range queryResults {
		revMap[q.DateStr] = q.Revenue
	}

	chartData := make([]model.DailyRevenue, 30)
	for i := 0; i < 30; i++ {
		t := startDate.AddDate(0, 0, i)
		dateStr := t.Format("2006-01-02")
		revenue := decimal.Zero
		if val, exists := revMap[dateStr]; exists {
			revenue = val
		}
		chartData[i] = model.DailyRevenue{
			Date:    dateStr,
			Revenue: revenue,
		}
	}
	return chartData, nil
}

func (r *TenantAnalyticsRepository) GetRecentOrders(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, limit int) ([]model.RecentOrderResponse, error) {
	var recentOrders []model.RecentOrderResponse
	err := tx.WithContext(ctx).Table("orders").
		Select(`
			orders.id,
			orders.external_id,
			TRIM(COALESCE(telegram_users.first_name, '') || ' ' || COALESCE(telegram_users.last_name, '')) as member_name,
			COALESCE(telegram_users.username, '') as member_username,
			packages.name as package_name,
			orders.amount,
			orders.status,
			orders.created_at
		`).
		Joins("LEFT JOIN telegram_users ON telegram_users.id = orders.telegram_user_id").
		Joins("LEFT JOIN packages ON packages.id = orders.package_id").
		Where("orders.client_id = ? AND orders.deleted_at IS NULL", clientID).
		Order("orders.created_at DESC").
		Limit(limit).
		Scan(&recentOrders).Error
	return recentOrders, err
}

func (r *TenantAnalyticsRepository) GetPackagePopularity(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]model.PackagePopularity, error) {
	var result []model.PackagePopularity
	err := tx.WithContext(ctx).Table("orders").
		Select("packages.name as package_name, COUNT(orders.id) as count").
		Joins("JOIN packages ON packages.id = orders.package_id").
		Where("orders.client_id = ? AND orders.status = ? AND orders.deleted_at IS NULL", clientID, "paid").
		Group("packages.id, packages.name").
		Order("count DESC").
		Scan(&result).Error
	return result, err
}
