package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// MySubHandler handles the /mysub (alias /status) command.
// It shows the member their currently active subscriptions for this tenant's bot.
type MySubHandler struct {
	db              *entity.Database
	subRepo         repository.ISubscriptionRepository
	groupRepo       repository.ITelegramGroupRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewMySubHandler(
	db *entity.Database,
	subRepo repository.ISubscriptionRepository,
	groupRepo repository.ITelegramGroupRepository,
	factory telegram.BotFactory,
	encKey string,
	log *zap.Logger,
) *MySubHandler {
	return &MySubHandler{
		db:              db,
		subRepo:         subRepo,
		groupRepo:       groupRepo,
		telegramFactory: factory,
		encryptionKey:   encKey,
		log:             log,
	}
}

func (h *MySubHandler) Name() string {
	return "mysub"
}

func (h *MySubHandler) AllowedRoles() []string {
	return []string{"all_in_one", "sales_only"}
}

func (h *MySubHandler) Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	h.log.Info("executing /mysub command",
		zap.Int64("user_id", msg.From.ID),
		zap.String("bot_id", bot.ID.String()),
	)

	// Hanya di DM — dorong interaksi privat (lihat requirePrivateChat)
	if ok, err := requirePrivateChat(ctx, h.telegramFactory, bot, msg, h.encryptionKey); err != nil || !ok {
		return err
	}

	// 1. Init Telegram client
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.log.Error("mysub handler: failed to decrypt token", zap.Error(err))
		return err
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.log.Error("mysub handler: failed to init bot client", zap.Error(err))
		return err
	}

	// 2. Fetch active subscriptions scoped to this bot's client (tenant)
	subs, err := h.subRepo.FindActiveByTelegramUserID(ctx, h.db.Gorm, msg.From.ID, bot.ClientID)
	if err != nil {
		h.log.Error("mysub handler: failed to fetch subscriptions", zap.Error(err))
		replyText := "❌ Gagal mengambil data langganan. Silakan coba beberapa saat lagi."
		if sendErr := botClient.SendMessage(ctx, msg.Chat.ID, replyText); sendErr != nil {
			h.log.Warn("mysub handler: failed to send error reply", zap.Error(sendErr))
		}
		return err
	}

	// 3. Build response
	replyText := h.buildReply(ctx, subs, bot)

	if err := botClient.SendMessage(ctx, msg.Chat.ID, replyText); err != nil {
		h.log.Error("mysub handler: failed to send reply", zap.Error(err))
		return err
	}

	h.log.Info("mysub handler: successfully replied", zap.Int64("user_id", msg.From.ID))
	return nil
}

func (h *MySubHandler) buildReply(ctx context.Context, subs []entity.Subscription, bot *entity.TelegramBot) string {
	loc, _ := time.LoadLocation("Asia/Jakarta")

	if len(subs) == 0 {
		return "📭 Anda tidak memiliki langganan aktif saat ini.\n\nKetik /packages untuk melihat paket yang tersedia."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>Status Langganan Anda</b>\n<i>Total: %d paket aktif</i>\n", len(subs)))

	for i, sub := range subs {
		sb.WriteString("\n─────────────────────\n")
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n", i+1, sub.Package.Name))

		expiredStr := sub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")
		remaining := time.Until(sub.ExpiredAt)

		// Status badge based on remaining time
		var statusBadge string
		switch {
		case remaining <= 24*time.Hour:
			statusBadge = "🔴 Hampir Habis"
		case remaining <= 72*time.Hour:
			statusBadge = "🟡 Segera Berakhir"
		default:
			statusBadge = "🟢 Aktif"
		}

		sb.WriteString(fmt.Sprintf("Status   : %s\n", statusBadge))
		sb.WriteString(fmt.Sprintf("Berakhir : <b>%s</b>\n", expiredStr))
		sb.WriteString(fmt.Sprintf("Sisa     : %s\n", formatDuration(remaining)))

		// Resolve groups
		var groupNames []string
		if sub.Package.IsAllAccess {
			groups, err := h.groupRepo.FindByClientID(ctx, h.db.Gorm, bot.ClientID, 1, 10000000000000000)
			if err == nil {
				for _, g := range groups {
					groupNames = append(groupNames, g.Name)
				}
			}
		} else {
			for _, g := range sub.Package.Groups {
				groupNames = append(groupNames, g.Name)
			}
		}

		if len(groupNames) > 0 {
			sb.WriteString(fmt.Sprintf("Akses ke : %s\n", strings.Join(groupNames, ", ")))
		}
	}

	sb.WriteString("\n─────────────────────\n")
	sb.WriteString("Ketik /packages untuk memperpanjang atau membeli paket baru.")

	return sb.String()
}

// formatDuration converts a duration into a human-readable Indonesian string.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "sudah berakhir"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24

	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%d hari %d jam", days, hours)
	case days > 0:
		return fmt.Sprintf("%d hari", days)
	case hours > 0:
		return fmt.Sprintf("%d jam", hours)
	default:
		return "kurang dari 1 jam"
	}
}

// StatusAliasHandler is a thin wrapper so /status and /mysub share the same logic.
type StatusAliasHandler struct {
	delegate *MySubHandler
}

func NewStatusAliasHandler(delegate *MySubHandler) *StatusAliasHandler {
	return &StatusAliasHandler{delegate: delegate}
}

func (h *StatusAliasHandler) Name() string { return "status" }

func (h *StatusAliasHandler) AllowedRoles() []string { return h.delegate.AllowedRoles() }

func (h *StatusAliasHandler) Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	return h.delegate.Execute(ctx, bot, msg)
}
