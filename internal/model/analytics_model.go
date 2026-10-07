package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type TenantAnalyticsOverviewResponse struct {
	TotalRevenueThisMonth decimal.Decimal       `json:"total_revenue_this_month"`
	TotalActiveMembers    int64                 `json:"total_active_members"`
	TotalGroups           int64                 `json:"total_groups"`
	TotalMembersInGroups  int64                 `json:"total_members_in_groups"`
	SuccessTransactions   int64                 `json:"success_transactions"`
	RevenueChart          []DailyRevenue        `json:"revenue_chart"`
	PackagePopularity     []PackagePopularity   `json:"package_popularity"`
	RecentOrders          []RecentOrderResponse `json:"recent_orders"`
}

type DailyRevenue struct {
	Date    string          `json:"date"` // Format YYYY-MM-DD
	Revenue decimal.Decimal `json:"revenue"`
}

type PackagePopularity struct {
	PackageName string `json:"package_name"`
	Count       int64  `json:"count"`
}

type RecentOrderResponse struct {
	ID             uuid.UUID       `json:"id"`
	ExternalID     string          `json:"external_id"`
	MemberName     string          `json:"member_name"`
	MemberUsername string          `json:"member_username"`
	PackageName    string          `json:"package_name"`
	Amount         decimal.Decimal `json:"amount"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
}
