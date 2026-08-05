package controller

import (
	"strconv"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type BroadcastController struct {
	broadcastUC usecase.IBroadcastUseCase
	log         *zap.Logger
	validator   *validator.Validate
}

func NewBroadcastController(uc usecase.IBroadcastUseCase, log *zap.Logger, v *validator.Validate) *BroadcastController {
	return &BroadcastController{
		broadcastUC: uc,
		log:         log,
		validator:   v,
	}
}

// Create godoc
// POST /api/v1/tenant/bots/:bot_id/broadcasts
func (c *BroadcastController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("broadcast controller create request")

	clientID := middleware.GetTenantClientID(ctx)

	botIDStr := ctx.Params("bot_id")
	botID, err := uuid.Parse(botIDStr)
	if err != nil {
		return helper.BadRequest(ctx, "Bot ID tidak valid")
	}

	var req model.CreateBroadcastRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("broadcast controller create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	req.BotID = botID

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("broadcast controller create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	// Batasan karakter response_text sesuai target_type/message_type
	textLen := len([]rune(req.MessageText))
	if req.MessageType == "text" {
		if textLen > 4096 {
			return helper.BadRequest(ctx, "Panjang isi pesan teks maksimal 4096 karakter")
		}
	} else {
		if textLen > 1024 {
			return helper.BadRequest(ctx, "Panjang keterangan (caption) maksimal 1024 karakter")
		}
	}

	result, err := c.broadcastUC.Create(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("broadcast controller create failed", zap.Error(err))
		return err
	}

	log.Info("broadcast controller create succeeded", zap.String("broadcast_id", result.ID.String()))
	return helper.Created(ctx, "Broadcast berhasil dibuat dan mulai diproses", result)
}

// List godoc
// GET /api/v1/tenant/bots/:bot_id/broadcasts
func (c *BroadcastController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("broadcast controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	botIDStr := ctx.Params("bot_id")
	botID, err := uuid.Parse(botIDStr)
	if err != nil {
		return helper.BadRequest(ctx, "Bot ID tidak valid")
	}

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var req model.BroadcastFilterRequest
	req.BotID = &botID
	req.Page = page
	req.Limit = limit

	result, total, err := c.broadcastUC.List(ctx.Context(), clientID, botID, &req)
	if err != nil {
		log.Error("broadcast controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil riwayat broadcast", result, meta)
}

// GetReach godoc
// GET /api/v1/tenant/bots/:bot_id/broadcast-reach
func (c *BroadcastController) GetReach(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	clientID := middleware.GetTenantClientID(ctx)

	botIDStr := ctx.Params("bot_id")
	botID, err := uuid.Parse(botIDStr)
	if err != nil {
		return helper.BadRequest(ctx, "Bot ID tidak valid")
	}

	result, err := c.broadcastUC.GetReach(ctx.Context(), clientID, botID)
	if err != nil {
		log.Error("broadcast controller get reach failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil jangkauan broadcast", result)
}
