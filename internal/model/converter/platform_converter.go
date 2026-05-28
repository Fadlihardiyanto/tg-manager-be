package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// PlatformPlanToResponse converts PlatformPlan entity to PlatformPlanResponse model
func PlatformPlanToResponse(p *entity.PlatformPlan) *model.PlatformPlanResponse {
	if p == nil {
		return nil
	}

	var features map[string]interface{}
	if len(p.Features) > 0 {
		features = p.Features
	} else {
		features = map[string]interface{}{}
	}

	return &model.PlatformPlanResponse{
		ID:           p.ID,
		Name:         p.Name,
		DisplayName:  p.DisplayName,
		PriceMonthly: p.PriceMonthly,
		PriceYearly:  p.PriceYearly,
		MaxBots:      p.MaxBots,
		MaxGroups:    p.MaxGroups,
		MaxPackages:  p.MaxPackages,
		MaxMembers:   p.MaxMembers,
		Features:     features,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

// ClientBillingToResponse converts ClientBilling entity to ClientBillingResponse model
func ClientBillingToResponse(b *entity.ClientBilling) *model.ClientBillingResponse {
	if b == nil {
		return nil
	}

	client := model.ClientBriefResponse{
		ID:   b.Client.ID,
		Name: b.Client.Name,
		Slug: b.Client.Slug,
	}

	platformPlan := PlatformPlanToResponse(&b.Plan)

	return &model.ClientBillingResponse{
		ID:           b.ID,
		Client:       client,
		Plan:         *platformPlan,
		Status:       b.Status,
		BillingCycle: b.BillingCycle,
		Amount:       b.Amount,
		StartedAt:    b.StartedAt,
		ExpiredAt:    b.ExpiredAt,
		CancelledAt:  b.CancelledAt,
		IsManual:     b.IsManual,
		Note:         b.Note,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}
