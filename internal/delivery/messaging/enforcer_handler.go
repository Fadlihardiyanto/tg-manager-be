package messaging

import (
	"context"
	"time"

	json "github.com/bytedance/sonic"

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
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
}

func NewEnforcerHandler(
	db *gorm.DB,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
) *EnforcerHandler {
	return &EnforcerHandler{
		db:              db,
		botRepo:         botRepo,
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
	// untilDate set to now() means they are removed but can rejoin immediately if unbanned.
	if err := botClient.KickChatMember(ctx, payload.TelegramChatID, payload.TelegramUserID, time.Now()); err != nil {
		h.logger.Error("enforcer handler: failed to kick chat member", append(logFields, zap.Error(err))...)
		return err
	}

	h.logger.Info("enforcer handler: successfully kicked user", append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)

	// 2. Unban User (Soft Kick)
	// Memungkinkan user untuk gabung lagi di masa depan jika mereka beli paket baru
	if err := botClient.UnbanChatMember(ctx, payload.TelegramChatID, payload.TelegramUserID, false); err != nil {
		h.logger.Error("enforcer handler: failed to unban chat member (soft kick)", append(logFields, zap.Error(err))...)
	} else {
		h.logger.Info("enforcer handler: successfully unbanned user for future re-entry", append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)
	}

	return nil
}
