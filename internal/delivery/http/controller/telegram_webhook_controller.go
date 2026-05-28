package controller

import (
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TelegramWebhookController struct {
	webhookUC usecase.ITelegramWebhookUseCase
	log       *zap.Logger
}

func NewTelegramWebhookController(uc usecase.ITelegramWebhookUseCase, log *zap.Logger) *TelegramWebhookController {
	return &TelegramWebhookController{
		webhookUC: uc,
		log:       log,
	}
}

// HandleIncomingUpdate receives updates from Telegram for a specific bot.
// POST /webhooks/telegram/:bot_id
func (c *TelegramWebhookController) HandleIncomingUpdate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	botIDStr := ctx.Params("bot_id")

	fmt.Println("Received Telegram webhook for bot_id:", botIDStr)

	botID, err := uuid.Parse(botIDStr)
	if err != nil {
		log.Warn("telegram webhook invalid bot id", zap.String("bot_id", botIDStr))
		return helper.BadRequest(ctx, "Invalid bot ID")
	}

	var update tgbotapi.Update
	if err := ctx.Bind().JSON(&update); err != nil {
		log.Warn("telegram webhook bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Invalid JSON payload")
	}

	if err := c.webhookUC.ProcessUpdate(ctx.Context(), botID, &update); err != nil {
		log.Error("telegram webhook process update failed", zap.Error(err))
		// Still return 200 so Telegram doesn't retry infinitely for unrecoverable errors.
		// If it's a temporary error, we could return 500 to trigger Telegram retry.
		// We'll return 200 to ack the message.
	}

	return ctx.SendStatus(fiber.StatusOK)
}
