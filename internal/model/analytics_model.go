package model

import "github.com/shopspring/decimal"

type TenantAnalyticsOverviewResponse struct {
	TotalActiveMembers int64           `json:"total_active_members"`
	TotalRevenue       decimal.Decimal `json:"total_revenue"`
	TotalGroups        int64           `json:"total_groups"`
	TotalPackages      int64           `json:"total_packages"`
}
