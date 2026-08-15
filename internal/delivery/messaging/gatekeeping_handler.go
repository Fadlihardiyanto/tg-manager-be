package messaging

import (
	"context"
	"fmt"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// GatekeepingPayload is the message payload published by the Webhook usecase.
type GatekeepingPayload struct {
	BotID          uuid.UUID `json:"bot_id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	TelegramChatID int64     `json:"telegram_chat_id"`
}

type GatekeepingHandler struct {
	db              *gorm.DB
	subRepo         repository.ISubscriptionRepository
	botRepo         repository.ITelegramBotRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
}

func NewGatekeepingHandler(
	db *gorm.DB,
	subRepo repository.ISubscriptionRepository,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
) *GatekeepingHandler {
	return &GatekeepingHandler{
		db:              db,
		subRepo:         subRepo,
		botRepo:         botRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		logger:          logger,
	}
}

func (h *GatekeepingHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	var payload GatekeepingPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("gatekeeping handler: failed to unmarshal payload", append(logFields, zap.Error(err))...)
		// Return nil so it doesn't loop forever in DLQ, bad format is permanent
		return nil
	}

	h.logger.Info("gatekeeping handler: processing join request",
		append(logFields,
			zap.Int64("user_id", payload.TelegramUserID),
			zap.Int64("chat_id", payload.TelegramChatID),
		)...,
	)

	// 1. Cek apakah user punya subscription aktif untuk grup ini
	hasAccess, err := h.subRepo.HasActiveSubscriptionForGroup(ctx, h.db, payload.TelegramUserID, payload.TelegramChatID)
	if err != nil {
		h.logger.Error("gatekeeping handler: failed to check active subscription", append(logFields, zap.Error(err))...)
		return err // Retryable
	}

	// 2. Dapatkan kredensial Bot
	bot, err := h.botRepo.FindByID(ctx, h.db, payload.BotID)
	if err != nil {
		h.logger.Error("gatekeeping handler: failed to find bot", append(logFields, zap.Error(err))...)
		return err // Retryable
	}
	if bot == nil {
		err := fmt.Errorf("gatekeeping handler: bot not found")
		h.logger.Error("gatekeeping handler: failed to find bot", append(logFields, zap.Error(err))...)
		return err // Retryable
	}

	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.logger.Error("gatekeeping handler: failed to decrypt bot token", append(logFields, zap.Error(err))...)
		return err // Retryable
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.logger.Error("gatekeeping handler: failed to create bot client", append(logFields, zap.Error(err))...)
		return err // Retryable
	}

	// 3. Approve atau Decline
	// The botClient automatically respects the 25 req/sec rate limit
	if hasAccess {
		h.logger.Info("approving chat join request", append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Int64("chat_id", payload.TelegramChatID))...)
		if err := botClient.ApproveChatJoinRequest(ctx, payload.TelegramChatID, payload.TelegramUserID); err != nil {
			h.logger.Error("gatekeeping handler: failed to approve join request", append(logFields, zap.Error(err))...)
			return err
		}
	} else {
		h.logger.Info("declining chat join request (no active sub)", append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Int64("chat_id", payload.TelegramChatID))...)
		if err := botClient.DeclineChatJoinRequest(ctx, payload.TelegramChatID, payload.TelegramUserID); err != nil {
			h.logger.Error("gatekeeping handler: failed to decline join request", append(logFields, zap.Error(err))...)
			return err
		}
	}

	h.logger.Info("gatekeeping handler: finished processing join request", append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)
	return nil
}
