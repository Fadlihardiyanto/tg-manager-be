package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Platform Plan Models
type PlatformPlanResponse struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	DisplayName  string                 `json:"display_name"`
	PriceMonthly decimal.Decimal        `json:"price_monthly"`
	PriceYearly  decimal.Decimal        `json:"price_yearly"`
	MaxBots      int                    `json:"max_bots"`
	MaxGroups    int                    `json:"max_groups"`
	MaxPackages  int                    `json:"max_packages"`
	MaxMembers   int                    `json:"max_members"`
	Features     map[string]interface{} `json:"features"`
	IsActive     bool                   `json:"is_active"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type CreatePlatformPlanRequest struct {
	Name         string                 `json:"name" validate:"required,min=2,max=50,alphanum"`
	DisplayName  string                 `json:"display_name" validate:"required,max=100"`
	PriceMonthly decimal.Decimal        `json:"price_monthly" validate:"gte=0"`
	PriceYearly  decimal.Decimal        `json:"price_yearly" validate:"gte=0"`
	MaxBots      int                    `json:"max_bots" validate:"required,gte=-1"`
	MaxGroups    int                    `json:"max_groups" validate:"required,gte=-1"`
	MaxPackages  int                    `json:"max_packages" validate:"required,gte=-1"`
	MaxMembers   int                    `json:"max_members" validate:"required,gte=-1"`
	Features     map[string]interface{} `json:"features"`
	IsActive     bool                   `json:"is_active"`
}

type UpdatePlatformPlanRequest struct {
	DisplayName  string                 `json:"display_name" validate:"omitempty,max=100"`
	PriceMonthly *decimal.Decimal       `json:"price_monthly" validate:"omitempty,gte=0"`
	PriceYearly  *decimal.Decimal       `json:"price_yearly" validate:"omitempty,gte=0"`
	MaxBots      *int                   `json:"max_bots" validate:"omitempty,gte=-1"`
	MaxGroups    *int                   `json:"max_groups" validate:"omitempty,gte=-1"`
	MaxPackages  *int                   `json:"max_packages" validate:"omitempty,gte=-1"`
	MaxMembers   *int                   `json:"max_members" validate:"omitempty,gte=-1"`
	Features     map[string]interface{} `json:"features"`
	IsActive     *bool                  `json:"is_active"`
}

type PlatformPlanFilterRequest struct {
	IsActive *bool `query:"is_active"`
	Page     int   `query:"page" validate:"omitempty,gte=1"`
	Limit    int   `query:"limit" validate:"omitempty,gte=1,lte=100"`
}

type PlatformPlanLimits struct {
	MaxBots     int `json:"max_bots"`
	MaxGroups   int `json:"max_groups"`
	MaxPackages int `json:"max_packages"`
	MaxMembers  int `json:"max_members"`
}
