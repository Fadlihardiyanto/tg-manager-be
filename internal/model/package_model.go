package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Package Models
type PackageCreateRequest struct {
	Name                  string          `json:"name" validate:"required,min=3"`
	Description           string          `json:"description" validate:"omitempty"`
	Price                 decimal.Decimal `json:"price" validate:"required,min=0"`
	DurationDays          int             `json:"duration_days" validate:"required,min=1"`
	IsAllAccess           bool            `json:"is_all_access" validate:"omitempty"`
	MaxPurchasesPerMember int             `json:"max_purchases_per_member" validate:"omitempty,min=0"`
}

type PackageResponse struct {
	ID                    uuid.UUID       `json:"id"`
	ClientID              uuid.UUID       `json:"client_id"`
	Name                  string          `json:"name"`
	Description           string          `json:"description"`
	Price                 decimal.Decimal `json:"price"`
	DurationDays          int             `json:"duration_days"`
	IsAllAccess           bool            `json:"is_all_access"`
	IsActive              bool            `json:"is_active"`
	MaxPurchasesPerMember int             `json:"max_purchases_per_member"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	Groups                []GroupResponse `json:"groups,omitempty"`
}

type PackageUpdateRequest struct {
	Name                  string           `json:"name" validate:"omitempty,min=3"`
	Description           *string          `json:"description" validate:"omitempty"`
	Price                 *decimal.Decimal `json:"price" validate:"omitempty,min=0"`
	DurationDays          int              `json:"duration_days" validate:"omitempty,min=1"`
	IsAllAccess           *bool            `json:"is_all_access" validate:"omitempty"`
	IsActive              *bool            `json:"is_active" validate:"omitempty"`
	MaxPurchasesPerMember *int             `json:"max_purchases_per_member" validate:"omitempty,min=0"`
}

// Package-Group Association
type PackageGroupAssociateRequest struct {
	GroupIDs []uuid.UUID `json:"group_ids" validate:"required"`
}

type PackageFilterRequest struct {
	Page        int    `json:"page"`
	Limit       int    `json:"limit"`
	Search      string `json:"search"`
	IsAllAccess []bool `json:"is_all_access"`
	IsActive    []bool `json:"is_active"`
}
