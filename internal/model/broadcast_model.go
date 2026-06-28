package model

import (
	"time"

	"github.com/google/uuid"
)

type BroadcastResponse struct {
	ID           uuid.UUID  `json:"id"`
	ClientID     uuid.UUID  `json:"client_id"`
	BotID        uuid.UUID  `json:"bot_id"`
	TargetType   string     `json:"target_type"`
	MessageType  string     `json:"message_type"`
	MessageText  string     `json:"message_text"`
	FileUrl      *string    `json:"file_url"`
	Status       string     `json:"status"`
	TotalTargets int        `json:"total_targets"`
	SentCount    int        `json:"sent_count"`
	FailedCount  int        `json:"failed_count"`
	ScheduledAt  *time.Time `json:"scheduled_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CreateBroadcastRequest struct {
	BotID       uuid.UUID  `json:"bot_id" validate:"required"`
	TargetType  string     `json:"target_type" validate:"required,oneof=group member"`
	MessageType string     `json:"message_type" validate:"required,oneof=text photo document"`
	MessageText string     `json:"message_text" validate:"required"`
	FileUrl     *string    `json:"file_url" validate:"omitempty,url"`
	ScheduledAt *time.Time `json:"scheduled_at" validate:"omitempty"`
}

type BroadcastFilterRequest struct {
	BotID *uuid.UUID `query:"bot_id"`
	Page  int        `query:"page" validate:"omitempty,gte=1"`
	Limit int        `query:"limit" validate:"omitempty,gte=1,lte=100"`
}
