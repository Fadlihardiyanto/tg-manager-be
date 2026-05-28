package handler

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type StartHandler struct {
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewStartHandler(factory telegram.BotFactory, encKey string, log *zap.Logger) *StartHandler {
	return &StartHandler{
		telegramFactory: factory,
		encryptionKey:   encKey,
		log:             log,
	}
}

func (h *StartHandler) Name() string {
	return "start"
}

func (h *StartHandler) AllowedRoles() []string {
	// Sesuai logika: gatekeeper_only mengabaikan message, jadi hanya role ini yang boleh
	return []string{"all_in_one", "sales_only"}
}

func (h *StartHandler) Execute(ctx context.Context, bot *entity.TelegramBot, message *tgbotapi.Message) error {
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.log.Error("failed to decrypt token", zap.Error(err))
		return err
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.log.Error("failed to init bot client", zap.Error(err))
		return err
	}

	replyMsg := "Halo! Selamat datang di bot ini. 😊"
	if err := botClient.SendMessage(ctx, message.Chat.ID, replyMsg); err != nil {
		h.log.Error("failed to send reply", zap.Error(err))
		return err
	}

	h.log.Info("successfully replied to /start command", zap.Int64("user_id", message.From.ID))
	return nil
}
