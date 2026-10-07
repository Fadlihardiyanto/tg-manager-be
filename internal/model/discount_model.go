package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ── Layer 1: Platform Discount Request/Response ───────────────

type CreatePlatformDiscountRequest struct {
	Name                string           `json:"name" validate:"required,max=255"`
	Code                *string          `json:"code" validate:"omitempty,max=100"`
	Type                string           `json:"type" validate:"required,oneof=percentage fixed"`
	Value               decimal.Decimal  `json:"value" validate:"required,gt=0"`
	MaxDiscount         *decimal.Decimal `json:"max_discount" validate:"omitempty,gt=0"`
	MinPurchase         decimal.Decimal  `json:"min_purchase" validate:"gte=0"`
	MaxUsage            int              `json:"max_usage" validate:"gte=-1"`
	ApplicablePlanIDs   []uuid.UUID      `json:"applicable_plan_ids"`
	ApplicableClientIDs []uuid.UUID      `json:"applicable_client_ids"`
	ValidFrom           time.Time        `json:"valid_from"`
	ValidUntil          *time.Time       `json:"valid_until"`

	// Dari JWT claims
	CallerPermissions []string  `json:"-"`
	CallerRoles       []string  `json:"-" validate:"-"`
	AdminID           uuid.UUID `json:"-"`
}

type UpdatePlatformDiscountRequest struct {
	Name        string           `json:"name" validate:"omitempty,max=255"`
	Type        string           `json:"type" validate:"omitempty,oneof=percentage fixed"`
	Value       *decimal.Decimal `json:"value" validate:"omitempty,gt=0"`
	MaxDiscount *decimal.Decimal `json:"max_discount" validate:"omitempty,gt=0"`
	MinPurchase *decimal.Decimal `json:"min_purchase" validate:"omitempty,gte=0"`
	MaxUsage    *int             `json:"max_usage" validate:"omitempty,gte=-1"`
	ValidUntil  *time.Time       `json:"valid_until"`
	IsActive    *bool            `json:"is_active"`

	// Dari path param & JWT
	DiscountID        uuid.UUID `json:"-"`
	CallerPermissions []string  `json:"-"`
	CallerRoles       []string  `json:"-" validate:"-"`
}

// Request saat client checkout dengan diskon platform
type ApplyPlatformDiscountRequest struct {
	Code     string          `json:"code" validate:"required"`
	PlanID   uuid.UUID       `json:"plan_id" validate:"required,uuid"`
	ClientID uuid.UUID       `json:"-"` // dari JWT
	Amount   decimal.Decimal `json:"amount" validate:"required,gt=0"`
}

type PlatformDiscountResponse struct {
	ID                  uuid.UUID        `json:"id"`
	Name                string           `json:"name"`
	Code                *string          `json:"code"`
	Type                string           `json:"type"`
	Value               decimal.Decimal  `json:"value"`
	MaxDiscount         *decimal.Decimal `json:"max_discount"`
	MinPurchase         decimal.Decimal  `json:"min_purchase"`
	MaxUsage            int              `json:"max_usage"`
	UsedCount           int              `json:"used_count"`
	ApplicablePlanIDs   []uuid.UUID      `json:"applicable_plan_ids"`
	ApplicableClientIDs []uuid.UUID      `json:"applicable_client_ids"`
	ValidFrom           time.Time        `json:"valid_from"`
	ValidUntil          *time.Time       `json:"valid_until"`
	IsActive            bool             `json:"is_active"`
	CreatedAt           time.Time        `json:"created_at"`
}

// Response preview diskon sebelum checkout
type DiscountPreviewResponse struct {
	DiscountID     uuid.UUID       `json:"discount_id"`
	DiscountName   string          `json:"discount_name"`
	OriginalAmount decimal.Decimal `json:"original_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	FinalAmount    decimal.Decimal `json:"final_amount"`
}

// ── Layer 2: Member Discount Request/Response ─────────────────

type CreateMemberDiscountRequest struct {
	Name                 string           `json:"name" validate:"required,max=255"`
	Code                 *string          `json:"code" validate:"omitempty,max=100"`
	Type                 string           `json:"type" validate:"required,oneof=percentage fixed"`
	Value                decimal.Decimal  `json:"value" validate:"required,gt=0"`
	MaxDiscount          *decimal.Decimal `json:"max_discount" validate:"omitempty,gt=0"`
	MinPurchase          decimal.Decimal  `json:"min_purchase" validate:"gte=0"`
	MaxUsage             int              `json:"max_usage" validate:"gte=-1"`
	MaxUsagePerUser      int              `json:"max_usage_per_user" validate:"gte=1"`
	ApplicablePackageIDs []uuid.UUID      `json:"applicable_package_ids"`
	ValidFrom            time.Time        `json:"valid_from"`
	ValidUntil           *time.Time       `json:"valid_until"`

	// Dari JWT claims
	ClientID uuid.UUID `json:"-"`
}

type UpdateMemberDiscountRequest struct {
	Name            string           `json:"name" validate:"omitempty,max=255"`
	Value           *decimal.Decimal `json:"value" validate:"omitempty,gt=0"`
	MaxDiscount     *decimal.Decimal `json:"max_discount" validate:"omitempty,gt=0"`
	MinPurchase     *decimal.Decimal `json:"min_purchase" validate:"omitempty,gte=0"`
	MaxUsage        *int             `json:"max_usage" validate:"omitempty,gte=-1"`
	MaxUsagePerUser *int             `json:"max_usage_per_user" validate:"omitempty,gte=1"`
	ValidUntil      *time.Time       `json:"valid_until"`
	IsActive        *bool            `json:"is_active"`

	DiscountID uuid.UUID `json:"-"`
	ClientID   uuid.UUID `json:"-"`
}

// Request saat member apply kode promo
type ApplyMemberDiscountRequest struct {
	Code *string `json:"code"`
	// NULL = cari diskon otomatis
	PackageID      uuid.UUID       `json:"package_id" validate:"required,uuid"`
	Amount         decimal.Decimal `json:"amount" validate:"required,gt=0"`
	ClientID       uuid.UUID       `json:"-"`
	TelegramUserID uuid.UUID       `json:"-"`
}

type MemberDiscountResponse struct {
	ID                   uuid.UUID        `json:"id"`
	Name                 string           `json:"name"`
	Code                 *string          `json:"code"`
	Type                 string           `json:"type"`
	Value                decimal.Decimal  `json:"value"`
	MaxDiscount          *decimal.Decimal `json:"max_discount"`
	MinPurchase          decimal.Decimal  `json:"min_purchase"`
	MaxUsage             int              `json:"max_usage"`
	UsedCount            int              `json:"used_count"`
	MaxUsagePerUser      int              `json:"max_usage_per_user"`
	ApplicablePackageIDs []uuid.UUID      `json:"applicable_package_ids"`
	ValidFrom            time.Time        `json:"valid_from"`
	ValidUntil           *time.Time       `json:"valid_until"`
	IsActive             bool             `json:"is_active"`
	CreatedAt            time.Time        `json:"created_at"`
}
