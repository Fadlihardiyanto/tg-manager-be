package usecase

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// IAdminAnalyticsUseCase handles platform-wide metrics and reporting.
type IAdminAnalyticsUseCase interface {
	// GetPlatformOverview returns high-level metrics (total clients, revenue, active subscriptions).
	// TODO: Replace interface{} with specific model.PlatformOverviewResponse
	GetPlatformOverview(ctx context.Context, req *model.AdminPlatformOverviewRequest) (interface{}, error)

	// GetRevenueReport returns revenue data grouped by period.
	// TODO: Replace interface{} with specific model.RevenueReportResponse
	GetRevenueReport(ctx context.Context, req *model.AdminRevenueReportRequest) (interface{}, error)
}
