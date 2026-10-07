package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type ClientCheckoutPlanRequest struct {
	PlanID       uuid.UUID `json:"plan_id" validate:"required,uuid"`
	BillingCycle string    `json:"billing_cycle" validate:"required,oneof=monthly yearly"`
	DiscountCode *string   `json:"discount_code"` // opsional
	ClientID     uuid.UUID `json:"-"`
}

// Digunakan hanya oleh usecase/controller, bukan ekspor di sini.
// Struct ini sudah didefinisikan di admin_model.go sebagai AdminAssignPlanRequest.
// Sengaja dihapus dari sini untuk menghindari duplikasi.

// Webhook dari Midtrans
type MidtransWebhookRequest struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
	SignatureKey      string `json:"signature_key"`
	StatusCode        string `json:"status_code"`
	FraudStatus       string `json:"fraud_status"`
	RawNotification   string `json:"-"` // raw JSON body untuk disimpan ke JSONB
}

// Cancel billing
type CancelBillingRequest struct {
	BillingID         uuid.UUID `json:"billing_id" validate:"required,uuid"`
	Reason            string    `json:"reason"`
	CallerPermissions []string  `json:"-"`
	CallerRoles       []string  `json:"-" validate:"-"`
	AdminID           uuid.UUID `json:"-"`
}

// List billing (admin)
type AdminListBillingRequest struct {
	ClientID          *uuid.UUID `query:"client_id"`
	Status            string     `query:"status"`
	Page              int        `query:"page"`
	Limit             int        `query:"limit"`
	CallerPermissions []string   `json:"-"`
	CallerRoles       []string   `json:"-" validate:"-"`
}

// ── Response ─────────────────────────────────────────────────

type ClientBillingResponse struct {
	ID             uuid.UUID            `json:"id"`
	Client         ClientBriefResponse  `json:"client"`
	Plan           PlatformPlanResponse `json:"plan"`
	Usage          *PlatformPlanUsage   `json:"usage,omitempty"`
	Status         string               `json:"status"`
	BillingCycle   string               `json:"billing_cycle"`
	OriginalAmount decimal.Decimal      `json:"original_amount"`
	DiscountAmount decimal.Decimal      `json:"discount_amount"`
	Amount         decimal.Decimal      `json:"amount"`
	StartedAt      time.Time            `json:"started_at"`
	ExpiredAt      time.Time            `json:"expired_at"`
	CancelledAt    *time.Time           `json:"cancelled_at,omitempty"`
	CancelledBy    *AdminBriefResponse  `json:"cancelled_by,omitempty"`
	UpdatedBy      *AdminBriefResponse  `json:"updated_by,omitempty"`
	PaidAt         *time.Time           `json:"paid_at"`
	PaymentURL     string               `json:"payment_url,omitempty"`
	ReceiptURL     string               `json:"receipt_url,omitempty"`
	IsManual       bool                 `json:"is_manual"`
	Note           string               `json:"note,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	OrderID        string               `json:"order_id,omitempty"`
	SnapToken      string               `json:"snap_token,omitempty"`
	ClientKey      string               `json:"client_key,omitempty"`
}

type CancelByAdminResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ClientBriefResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type AdminBriefResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Response setelah checkout — berisi payment URL untuk redirect
type CheckoutResponse struct {
	OrderID        string          `json:"order_id"`
	ClientKey      string          `json:"client_key"`
	BillingID      uuid.UUID       `json:"billing_id"`
	ExternalID     string          `json:"external_id"`
	PaymentURL     string          `json:"payment_url"`
	SnapToken      string          `json:"snap_token"`
	OriginalAmount decimal.Decimal `json:"original_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	Amount         decimal.Decimal `json:"amount"`
	ExpiredAt      time.Time       `json:"expired_at"`
}
