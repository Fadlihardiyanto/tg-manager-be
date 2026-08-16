package converter

import (
	"fmt"

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

// ── Tenant Transaction converters ──────────────────────────────────

// TransactionToResponse converts an Order entity (with preloaded Package, User, Discount)
// to a TransactionResponse for the tenant transactions table.
func TransactionToResponse(order *entity.Order) *model.TransactionResponse {
	if order == nil {
		return nil
	}

	// Build member display name
	memberName := order.User.FirstName
	if order.User.LastName != "" {
		memberName = fmt.Sprintf("%s %s", order.User.FirstName, order.User.LastName)
	}
	if memberName == "" {
		memberName = order.User.Username
	}
	if memberName == "" {
		memberName = fmt.Sprintf("User-%d", order.User.TelegramUserID)
	}

	// ponytail: clone pointer fields so the response does not alias the GORM entity —
	// caller mutation of resp.PaidAt/ExpiredAt/DiscountCode would corrupt the entity.
	paidAt := order.PaidAt
	if paidAt != nil {
		v := *paidAt
		paidAt = &v
	}
	expiredAt := order.ExpiredAt
	if expiredAt != nil {
		v := *expiredAt
		expiredAt = &v
	}

	resp := &model.TransactionResponse{
		ID:             order.ID,
		ExternalID:     order.ExternalID,
		MemberName:     memberName,
		MemberUsername: order.User.Username,
		TelegramUserID: order.User.TelegramUserID,
		PackageID:      order.PackageID,
		PackageName:    order.Package.Name,
		OriginalAmount: order.OriginalAmount,
		DiscountAmount: order.DiscountAmount,
		Amount:         order.Amount,
		PaymentMethod:  order.PaymentMethod,
		Status:         order.Status,
		ReceiptURL:     order.ReceiptURL,
		PaidAt:         paidAt,
		ExpiredAt:      expiredAt,
		CreatedAt:      order.CreatedAt,
	}

	// Populate discount code if available
	if order.Discount != nil && order.Discount.Code != nil {
		code := *order.Discount.Code
		resp.DiscountCode = &code
	}

	return resp
}

// TransactionsToResponse converts a slice of Order entities to TransactionResponse slice.
func TransactionsToResponse(orders []entity.Order) []model.TransactionResponse {
	if len(orders) == 0 {
		return []model.TransactionResponse{}
	}

	responses := make([]model.TransactionResponse, 0, len(orders))
	for i := range orders {
		res := TransactionToResponse(&orders[i])
		if res != nil {
			responses = append(responses, *res)
		}
	}
	return responses
}
