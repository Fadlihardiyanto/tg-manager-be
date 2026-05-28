package model

import (
	"time"

	"github.com/google/uuid"
)

// Telegram User Models
type TelegramUserResponse struct {
	ID             uuid.UUID `json:"id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	Username       string    `json:"username"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Phone          string    `json:"phone"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Audit Log Models
type AuditLogResponse struct {
	ID         uuid.UUID              `json:"id"`
	ClientID   *uuid.UUID             `json:"client_id,omitempty"`
	EntityType string                 `json:"entity_type"`
	EntityID   uuid.UUID              `json:"entity_id"`
	Action     string                 `json:"action"`
	ActorType  string                 `json:"actor_type"`
	ActorID    string                 `json:"actor_id"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
}

type AuditLogFilterRequest struct {
	EntityType string    `json:"entity_type" validate:"omitempty"`
	Action     string    `json:"action" validate:"omitempty"`
	DateFrom   time.Time `json:"date_from" validate:"omitempty"`
	DateTo     time.Time `json:"date_to" validate:"omitempty"`
	Limit      int       `json:"limit" validate:"omitempty,min=1,max=100"`
	Offset     int       `json:"offset" validate:"omitempty,min=0"`
}
