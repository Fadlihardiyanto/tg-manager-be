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
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type PackagesHandler struct {
	db              *entity.Database
	packageRepo     repository.IPackageRepository
	tgUserRepo      repository.ITelegramUserRepository
	orderRepo       repository.IOrderRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewPackagesHandler(db *entity.Database, packageRepo repository.IPackageRepository, tgUserRepo repository.ITelegramUserRepository, orderRepo repository.IOrderRepository, factory telegram.BotFactory, encKey string, log *zap.Logger) *PackagesHandler {
	return &PackagesHandler{
		db:              db,
		packageRepo:     packageRepo,
		tgUserRepo:      tgUserRepo,
		orderRepo:       orderRepo,
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

	// Hanya di DM — dorong interaksi privat (lihat requirePrivateChat)
	if ok, err := requirePrivateChat(ctx, h.telegramFactory, bot, msg, h.encryptionKey); err != nil || !ok {
		return err
	}

	// Fetch active packages for this client
	packages, err := h.packageRepo.FindByClientID(ctx, h.db.Gorm, bot.ClientID, 1, 10000000000000000)
	if err != nil {
		h.log.Error("failed to fetch packages", zap.Error(err))
		return err
	}

	// Resolve member purchase counts per package (for max_purchases_per_member).
	// Member yang belum pernah checkout → tidak ada query, semua paket normal.
	var purchaseCounts map[uuid.UUID]int64
	if tu, err := h.tgUserRepo.FindByTelegramID(ctx, h.db.Gorm, msg.From.ID); err == nil && tu != nil {
		pkgIDs := make([]uuid.UUID, 0, len(packages))
		for _, p := range packages {
			pkgIDs = append(pkgIDs, p.ID)
		}
		if counts, err := h.orderRepo.CountPaidByUserAndPackages(ctx, h.db.Gorm, tu.ID, pkgIDs); err == nil {
			purchaseCounts = counts
		} else {
			h.log.Warn("failed to fetch purchase counts", zap.Error(err))
		}
	}

	// Format Response
	var responseText strings.Builder
	if len(packages) == 0 {
		responseText.WriteString("😔 Mohon maaf, saat ini belum ada paket berlangganan yang tersedia.")
	} else {
		responseText.WriteString("📦 <b>Daftar Paket Langganan Tersedia:</b>\n\n")

		printer := message.NewPrinter(language.Indonesian)
		var keyboardRows [][]tgbotapi.InlineKeyboardButton

		activeIdx := 0
		for _, pkg := range packages {
			if !pkg.IsActive {
				continue
			}
			activeIdx++

			price, _ := pkg.Price.Float64()
			priceStr := printer.Sprintf("Rp %.0f", price)

			// Misal: 1. Paket VIP - Rp 100.000 / 30 Hari
			responseText.WriteString(fmt.Sprintf("%d. <b>%s</b> — %s / %d Hari\n", activeIdx, pkg.Name, priceStr, pkg.DurationDays))

			if pkg.Description != "" {
				responseText.WriteString(fmt.Sprintf("   <i>%s</i>\n", pkg.Description))
			}
			responseText.WriteString("\n")

			if pkg.MaxPurchasesPerMember > 0 && purchaseCounts[pkg.ID] >= int64(pkg.MaxPurchasesPerMember) {
				responseText.WriteString("   🚫 <i>Sudah mencapai batas pembelian</i>\n\n")
				continue // tanpa tombol "Pilih"
			}

			// Add button for this package
			callbackData := fmt.Sprintf("pkg_sel:%s", pkg.ID.String())
			btnText := fmt.Sprintf("Pilih %s", pkg.Name)
			row := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, callbackData))
			keyboardRows = append(keyboardRows, row)
		}

		responseText.WriteString("Silakan klik menu di bawah untuk memilih paket dan melanjutkan pembayaran.")

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

		// Send HTML formatted message with keyboard
		replyMsg := tgbotapi.NewMessage(msg.Chat.ID, responseText.String())
		replyMsg.ParseMode = tgbotapi.ModeHTML
		if len(keyboardRows) > 0 {
			replyMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
		}

		if _, err := botClient.Send(ctx, replyMsg); err != nil {
			h.log.Error("failed to send packages reply", zap.Error(err))
			return err
		}

		h.log.Info("successfully replied to /packages command")
		return nil
	}

	// This path is for when len(packages) == 0
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		return err
	}
	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		return err
	}
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, responseText.String())
	replyMsg.ParseMode = tgbotapi.ModeHTML

	if _, err := botClient.Send(ctx, replyMsg); err != nil {
		h.log.Error("failed to send packages reply", zap.Error(err))
		return err
	}

	h.log.Info("successfully replied to /packages command")
	return nil
}
