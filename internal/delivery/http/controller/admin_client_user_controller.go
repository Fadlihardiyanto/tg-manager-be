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

// AdminClientUserController handles CRUD for tenant users (users table + client_users relation).
type AdminClientUserController struct {
	uc        usecase.IAdminTenantUserUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewAdminClientUserController(uc usecase.IAdminTenantUserUseCase, log *zap.Logger, v *validator.Validate) *AdminClientUserController {
	return &AdminClientUserController{uc: uc, log: log, validator: v}
}

// List godoc
// GET /admin/v1/clients/:client_id/users
func (c *AdminClientUserController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin tenant user list request")

	clientID, err := uuid.Parse(ctx.Params("client_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	page, err := strconv.Atoi(ctx.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	sizeValue := ctx.Query("size", ctx.Query("limit", "20"))
	size, err := strconv.Atoi(sizeValue)
	if err != nil || size < 1 {
		size = 20
	}

	req := &model.AdminTenantUserListRequest{
		ClientID:          clientID,
		UserID:            ctx.Query("user_id"),
		Email:             ctx.Query("email"),
		Role:              ctx.Query("role"),
		Verified:          ctx.Query("verified"),
		Page:              page,
		Limit:             size,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin tenant user list validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	users, total, err := c.uc.ListTenantUsers(ctx.Context(), req)
	if err != nil {
		log.Error("admin tenant user list failed", zap.Error(err))
		return err
	}

	return helper.SuccessWithMeta(ctx, "Daftar user tenant berhasil diambil", users, helper.NewMeta(page, size, total))
}

// GetByID godoc
// GET /admin/v1/clients/:client_id/users/:user_id
func (c *AdminClientUserController) GetByID(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin tenant user get request")

	clientID, err := uuid.Parse(ctx.Params("client_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}
	userID, err := uuid.Parse(ctx.Params("user_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID user tidak valid")
	}

	req := &model.AdminTenantUserGetRequest{
		ClientID:          clientID,
		UserID:            userID,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	user, err := c.uc.GetTenantUser(ctx.Context(), req)
	if err != nil {
		log.Error("admin tenant user get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Detail user tenant berhasil diambil", user)
}

// Create godoc
// POST /admin/v1/clients/:client_id/users
func (c *AdminClientUserController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin tenant user create request")

	clientID, err := uuid.Parse(ctx.Params("client_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	var req model.AdminTenantUserCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin tenant user create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin tenant user create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.ClientID = clientID
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	user, err := c.uc.CreateTenantUser(ctx.Context(), &req)
	if err != nil {
		log.Error("admin tenant user create failed", zap.Error(err))
		return err
	}

	return helper.Created(ctx, "User tenant berhasil dibuat", user)
}

// Update godoc
// PUT /admin/v1/clients/:client_id/users/:user_id
func (c *AdminClientUserController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin tenant user update request")

	clientID, err := uuid.Parse(ctx.Params("client_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}
	userID, err := uuid.Parse(ctx.Params("user_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID user tidak valid")
	}

	var req model.AdminTenantUserUpdateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin tenant user update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin tenant user update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.ClientID = clientID
	req.UserID = userID
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	user, err := c.uc.UpdateTenantUser(ctx.Context(), &req)
	if err != nil {
		log.Error("admin tenant user update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "User tenant berhasil diperbarui", user)
}

// Delete godoc
// DELETE /admin/v1/clients/:client_id/users/:user_id
func (c *AdminClientUserController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin tenant user delete request")

	clientID, err := uuid.Parse(ctx.Params("client_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}
	userID, err := uuid.Parse(ctx.Params("user_id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID user tidak valid")
	}

	req := &model.AdminTenantUserDeleteRequest{
		ClientID:          clientID,
		UserID:            userID,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if err := c.uc.DeleteTenantUser(ctx.Context(), req); err != nil {
		log.Error("admin tenant user delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "User tenant berhasil dihapus", nil)
}
