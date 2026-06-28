package model

import (
	"time"

	"github.com/google/uuid"
)

// TelegramBot Models
type TelegramBotCreateRequest struct {
	Token   string `json:"token" validate:"required"`
	BotRole string `json:"bot_role" validate:"omitempty,oneof=sales_only gatekeeper_only all_in_one"`
}

type TelegramBotResponse struct {
	ID            uuid.UUID `json:"id"`
	ClientID      uuid.UUID `json:"client_id"`
	TelegramBotID int64     `json:"telegram_bot_id"`
	Username      string    `json:"username"`
	BotRole       string    `json:"bot_role"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type TelegramBotUpdateRequest struct {
	BotRole  *string `json:"bot_role" validate:"omitempty,oneof=sales_only gatekeeper_only all_in_one"`
	IsActive *bool   `json:"is_active" validate:"omitempty"`
}

// Group Models
type GroupCreateRequest struct {
	BotID          uuid.UUID `json:"bot_id" validate:"required,uuid"`
	TelegramChatID int64     `json:"telegram_chat_id" validate:"required"`
	Name           string    `json:"name" validate:"required,min=3"`
}

type GroupResponse struct {
	ID             uuid.UUID `json:"id"`
	ClientID       uuid.UUID `json:"client_id"`
	BotID          uuid.UUID `json:"bot_id"`
	BotUsername    string    `json:"bot_username,omitempty"`
	BotRole        string    `json:"bot_role,omitempty"`
	TelegramChatID int64     `json:"telegram_chat_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	MemberCount    int       `json:"member_count"`
	IsActive       bool      `json:"is_active"`
	InactiveReason string    `json:"inactive_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type GroupUpdateRequest struct {
	BotID uuid.UUID `json:"bot_id" validate:"omitempty,uuid"`
	Name  string    `json:"name" validate:"omitempty,min=3"`
}

type GroupConnectTokenResponse struct {
	Token       string `json:"token"`
	ExpiresIn   int    `json:"expires_in"`
	BotUsername string `json:"bot_username"`
}
