package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// OrderToResponse converts Order entity to OrderResponse model
func OrderToResponse(order *entity.Order) *model.OrderResponse {
	if order == nil {
		return nil
	}

	return &model.OrderResponse{
		ID:             order.ID,
		TelegramUserID: order.TelegramUserID,
		PackageID:      order.PackageID,
		ClientID:       order.ClientID,
		SubscriptionID: order.SubscriptionID,
		ExternalID:     order.ExternalID,
		PaymentURL:     order.PaymentURL,
		Amount:         order.Amount,
		Status:         order.Status,
		PaymentMethod:  order.PaymentMethod,
		PaidAt:         order.PaidAt,
		ExpiredAt:      order.ExpiredAt,
		CreatedAt:      order.CreatedAt,
		UpdatedAt:      order.UpdatedAt,
	}
}

// OrdersToResponse converts multiple Order entities to OrderResponse models
func OrdersToResponse(orders []entity.Order) []model.OrderResponse {
	if len(orders) == 0 {
		return []model.OrderResponse{}
	}

	responses := make([]model.OrderResponse, len(orders))
	for i, order := range orders {
		responses[i] = *OrderToResponse(&order)
	}
	return responses
}
