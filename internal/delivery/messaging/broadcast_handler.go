package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	jsonlib "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type BroadcastHandler struct {
	db              *gorm.DB
	broadcastRepo   repository.IBroadcastRepository
	botRepo         repository.ITelegramBotRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
}

func NewBroadcastHandler(
	db *gorm.DB,
	broadcastRepo repository.IBroadcastRepository,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
) *BroadcastHandler {
	return &BroadcastHandler{
		db:              db,
		broadcastRepo:   broadcastRepo,
		botRepo:         botRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		logger:          logger,
	}
}

type BroadcastPayload struct {
	BroadcastID string `json:"broadcast_id"`
	BotID       string `json:"bot_id"`
	ChatID      int64  `json:"chat_id"`
	MessageType string `json:"message_type"`
	MessageText string `json:"message_text"`
	FileUrl     string `json:"file_url"`
}

func (h *BroadcastHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	// 1. Parse Payload
	var payload BroadcastPayload
	if err := jsonlib.Unmarshal(body, &payload); err != nil {
		h.logger.Error("broadcast handler: failed to unmarshal payload", append(logFields, zap.Error(err))...)
		return nil
	}

	broadcastUUID, err := uuid.Parse(payload.BroadcastID)
	if err != nil {
		h.logger.Error("broadcast handler: invalid broadcast_id", append(logFields, zap.String("broadcast_id", payload.BroadcastID))...)
		return nil
	}

	h.logger.Info("broadcast handler: sending message",
		append(logFields,
			zap.String("broadcast_id", payload.BroadcastID),
			zap.Int64("chat_id", payload.ChatID),
			zap.String("message_type", payload.MessageType),
		)...,
	)

	// 2. Fetch Bot & Initialize Telegram Client
	botUUID, err := uuid.Parse(payload.BotID)
	if err != nil {
		h.logger.Error("broadcast handler: invalid bot_id", append(logFields, zap.String("bot_id", payload.BotID))...)
		return nil
	}

	bot, err := h.botRepo.FindByID(ctx, h.db, botUUID)
	if err != nil {
		h.logger.Error("broadcast handler: bot lookup failed", append(logFields, zap.Error(err))...)
		return err // Retry if database lookup fails
	}
	if bot == nil {
		h.logger.Error("broadcast handler: bot not found", append(logFields, zap.String("bot_id", payload.BotID))...)
		return nil // Non-retryable
	}

	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.logger.Error("broadcast handler: failed to decrypt token", append(logFields, zap.Error(err))...)
		return nil // Non-retryable
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.logger.Error("broadcast handler: failed to init telegram client", append(logFields, zap.Error(err))...)
		return err // Retry
	}

	var broadcast entity.Broadcast
	if err := h.db.Where("id = ?", broadcastUUID).First(&broadcast).Error; err != nil {
		h.logger.Error("broadcast handler: broadcast not found", append(logFields, zap.Error(err))...)
		return nil
	}

	if broadcast.Status == "pending" {
		h.db.Model(&entity.Broadcast{}).Where("id = ?", broadcastUUID).Update("status", "processing")
	}

	// 3. Send Message based on type
	var sendErr error

	messageText := helper.SanitizeTelegramHTML(payload.MessageText)

	switch payload.MessageType {
	case "text":
		msgConfig := tgbotapi.NewMessage(payload.ChatID, messageText)
		msgConfig.ParseMode = tgbotapi.ModeHTML
		_, sendErr = botClient.Send(ctx, msgConfig)

	case "photo":
		if broadcast.TelegramFileID != nil && *broadcast.TelegramFileID != "" {
			photoConfig := tgbotapi.NewPhoto(payload.ChatID, tgbotapi.FileID(*broadcast.TelegramFileID))
			photoConfig.Caption = messageText
			photoConfig.ParseMode = tgbotapi.ModeHTML
			_, sendErr = botClient.Send(ctx, photoConfig)
		} else {
			if payload.FileUrl == "" {
				sendErr = fmt.Errorf("missing file_url for photo broadcast")
				break
			}
			photoConfig := tgbotapi.NewPhoto(payload.ChatID, tgbotapi.FileURL(payload.FileUrl))
			photoConfig.Caption = messageText
			photoConfig.ParseMode = tgbotapi.ModeHTML
			msg, err := botClient.Send(ctx, photoConfig)
			sendErr = err

			if err == nil && len(msg.Photo) > 0 {
				fileID := msg.Photo[len(msg.Photo)-1].FileID
				h.db.Model(&entity.Broadcast{}).Where("id = ? AND telegram_file_id IS NULL", broadcastUUID).Update("telegram_file_id", fileID)
			}
		}

	case "document":
		if broadcast.TelegramFileID != nil && *broadcast.TelegramFileID != "" {
			docConfig := tgbotapi.NewDocument(payload.ChatID, tgbotapi.FileID(*broadcast.TelegramFileID))
			docConfig.Caption = messageText
			docConfig.ParseMode = tgbotapi.ModeHTML
			_, sendErr = botClient.SendDocument(ctx, docConfig)
		} else {
			if payload.FileUrl == "" {
				sendErr = fmt.Errorf("missing file_url for document broadcast")
				break
			}
			docConfig := tgbotapi.NewDocument(payload.ChatID, tgbotapi.FileURL(payload.FileUrl))
			docConfig.Caption = messageText
			docConfig.ParseMode = tgbotapi.ModeHTML
			msg, err := botClient.SendDocument(ctx, docConfig)
			sendErr = err

			if err == nil && msg.Document != nil {
				fileID := msg.Document.FileID
				h.db.Model(&entity.Broadcast{}).Where("id = ? AND telegram_file_id IS NULL", broadcastUUID).Update("telegram_file_id", fileID)
			}
		}

	default:
		sendErr = fmt.Errorf("unknown message type: %s", payload.MessageType)
	}

	// 4. Update Broadcast Stats
	if sendErr != nil {
		h.logger.Error("broadcast handler: failed to send message", append(logFields, zap.Int64("chat_id", payload.ChatID), zap.Error(sendErr))...)
		h.recordFailure(ctx, broadcastUUID, payload.ChatID, sendErr.Error())
		_, dbErr := h.broadcastRepo.IncrementCounters(ctx, h.db, broadcastUUID, false)
		if dbErr != nil {
			h.logger.Error("broadcast handler: failed to increment failed counter", append(logFields, zap.Error(dbErr))...)
		}
		return nil
	}

	// Increment sent counter
	_, dbErr := h.broadcastRepo.IncrementCounters(ctx, h.db, broadcastUUID, true)
	if dbErr != nil {
		h.logger.Error("broadcast handler: failed to increment sent counter", append(logFields, zap.Error(dbErr))...)
	}

	return nil
}

type broadcastFailure struct {
	ChatID int64  `json:"chat_id"`
	Error  string `json:"error"`
}

func (h *BroadcastHandler) recordFailure(ctx context.Context, broadcastID uuid.UUID, chatID int64, errMsg string) {
	failure := broadcastFailure{ChatID: chatID, Error: errMsg}
	b, marshalErr := json.Marshal([]broadcastFailure{failure})
	if marshalErr != nil {
		return
	}

	dbErr := h.db.WithContext(ctx).Exec(
		`UPDATE broadcasts SET failed_details = COALESCE(failed_details, '[]'::jsonb) || ?::jsonb, updated_at = NOW() WHERE id = ?`,
		string(b), broadcastID,
	).Error
	if dbErr != nil {
		h.logger.Error("broadcast handler: failed to record failure detail", zap.Error(dbErr))
	}
}
