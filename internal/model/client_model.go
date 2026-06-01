package model

import (
	"time"

	"github.com/google/uuid"
)

// Client Models
type ClientCreateRequest struct {
	Name             string `json:"name" validate:"required,min=3"`
	Slug             string `json:"slug" validate:"required,min=3,unique"`
	Description      string `json:"description" validate:"omitempty"`
	LogoURL          string `json:"logo_url" validate:"omitempty,url"`
	SubscriptionTier string `json:"subscription_tier" validate:"omitempty,oneof=free basic pro enterprise"`
}

type ClientResponse struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	Description      string    `json:"description"`
	LogoURL          string    `json:"logo_url"`
	OwnerUserID      uuid.UUID `json:"owner_user_id"`
	IsActive         bool      `json:"is_active"`
	SubscriptionTier string    `json:"subscription_tier"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ClientUpdateRequest struct {
	Name             string `json:"name" validate:"omitempty,min=3"`
	Description      string `json:"description" validate:"omitempty"`
	LogoURL          string `json:"logo_url" validate:"omitempty,url"`
	SubscriptionTier string `json:"subscription_tier" validate:"omitempty,oneof=free basic pro enterprise"`
	IsActive         *bool  `json:"is_active" validate:"omitempty"`
}

type PaymentSettingsUpdateRequest struct {
	MidtransServerKey string `json:"midtrans_server_key" validate:"required"`
	MidtransClientKey string `json:"midtrans_client_key" validate:"required"`
	MidtransIsSandbox *bool  `json:"midtrans_is_sandbox" validate:"required"`
}

type PaymentSettingsResponse struct {
	MidtransClientKey string `json:"midtrans_client_key"`
	MidtransIsSandbox bool   `json:"midtrans_is_sandbox"`
	// We deliberately do not return MidtransServerKey for security reasons
}

// ClientUser Models
type ClientUserInviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required,oneof=admin manager viewer"`
}

type ClientUserResponse struct {
	ID         uuid.UUID  `json:"id"`
	ClientID   uuid.UUID  `json:"client_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Role       string     `json:"role"`
	IsActive   bool       `json:"is_active"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type TenantUserResponse struct {
	ID         uuid.UUID    `json:"id"`
	ClientID   uuid.UUID    `json:"client_id"`
	UserID     uuid.UUID    `json:"user_id"`
	Role       string       `json:"role"`
	IsActive   bool         `json:"is_active"`
	AcceptedAt *time.Time   `json:"accepted_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	User       UserResponse `json:"user"`
}

type ClientUserUpdateRequest struct {
	Role     string `json:"role" validate:"required,oneof=admin manager viewer"`
	IsActive *bool  `json:"is_active" validate:"omitempty"`
}
