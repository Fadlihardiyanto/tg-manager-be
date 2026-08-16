package model

import "github.com/google/uuid"

type ReportSettingRequest struct {
	Enabled      bool      `json:"enabled"`
	TargetChatID int64     `json:"target_chat_id" validate:"required,min=1"`
	BotID        uuid.UUID `json:"bot_id" validate:"required,uuid"`
	ReportTime   string    `json:"report_time" validate:"required,len=5"`
}

type ReportSettingResponse struct {
	Enabled      bool      `json:"enabled"`
	TargetChatID int64     `json:"target_chat_id"`
	BotID        uuid.UUID `json:"bot_id"`
	ReportTime   string    `json:"report_time"`
}

type ReportFailureItem struct {
	EventType      string `json:"event_type"`
	TelegramChatID *int64 `json:"telegram_chat_id,omitempty"`
	Detail         string `json:"detail"`
	CreatedAt      string `json:"created_at"`
}
