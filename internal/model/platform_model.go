package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Platform Plan Models
type PlatformPlanFeature struct {
	Name     string `json:"name" validate:"required"`
	Included bool   `json:"included"`
}

type PlatformPlanResponse struct {
	ID                uuid.UUID             `json:"id"`
	Name              string                `json:"name"`
	DisplayName       string                `json:"display_name"`
	PriceMonthly      decimal.Decimal       `json:"price_monthly"`
	PriceYearly       decimal.Decimal       `json:"price_yearly"`
	MaxBots           int                   `json:"max_bots"`
	MaxGroups         int                   `json:"max_groups"`
	MaxPackages       int                   `json:"max_packages"`
	MaxMembers        int                   `json:"max_members"`
	MaxCustomCommands int                   `json:"max_custom_commands"`
	Features          []PlatformPlanFeature `json:"features"`
	IsActive          bool                  `json:"is_active"`
	IsLandingPage     bool                  `json:"is_landing_page"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type CreatePlatformPlanRequest struct {
	Name              string                `json:"name" validate:"required,min=2,max=50,alphanum"`
	DisplayName       string                `json:"display_name" validate:"required,max=100"`
	PriceMonthly      decimal.Decimal       `json:"price_monthly" validate:"gte=0"`
	PriceYearly       decimal.Decimal       `json:"price_yearly" validate:"gte=0"`
	MaxBots           int                   `json:"max_bots" validate:"gte=-1"`
	MaxGroups         int                   `json:"max_groups" validate:"gte=-1"`
	MaxPackages       int                   `json:"max_packages" validate:"gte=-1"`
	MaxMembers        int                   `json:"max_members" validate:"gte=-1"`
	MaxCustomCommands int                   `json:"max_custom_commands" validate:"gte=-1"`
	Features          []PlatformPlanFeature `json:"features" validate:"dive"`
	IsActive          bool                  `json:"is_active"`
	IsLandingPage     bool                  `json:"is_landing_page"`
}

type UpdatePlatformPlanRequest struct {
	DisplayName       string                `json:"display_name" validate:"omitempty,max=100"`
	PriceMonthly      *decimal.Decimal      `json:"price_monthly" validate:"omitempty,gte=0"`
	PriceYearly       *decimal.Decimal      `json:"price_yearly" validate:"omitempty,gte=0"`
	MaxBots           *int                  `json:"max_bots" validate:"omitempty,gte=-1"`
	MaxGroups         *int                  `json:"max_groups" validate:"omitempty,gte=-1"`
	MaxPackages       *int                  `json:"max_packages" validate:"omitempty,gte=-1"`
	MaxMembers        *int                  `json:"max_members" validate:"omitempty,gte=-1"`
	MaxCustomCommands *int                  `json:"max_custom_commands" validate:"omitempty,gte=-1"`
	Features          []PlatformPlanFeature `json:"features" validate:"omitempty,dive"`
	IsActive          *bool                 `json:"is_active"`
	IsLandingPage     *bool                 `json:"is_landing_page"`
}

type PlatformPlanFilterRequest struct {
	IsActive      *bool `query:"is_active"`
	IsLandingPage *bool `query:"is_landing_page"`
	Page          int   `query:"page" validate:"omitempty,gte=1"`
	Limit         int   `query:"limit" validate:"omitempty,gte=1,lte=100"`
}

type PlatformPlanLimits struct {
	MaxBots           int `json:"max_bots"`
	MaxGroups         int `json:"max_groups"`
	MaxPackages       int `json:"max_packages"`
	MaxMembers        int `json:"max_members"`
	MaxCustomCommands int `json:"max_custom_commands"`
}
