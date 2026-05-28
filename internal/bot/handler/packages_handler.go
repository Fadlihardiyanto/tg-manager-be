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
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type PackagesHandler struct {
	db              *entity.Database
	packageRepo     repository.IPackageRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewPackagesHandler(db *entity.Database, packageRepo repository.IPackageRepository, factory telegram.BotFactory, encKey string, log *zap.Logger) *PackagesHandler {
	return &PackagesHandler{
		db:              db,
		packageRepo:     packageRepo,
		telegramFactory: factory,
		encryptionKey:   encKey,
		log:             log,
	}
}

func (h *PackagesHandler) Name() string {
	return "packages"
}

func (h *PackagesHandler) AllowedRoles() []string {
	// Hanya "all_in_one" dan "sales_only" yang melayani penjualan
	return []string{"all_in_one", "sales_only"}
}

func (h *PackagesHandler) Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	h.log.Info("executing /packages command", zap.Int64("user_id", msg.From.ID), zap.String("bot_id", bot.ID.String()))

	// Fetch active packages for this client
	packages, err := h.packageRepo.FindByClientID(ctx, h.db.Gorm, bot.ClientID)
	if err != nil {
		h.log.Error("failed to fetch packages", zap.Error(err))
		return err
	}

	// Format Response
	var responseText strings.Builder
	if len(packages) == 0 {
		responseText.WriteString("😔 Mohon maaf, saat ini belum ada paket berlangganan yang tersedia.")
	} else {
		responseText.WriteString("📦 <b>Daftar Paket Langganan Tersedia:</b>\n\n")

		printer := message.NewPrinter(language.Indonesian)

		for i, pkg := range packages {
			if !pkg.IsActive {
				continue
			}

			priceStr := printer.Sprintf("Rp %.0f", pkg.Price)

			// Misal: 1. Paket VIP - Rp 100.000 / 30 Hari
			responseText.WriteString(fmt.Sprintf("%d. <b>%s</b> — %s / %d Hari\n", i+1, pkg.Name, priceStr, pkg.DurationDays))

			if pkg.Description != "" {
				responseText.WriteString(fmt.Sprintf("   <i>%s</i>\n", pkg.Description))
			}
			responseText.WriteString("\n")
		}

		responseText.WriteString("Silakan balas pesan ini atau klik menu di bawah untuk melanjutkan pembayaran (Fitur ini sedang dalam pengembangan 🛠️).")
	}

	// Initialize Telegram Client
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

	// Send HTML formatted message
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, responseText.String())
	replyMsg.ParseMode = tgbotapi.ModeHTML

	if _, err := botClient.GetBot().Send(replyMsg); err != nil {
		h.log.Error("failed to send packages reply", zap.Error(err))
		return err
	}

	h.log.Info("successfully replied to /packages command")
	return nil
}
