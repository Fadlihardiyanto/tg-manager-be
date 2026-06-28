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

type TelegramGroupController struct {
	groupUC   usecase.ITelegramGroupUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewTelegramGroupController(uc usecase.ITelegramGroupUseCase, log *zap.Logger, v *validator.Validate) *TelegramGroupController {
	return &TelegramGroupController{
		groupUC:   uc,
		log:       log,
		validator: v,
	}
}

// Create godoc
// POST /api/v1/tenant/groups
func (c *TelegramGroupController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller create request")

	clientID := middleware.GetTenantClientID(ctx)

	var req model.GroupCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("group controller create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("group controller create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.groupUC.Create(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("group controller create failed", zap.Error(err))
		return err
	}

	log.Info("group controller create succeeded", zap.String("group_id", result.ID.String()))
	return helper.Created(ctx, "Grup berhasil didaftarkan", result)
}

// List godoc
// GET /api/v1/tenant/groups
func (c *TelegramGroupController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	result, total, err := c.groupUC.FindAllByClient(ctx.Context(), clientID, page, limit)
	if err != nil {
		log.Error("group controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar grup", result, meta)
}

// Get godoc
// GET /api/v1/tenant/groups/:id
func (c *TelegramGroupController) Get(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller get request")

	clientID := middleware.GetTenantClientID(ctx)
	groupID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("group controller get invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID grup tidak valid")
	}

	result, err := c.groupUC.FindByID(ctx.Context(), clientID, groupID)
	if err != nil {
		log.Error("group controller get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil detail grup", result)
}

// Update godoc
// PUT /api/v1/tenant/groups/:id
func (c *TelegramGroupController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller update request")

	clientID := middleware.GetTenantClientID(ctx)
	groupID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("group controller update invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID grup tidak valid")
	}

	var req model.GroupUpdateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("group controller update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("group controller update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.groupUC.Update(ctx.Context(), clientID, groupID, &req)
	if err != nil {
		log.Error("group controller update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Grup berhasil diperbarui", result)
}

// Delete godoc
// DELETE /api/v1/tenant/groups/:id
func (c *TelegramGroupController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller delete request")

	clientID := middleware.GetTenantClientID(ctx)
	groupID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("group controller delete invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID grup tidak valid")
	}

	if err := c.groupUC.Delete(ctx.Context(), clientID, groupID); err != nil {
		log.Error("group controller delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Grup berhasil dihapus", nil)
}

// GenerateConnectToken godoc
// POST /api/v1/tenant/bots/:bot_id/groups/connect-token
func (c *TelegramGroupController) GenerateConnectToken(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller generate connect token request")

	clientID := middleware.GetTenantClientID(ctx)
	botID, err := uuid.Parse(ctx.Params("bot_id"))
	if err != nil {
		log.Warn("group controller generate token invalid bot id", zap.Error(err))
		return helper.BadRequest(ctx, "ID bot tidak valid")
	}

	token, botUsername, err := c.groupUC.GenerateConnectToken(ctx.Context(), clientID, botID)
	if err != nil {
		log.Error("group controller generate token failed", zap.Error(err))
		return err
	}

	resp := model.GroupConnectTokenResponse{
		Token:       token,
		ExpiresIn:   900, // 15 minutes
		BotUsername: botUsername,
	}

	return helper.Success(ctx, "Token koneksi berhasil dibuat", resp)
}

// CheckConnectStatus godoc
// GET /api/v1/tenant/bots/:bot_id/groups/connect-status/:token
func (c *TelegramGroupController) CheckConnectStatus(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("group controller check connect status request")

	clientID := middleware.GetTenantClientID(ctx)
	botID, err := uuid.Parse(ctx.Params("bot_id"))
	if err != nil {
		log.Warn("group controller check status invalid bot id", zap.Error(err))
		return helper.BadRequest(ctx, "ID bot tidak valid")
	}

	token := ctx.Params("token")
	if token == "" {
		return helper.BadRequest(ctx, "Token tidak valid")
	}

	status, err := c.groupUC.CheckConnectStatus(ctx.Context(), clientID, botID, token)
	if err != nil {
		log.Error("group controller check status failed", zap.Error(err))
		return err
	}

	resp := map[string]interface{}{
		"status": status,
	}

	return helper.Success(ctx, "Status koneksi berhasil diperiksa", resp)
}
