package handler

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CallbackHandler interface {
	Prefix() string
	AllowedRoles() []string
	Execute(ctx context.Context, bot *entity.TelegramBot, query *tgbotapi.CallbackQuery) error
}

type Registry struct {
	commands  map[string]CommandHandler
	callbacks map[string]CallbackHandler
}

func NewRegistry() *Registry {
	return &Registry{
		commands:  make(map[string]CommandHandler),
		callbacks: make(map[string]CallbackHandler),
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

func (r *Registry) RegisterCallback(handler CallbackHandler) {
	r.callbacks[handler.Prefix()] = handler
}

func (r *Registry) HandleCallback(ctx context.Context, bot *entity.TelegramBot, query *tgbotapi.CallbackQuery) error {
	if query.Data == "" {
		return nil
	}

	var matchedHandler CallbackHandler
	for prefix, handler := range r.callbacks {
		if len(query.Data) >= len(prefix) && query.Data[:len(prefix)] == prefix {
			matchedHandler = handler
			break
		}
	}

	if matchedHandler == nil {
		return nil
	}

	// Validasi Bot Role (Multi-Tenant RBAC)
	botRole := bot.BotRole
	if botRole == "" {
		botRole = "all_in_one"
	}

	isAllowed := false
	for _, role := range matchedHandler.AllowedRoles() {
		if role == botRole {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return nil
	}

	return matchedHandler.Execute(ctx, bot, query)
}
