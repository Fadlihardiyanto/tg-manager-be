package handler

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CommandHandler menangani text commands (e.g., /start, /bsh)
type CommandHandler interface {
	Name() string
	AllowedRoles() []string // Peran bot yang diizinkan (e.g., []string{"all_in_one", "sales_only"})
	Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error
}
