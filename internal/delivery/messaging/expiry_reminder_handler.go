package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/reporting"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ExpiryReminderHandler consumes expiry.reminder_*h events from the
// ExpiryReminder queue and sends a Telegram DM to the member.
type ExpiryReminderHandler struct {
	db              *gorm.DB
	packageRepo     repository.IPackageRepository
	botRepo         repository.ITelegramBotRepository
	groupRepo       repository.ITelegramGroupRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
}

func NewExpiryReminderHandler(
	db *gorm.DB,
	packageRepo repository.IPackageRepository,
	botRepo repository.ITelegramBotRepository,
	groupRepo repository.ITelegramGroupRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
) *ExpiryReminderHandler {
	return &ExpiryReminderHandler{
		db:              db,
		packageRepo:     packageRepo,
		botRepo:         botRepo,
		groupRepo:       groupRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		logger:          logger,
	}
}

func (h *ExpiryReminderHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	// 1. Parse payload
	var payload ExpiryReminderPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("expiry reminder handler: failed to unmarshal payload", append(logFields, zap.Error(err))...)
		return nil // bad format → drop permanently
	}

	h.logger.Info("expiry reminder handler: processing",
		append(logFields,
			zap.String("subscription_id", payload.SubscriptionID),
			zap.Int("reminder_hours", payload.ReminderHours),
		)...,
	)

	// 2. Fetch Package (with Groups for non-AllAccess)
	pkgID, err := uuid.Parse(payload.PackageID)
	if err != nil {
		h.logger.Error("expiry reminder handler: invalid package_id", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	pkg, err := h.packageRepo.FindByID(ctx, h.db, pkgID)
	if err != nil {
		// Package tidak ditemukan = anomali data permanen, terminal (jangan retry ke DLQ).
		// Error lain (DB down dll) tetap retryable.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Error("expiry reminder handler: package not found", append(logFields, zap.String("package_id", payload.PackageID))...)
			return nil
		}
		return fmt.Errorf("expiry reminder handler: package lookup failed: %w", err)
	}

	// 3. Resolve group names for the message body
	var groupNames []string
	clientID, err := uuid.Parse(payload.ClientID)
	if err != nil {
		h.logger.Error("expiry reminder handler: invalid client_id", append(logFields, zap.String("client_id", payload.ClientID))...)
		return nil
	}

	if pkg.IsAllAccess {
		groups, err := h.groupRepo.FindByClientID(ctx, h.db, clientID, 1, 10000000000000000)
		if err != nil {
			return fmt.Errorf("expiry reminder handler: failed to fetch groups for all-access: %w", err)
		}
		for _, g := range groups {
			groupNames = append(groupNames, g.Name)
		}
	} else {
		for _, g := range pkg.Groups {
			groupNames = append(groupNames, g.Name)
		}
	}

	// 4. Fetch the Bot — use the bot from the package's first group, or fall back to any client bot
	var botUUID uuid.UUID
	if len(pkg.Groups) > 0 {
		botUUID = pkg.Groups[0].BotUUID
	}

	var bot *entity.TelegramBot
	if botUUID != uuid.Nil {
		bot, err = h.botRepo.FindByID(ctx, h.db, botUUID)
	} else {
		bot, err = h.botRepo.FindFirstByClientID(ctx, h.db, clientID)
	}
	if err != nil || bot == nil {
		h.logger.Error("expiry reminder handler: no bot found for client", append(logFields, zap.String("client_id", payload.ClientID))...)
		return nil
	}

	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		return fmt.Errorf("expiry reminder handler: failed to decrypt bot token: %w", err)
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		return fmt.Errorf("expiry reminder handler: failed to init telegram client: %w", err)
	}

	// 5. Fetch Subscription for ExpiredAt
	subID, err := uuid.Parse(payload.SubscriptionID)
	if err != nil {
		h.logger.Error("expiry reminder handler: invalid subscription_id", append(logFields, zap.String("subscription_id", payload.SubscriptionID))...)
		return nil
	}

	var sub entity.Subscription
	if err := h.db.WithContext(ctx).First(&sub, "id = ?", subID).Error; err != nil {
		return fmt.Errorf("expiry reminder handler: failed to fetch subscription: %w", err)
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// ponytail: container tanpa tzdata → fallback UTC, jangan panic (lihat telegram_action_handler).
		loc = time.UTC
	}
	expiredStr := sub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")

	// 6. Build message
	message := h.buildReminderMessage(payload.ReminderHours, pkg.Name, expiredStr, groupNames)

	// 7. Send DM
	if err := botClient.SendMessage(ctx, payload.TelegramUserID, message); err != nil {
		h.logger.Error("expiry reminder handler: failed to send dm",
			append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Error(err))...,
		)
		// Error permanen (chat not found / bot diblokir / bot tidak bisa
		// memulai percakapan): drop event — DLQ-loop tidak akan pernah sukses.
		if telegram.IsPermanentError(err) {
			h.logger.Warn("expiry reminder handler: permanent telegram error, dropping event",
				append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)
			reporting.Record(ctx, h.db, clientID, "expiry.reminder", "failed", payload.TelegramUserID, err.Error(), h.logger)
			return nil
		}
		reporting.Record(ctx, h.db, clientID, "expiry.reminder", "failed", payload.TelegramUserID, err.Error(), h.logger)
		return fmt.Errorf("expiry reminder handler: failed to send dm: %w", err)
	}

	reporting.Record(ctx, h.db, clientID, "expiry.reminder", "success", payload.TelegramUserID, "", h.logger)
	h.logger.Info("expiry reminder handler: reminder sent successfully",
		append(logFields,
			zap.String("subscription_id", payload.SubscriptionID),
			zap.Int("reminder_hours", payload.ReminderHours),
		)...,
	)
	return nil
}

// buildReminderMessage formats the DM text based on how many hours remain.
func (h *ExpiryReminderHandler) buildReminderMessage(reminderHours int, packageName, expiredStr string, groupNames []string) string {
	var sb strings.Builder

	switch reminderHours {
	case 72:
		sb.WriteString("⏰ <b>Pengingat Langganan — 3 Hari Lagi!</b>\n\n")
	case 24:
		sb.WriteString("🔔 <b>Pengingat Langganan — Besok Habis!</b>\n\n")
	default:
		sb.WriteString(fmt.Sprintf("⚠️ <b>Pengingat Langganan — %d Jam Lagi!</b>\n\n", reminderHours))
	}

	sb.WriteString(fmt.Sprintf("Paket <b>%s</b> Anda akan kedaluwarsa pada:\n<b>%s</b>\n\n", packageName, expiredStr))

	if len(groupNames) > 0 {
		sb.WriteString("Grup yang akan terpengaruh:\n")
		for _, name := range groupNames {
			sb.WriteString(fmt.Sprintf("• %s\n", name))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Segera perpanjang langganan Anda agar tetap dapat mengakses konten eksklusif. Ketik /packages untuk melihat pilihan paket.")

	return sb.String()
}
