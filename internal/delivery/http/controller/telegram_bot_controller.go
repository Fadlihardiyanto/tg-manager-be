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

type TelegramBotController struct {
	botUC     usecase.ITelegramBotUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewTelegramBotController(uc usecase.ITelegramBotUseCase, log *zap.Logger, v *validator.Validate) *TelegramBotController {
	return &TelegramBotController{
		botUC:     uc,
		log:       log,
		validator: v,
	}
}

// Create godoc
// POST /api/v1/tenant/bots
func (c *TelegramBotController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("bot controller create request")

	clientID := middleware.GetTenantClientID(ctx)

	var req model.TelegramBotCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("bot controller create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("bot controller create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.botUC.Create(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("bot controller create failed", zap.Error(err))
		return err
	}
	if result == nil {
		log.Error("bot controller create returned nil result")
		return helper.InternalError(ctx, "Terjadi kesalahan pada server")
	}

	log.Info("bot controller create succeeded", zap.String("bot_id", result.ID.String()))
	return helper.Created(ctx, "Bot berhasil ditambahkan", result)
}

// List godoc
// GET /api/v1/tenant/bots
func (c *TelegramBotController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("bot controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	page = clampPage(page)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, total, err := c.botUC.FindAllByClient(ctx.Context(), clientID, page, limit)
	if err != nil {
		log.Error("bot controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar bot", result, meta)
}

// Get godoc
// GET /api/v1/tenant/bots/:id
func (c *TelegramBotController) Get(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("bot controller get request")

	clientID := middleware.GetTenantClientID(ctx)
	botUUID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("bot controller get invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID bot tidak valid")
	}

	result, err := c.botUC.FindByID(ctx.Context(), clientID, botUUID)
	if err != nil {
		log.Error("bot controller get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil detail bot", result)
}

// Update godoc
// PUT /api/v1/tenant/bots/:id
func (c *TelegramBotController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("bot controller update request")

	clientID := middleware.GetTenantClientID(ctx)
	botUUID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("bot controller update invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID bot tidak valid")
	}

	var req model.TelegramBotUpdateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("bot controller update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("bot controller update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.botUC.Update(ctx.Context(), clientID, botUUID, &req)
	if err != nil {
		log.Error("bot controller update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Bot berhasil diperbarui", result)
}

// Delete godoc
// DELETE /api/v1/tenant/bots/:id
func (c *TelegramBotController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("bot controller delete request")

	clientID := middleware.GetTenantClientID(ctx)
	botUUID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("bot controller delete invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID bot tidak valid")
	}

	if err := c.botUC.Delete(ctx.Context(), clientID, botUUID); err != nil {
		log.Error("bot controller delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Bot berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /api/v1/tenant/bots/bulk
func (c *TelegramBotController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	clientID := middleware.GetTenantClientID(ctx)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("bot controller bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("bot controller bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.botUC.BulkDelete(ctx.Context(), clientID, req.IDs)
	log.Info("bot controller bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete bot selesai", result)
}
