package model

import (
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Subscription Models
type ActiveSubscriptionCheckResult struct {
	SamePackage *entity.Subscription
	AllAccess   *entity.Subscription
}

type SubscriptionResponse struct {
	ID               uuid.UUID  `json:"id"`
	TelegramUserID   uuid.UUID  `json:"telegram_user_id"`
	PackageID        uuid.UUID  `json:"package_id"`
	ClientID         uuid.UUID  `json:"client_id"`
	OrderID          *uuid.UUID `json:"order_id,omitempty"`
	Status           string     `json:"status"`
	ExpiredAt        time.Time  `json:"expired_at"`
	ActivatedAt      time.Time  `json:"activated_at"`
	AutoRenew        bool       `json:"auto_renew"`
	GracePeriodHours int        `json:"grace_period_hours"`
	KickedAt         *time.Time `json:"kicked_at,omitempty"`
	LastCheckedAt    time.Time  `json:"last_checked_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Order Models
type OrderCreateRequest struct {
	PackageID     uuid.UUID `json:"package_id" validate:"required,uuid"`
	PaymentMethod string    `json:"payment_method" validate:"required,oneof=midtrans xendit"`
}

type MemberCheckoutRequest struct {
	PackageID      uuid.UUID `json:"package_id" validate:"required,uuid"`
	TelegramUserID int64     `json:"telegram_user_id" validate:"required"`
	Username       string    `json:"username" validate:"omitempty"`
	FirstName      string    `json:"first_name" validate:"omitempty"`
	LastName       string    `json:"last_name" validate:"omitempty"`
	Phone          string    `json:"phone" validate:"omitempty"`
	DiscountCode   *string   `json:"discount_code,omitempty"`
	BotID          uuid.UUID `json:"-" validate:"-"` // bot asal checkout (dari handler)
}

type MemberCheckoutResponse struct {
	OrderID        string          `json:"order_id"`
	DBOrderID      uuid.UUID       `json:"db_order_id"`
	ExternalID     string          `json:"external_id"`
	PaymentURL     string          `json:"payment_url"`
	SnapToken      string          `json:"snap_token"`
	ClientKey      string          `json:"client_key"`
	PackageName    string          `json:"package_name"`
	DurationDays   int             `json:"duration_days"`
	OriginalAmount decimal.Decimal `json:"original_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	Amount         decimal.Decimal `json:"amount"`
}

type MemberCheckoutDetailResponse struct {
	OrderID     string          `json:"order_id"`
	SnapToken   string          `json:"snap_token"`
	PaymentURL  string          `json:"payment_url"`
	ClientKey   string          `json:"client_key"`
	Bot         string          `json:"bot"`
	Slug        string          `json:"slug"`
	Amount      decimal.Decimal `json:"amount"`
	PackageName string          `json:"package_name"`
	Status      string          `json:"status"`
}

type OrderResponse struct {
	ID             uuid.UUID       `json:"id"`
	TelegramUserID uuid.UUID       `json:"telegram_user_id"`
	PackageID      uuid.UUID       `json:"package_id"`
	ClientID       uuid.UUID       `json:"client_id"`
	SubscriptionID *uuid.UUID      `json:"subscription_id,omitempty"`
	ExternalID     string          `json:"external_id"`
	PaymentURL     string          `json:"payment_url"`
	Amount         decimal.Decimal `json:"amount"`
	Status         string          `json:"status"`
	PaymentMethod  string          `json:"payment_method"`
	PaidAt         *time.Time      `json:"paid_at,omitempty"`
	ExpiredAt      *time.Time      `json:"expired_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ── Tenant Transactions (orders JOIN packages JOIN telegram_users) ───

// TransactionFilterRequest holds query parameters for the tenant transaction list.
type TransactionFilterRequest struct {
	Page          int        `json:"page" validate:"omitempty,min=1"`
	Limit         int        `json:"limit" validate:"omitempty,min=1,max=100"`
	Status        string     `json:"status" validate:"omitempty,oneof=pending paid expired failed all"`
	Search        string     `json:"search" validate:"omitempty,max=100"`
	PackageID     uuid.UUID  `json:"package_id" validate:"omitempty,uuid"`
	PaymentMethod string     `json:"payment_method" validate:"omitempty"`
	DateFrom      *time.Time `json:"date_from" validate:"omitempty"`
	DateTo        *time.Time `json:"date_to" validate:"omitempty"`
}

// TransactionResponse is a single row in the tenant transactions table.
type TransactionResponse struct {
	ID             uuid.UUID       `json:"id"`
	ExternalID     string          `json:"external_id"`
	MemberName     string          `json:"member_name"`
	MemberUsername string          `json:"member_username"`
	TelegramUserID int64           `json:"telegram_user_id"`
	PackageID      uuid.UUID       `json:"package_id"`
	PackageName    string          `json:"package_name"`
	OriginalAmount decimal.Decimal `json:"original_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	Amount         decimal.Decimal `json:"amount"`
	PaymentMethod  string          `json:"payment_method"`
	DiscountCode   *string         `json:"discount_code,omitempty"`
	Status         string          `json:"status"`
	ReceiptURL     string          `json:"receipt_url,omitempty"`
	PaidAt         *time.Time      `json:"paid_at,omitempty"`
	ExpiredAt      *time.Time      `json:"expired_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Order Payment Webhook
type OrderPaymentWebhookRequest struct {
	ExternalID string          `json:"external_id" validate:"required"`
	Status     string          `json:"status" validate:"required,oneof=pending paid failed expired"`
	Amount     decimal.Decimal `json:"amount" validate:"required,min=0"`
	PaidAt     *time.Time      `json:"paid_at" validate:"omitempty"`
}
