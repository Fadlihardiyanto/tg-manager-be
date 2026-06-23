package model

import (
	"time"

	"github.com/google/uuid"
)

type CustomCommandResponse struct {
	ID             uuid.UUID `json:"id"`
	BotID          uuid.UUID `json:"bot_id"`
	CommandTrigger string    `json:"command_trigger"`
	ResponseType   string    `json:"response_type"`
	ResponseText   string    `json:"response_text"`
	FileUrl        *string   `json:"file_url"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateCustomCommandRequest struct {
	BotID          uuid.UUID `json:"bot_id" validate:"required"`
	CommandTrigger string    `json:"command_trigger" validate:"required,min=2,max=50"`
	ResponseType   string    `json:"response_type" validate:"required,oneof=text photo"`
	ResponseText   string    `json:"response_text" validate:"required"`
	FileUrl        *string   `json:"file_url" validate:"omitempty,url"`
}

type UpdateCustomCommandRequest struct {
	CommandTrigger *string `json:"command_trigger" validate:"omitempty,min=2,max=50"`
	ResponseType   *string `json:"response_type" validate:"omitempty,oneof=text photo"`
	ResponseText   *string `json:"response_text" validate:"omitempty"`
	FileUrl        *string `json:"file_url" validate:"omitempty,url"`
	IsActive       *bool   `json:"is_active"`
}

type CustomCommandFilterRequest struct {
	BotID    *uuid.UUID `query:"bot_id"`
	IsActive *bool      `query:"is_active"`
	Page     int        `query:"page" validate:"omitempty,gte=1"`
	Limit    int        `query:"limit" validate:"omitempty,gte=1,lte=100"`
}
