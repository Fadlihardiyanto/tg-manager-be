package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

const startReplyMessage = `Halo! Selamat datang di bot ini. 😊

Berikut adalah perintah yang bisa Anda gunakan:
/start - Menampilkan pesan sambutan ini
/packages - Melihat dan membeli paket langganan
/mysub (atau /status) - Melihat status langganan Anda saat ini
/myorders - Melihat riwayat dan status pembayaran Anda`

type StartHandler struct {
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
	db              *entity.Database
	commandRepo     repository.ICustomCommandRepository
}

func NewStartHandler(factory telegram.BotFactory, encKey string, log *zap.Logger, db *entity.Database, commandRepo repository.ICustomCommandRepository) *StartHandler {
	return &StartHandler{
		telegramFactory: factory,
		encryptionKey:   encKey,
		log:             log,
		db:              db,
		commandRepo:     commandRepo,
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

	reply := startReplyMessage

	customCmds, err := h.commandRepo.FindByClientID(ctx, h.db.Gorm, bot.ClientID, &bot.ID, ptrBool(true), 1, 50)
	if err != nil {
		h.log.Warn("failed to load custom commands for /start", zap.Error(err))
	} else if len(customCmds) > 0 {
		var sb strings.Builder
		sb.WriteString(reply)
		sb.WriteString("\n\n📋 Perintah khusus:")
		for _, cmd := range customCmds {
			sb.WriteString(fmt.Sprintf("\n%s — %s", cmd.CommandTrigger, truncateDescription(cmd.ResponseText, 50)))
		}
		reply = sb.String()
	}

	if err := botClient.SendMessage(ctx, message.Chat.ID, reply); err != nil {
		h.log.Error("failed to send reply", zap.Error(err))
		return err
	}

	h.log.Info("successfully replied to /start command", zap.Int64("user_id", message.From.ID))
	return nil
}

func ptrBool(b bool) *bool {
	return &b
}

func truncateDescription(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
