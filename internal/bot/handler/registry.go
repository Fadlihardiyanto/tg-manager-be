package handler

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Registry struct {
	commands map[string]CommandHandler
}

func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]CommandHandler),
	}
}

func (r *Registry) Register(cmd CommandHandler) {
	r.commands[cmd.Name()] = cmd
}

func (r *Registry) HandleCommand(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	if !msg.IsCommand() {
		return nil
	}

	cmdName := msg.Command()
	handler, exists := r.commands[cmdName]
	if !exists {
		// Abaikan jika command tidak dikenal
		return nil
	}

	// Validasi Bot Role (Multi-Tenant RBAC)
	botRole := bot.BotRole
	if botRole == "" {
		botRole = "all_in_one"
	}

	isAllowed := false
	for _, role := range handler.AllowedRoles() {
		if role == botRole {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		// Abaikan karena role bot saat ini tidak diizinkan mengakses command ini
		return nil
	}

	return handler.Execute(ctx, bot, msg)
}
