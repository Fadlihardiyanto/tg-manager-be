package controller

import (
	"crypto/subtle"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TelegramWebhookController struct {
	webhookUC     usecase.ITelegramWebhookUseCase
	log           *zap.Logger
	webhookSecret string
}

func NewTelegramWebhookController(uc usecase.ITelegramWebhookUseCase, log *zap.Logger, webhookSecret string) *TelegramWebhookController {
	return &TelegramWebhookController{
		webhookUC:     uc,
		log:           log,
		webhookSecret: webhookSecret,
	}
}

// HandleIncomingUpdate receives updates from Telegram for a specific bot.
// POST /webhooks/telegram/:bot_id
func (c *TelegramWebhookController) HandleIncomingUpdate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	botIDStr := ctx.Params("bot_id")
	botID, err := uuid.Parse(botIDStr)
	if err != nil {
		log.Warn("telegram webhook invalid bot id", zap.String("bot_id", botIDStr))
		return helper.BadRequest(ctx, "ID bot tidak valid")
	}

	if c.webhookSecret != "" {
		receivedSecret := ctx.Get("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(receivedSecret), []byte(c.webhookSecret)) != 1 {
			log.Warn("telegram webhook invalid secret token", zap.String("bot_id", botID.String()))
			return helper.Forbidden(ctx, "Webhook secret tidak valid")
		}
	}

	var update tgbotapi.Update
	if err := ctx.Bind().JSON(&update); err != nil {
		log.Warn("telegram webhook bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Payload JSON tidak valid")
	}

	if err := c.webhookUC.ProcessUpdate(ctx.Context(), botID, &update); err != nil {
		log.Error("telegram webhook process update failed", zap.Error(err))
		// Still return 200 so Telegram doesn't retry infinitely for unrecoverable errors.
		// If it's a temporary error, we could return 500 to trigger Telegram retry.
		// We'll return 200 to ack the message.
	}

	return ctx.SendStatus(fiber.StatusOK)
}
