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

type CustomCommandController struct {
	commandUC usecase.ICustomCommandUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewCustomCommandController(uc usecase.ICustomCommandUseCase, log *zap.Logger, v *validator.Validate) *CustomCommandController {
	return &CustomCommandController{
		commandUC: uc,
		log:       log,
		validator: v,
	}
}

// Create godoc
// POST /api/v1/tenant/commands
func (c *CustomCommandController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("custom command controller create request")

	clientID := middleware.GetTenantClientID(ctx)

	var req model.CreateCustomCommandRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("custom command controller create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("custom command controller create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.commandUC.Create(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("custom command controller create failed", zap.Error(err))
		return err
	}

	log.Info("custom command controller create succeeded", zap.String("command_id", result.ID.String()))
	return helper.Created(ctx, "Custom command berhasil dibuat", result)
}

// List godoc
// GET /api/v1/tenant/commands
func (c *CustomCommandController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("custom command controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var req model.CustomCommandFilterRequest
	req.Page = page
	req.Limit = limit

	if botIDStr := ctx.Query("bot_id"); botIDStr != "" {
		if id, err := uuid.Parse(botIDStr); err == nil {
			req.BotID = &id
		}
	}
	if isActiveStr := ctx.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		req.IsActive = &isActive
	}

	result, total, err := c.commandUC.FindAllByClient(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("custom command controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar custom command", result, meta)
}

// Get godoc
// GET /api/v1/tenant/commands/:id
func (c *CustomCommandController) Get(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("custom command controller get request")

	clientID := middleware.GetTenantClientID(ctx)
	commandID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("custom command controller get invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID command tidak valid")
	}

	result, err := c.commandUC.FindByID(ctx.Context(), clientID, commandID)
	if err != nil {
		log.Error("custom command controller get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil detail custom command", result)
}

// Update godoc
// PUT /api/v1/tenant/commands/:id
func (c *CustomCommandController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("custom command controller update request")

	clientID := middleware.GetTenantClientID(ctx)
	commandID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("custom command controller update invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID command tidak valid")
	}

	var req model.UpdateCustomCommandRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("custom command controller update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("custom command controller update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.commandUC.Update(ctx.Context(), clientID, commandID, &req)
	if err != nil {
		log.Error("custom command controller update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Custom command berhasil diperbarui", result)
}

// Delete godoc
// DELETE /api/v1/tenant/commands/:id
func (c *CustomCommandController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("custom command controller delete request")

	clientID := middleware.GetTenantClientID(ctx)
	commandID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("custom command controller delete invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID command tidak valid")
	}

	if err := c.commandUC.Delete(ctx.Context(), clientID, commandID); err != nil {
		log.Error("custom command controller delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Custom command berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /api/v1/tenant/commands/bulk
func (c *CustomCommandController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	clientID := middleware.GetTenantClientID(ctx)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("custom command controller bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("custom command controller bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.commandUC.BulkDelete(ctx.Context(), clientID, req.IDs)
	log.Info("custom command controller bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete custom command selesai", result)
}
