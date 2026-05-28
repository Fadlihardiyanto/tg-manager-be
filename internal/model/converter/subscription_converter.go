package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// SubscriptionToResponse converts Subscription entity to SubscriptionResponse model
func SubscriptionToResponse(subscription *entity.Subscription) *model.SubscriptionResponse {
	if subscription == nil {
		return nil
	}

	return &model.SubscriptionResponse{
		ID:               subscription.ID,
		TelegramUserID:   subscription.TelegramUserID,
		PackageID:        subscription.PackageID,
		ClientID:         subscription.ClientID,
		OrderID:          subscription.OrderID,
		Status:           subscription.Status,
		ExpiredAt:        subscription.ExpiredAt,
		ActivatedAt:      subscription.ActivatedAt,
		AutoRenew:        subscription.AutoRenew,
		GracePeriodHours: subscription.GracePeriodHours,
		KickedAt:         subscription.KickedAt,
		LastCheckedAt:    subscription.LastCheckedAt,
		CreatedAt:        subscription.CreatedAt,
		UpdatedAt:        subscription.UpdatedAt,
	}
}

// SubscriptionsToResponse converts multiple Subscription entities to SubscriptionResponse models
func SubscriptionsToResponse(subscriptions []entity.Subscription) []model.SubscriptionResponse {
	if len(subscriptions) == 0 {
		return []model.SubscriptionResponse{}
	}

	responses := make([]model.SubscriptionResponse, len(subscriptions))
	for i, subscription := range subscriptions {
		responses[i] = *SubscriptionToResponse(&subscription)
	}
	return responses
}
