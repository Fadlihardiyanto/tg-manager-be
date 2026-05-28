package usecase

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// IAdminBillingUseCase handles platform plans and client billing overrides.
type IAdminBillingUseCase interface {
	// ListPlans returns all platform plans.
	ListPlans(ctx context.Context, req *model.AdminListPlansRequest) ([]model.PlatformPlanResponse, error)

	// AssignPlan overrides or assigns a billing plan to a client manually.
	// TODO: Replace interface{} with specific model.AssignPlanRequest
	AssignPlan(ctx context.Context, req *model.AdminAssignPlanRequest) error
}
