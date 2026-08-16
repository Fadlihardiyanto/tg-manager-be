package messaging

import (
	"context"
	"strings"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/reporting"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type EnforcerHandler struct {
	db              *gorm.DB
	botRepo         repository.ITelegramBotRepository
	subscriptionRepo repository.ISubscriptionRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
}

func NewEnforcerHandler(
	db *gorm.DB,
	botRepo repository.ITelegramBotRepository,
	subscriptionRepo repository.ISubscriptionRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
) *EnforcerHandler {
	return &EnforcerHandler{
		db:              db,
		botRepo:         botRepo,
		subscriptionRepo: subscriptionRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		logger:          logger,
	}
}

func (h *EnforcerHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	var payload EnforcerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("enforcer handler: failed to unmarshal payload", append(logFields, zap.Error(err))...)
		return nil // bad format, drop
	}

	h.logger.Info("enforcer handler: processing eviction task",
		append(logFields,
			zap.Int64("user_id", payload.TelegramUserID),
			zap.Int64("chat_id", payload.TelegramChatID),
		)...,
	)

	// Double-check: skip kick if user still has another active subscription covering this group
	hasAccess, err := h.subscriptionRepo.HasActiveSubscriptionForGroup(ctx, h.db, payload.TelegramUserID, payload.TelegramChatID)
	if err != nil {
		h.logger.Error("enforcer handler: failed to check group access", append(logFields, zap.Error(err))...)
		return err
	}
	if hasAccess {
		h.logger.Info("skip kick, user still has active subscription covering this group",
			append(logFields,
				zap.Int64("chat_id", payload.TelegramChatID),
				zap.Int64("user_id", payload.TelegramUserID),
			)...,
		)
		return nil
	}

	// Get Bot Credentials
	bot, err := h.botRepo.FindByID(ctx, h.db, payload.BotID)
	if err != nil || bot == nil {
		h.logger.Error("enforcer handler: failed to find bot", append(logFields, zap.Error(err))...)
		return err // retryable
	}

	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.logger.Error("enforcer handler: failed to decrypt bot token", append(logFields, zap.Error(err))...)
		return err // retryable
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.logger.Error("enforcer handler: failed to create bot client", append(logFields, zap.Error(err))...)
		return err // retryable
	}

	// 1. Kick User
	// Telegram treats very short bans as permanent bans. Use >30s to ensure temporary ban,
	// then explicitly unban to produce "soft kick" behavior.
	untilDate := time.Now().Add(35 * time.Second)
	if err := botClient.KickChatMember(ctx, payload.TelegramChatID, payload.TelegramUserID, untilDate); err != nil {
		if strings.Contains(err.Error(), "USER_NOT_PARTICIPANT") {
			h.logger.Warn("enforcer handler: user is not a participant of the group, skipping eviction",
				append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Int64("chat_id", payload.TelegramChatID))...
			)
			return nil // ignore this error, successfully consumed
		}
		// Permanent (bot di-remove dari grup / grup dibongkar / bot bukan admin):
		// retry tidak akan pernah sukses — drop event, jangan DLQ-loop.
		if telegram.IsPermanentError(err) {
			h.logger.Warn("enforcer handler: permanent telegram error, dropping eviction",
				append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Int64("chat_id", payload.TelegramChatID))...)
			reporting.Record(ctx, h.db, bot.ClientID, "enforcer.kick", "failed", payload.TelegramUserID, err.Error(), h.logger)
			return nil
		}
		h.logger.Error("enforcer handler: failed to kick chat member", append(logFields, zap.Error(err))...)
		reporting.Record(ctx, h.db, bot.ClientID, "enforcer.kick", "failed", payload.TelegramUserID, err.Error(), h.logger)
		return err
	}

	h.logger.Info("enforcer handler: successfully kicked user", append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)
	reporting.Record(ctx, h.db, bot.ClientID, "enforcer.kick", "success", payload.TelegramUserID, "", h.logger)

	// 2. Unban User (Soft Kick)
	// Memungkinkan user untuk gabung lagi di masa depan jika mereka beli paket baru
	if err := botClient.UnbanChatMember(ctx, payload.TelegramChatID, payload.TelegramUserID, false); err != nil {
		h.logger.Error("enforcer handler: failed to unban chat member (soft kick)", append(logFields, zap.Error(err))...)
		return err
	}

	h.logger.Info("enforcer handler: successfully unbanned user for future re-entry", append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)
	return nil
}
