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

type AdminClientController struct {
	clientUC  usecase.IAdminTenantUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewAdminClientController(uc usecase.IAdminTenantUseCase, log *zap.Logger, v *validator.Validate) *AdminClientController {
	return &AdminClientController{clientUC: uc, log: log, validator: v}
}

// List godoc
// GET /admin/v1/clients
func (c *AdminClientController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client list request")

	page, err := strconv.Atoi(ctx.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	sizeValue := ctx.Query("size", ctx.Query("limit", "20"))
	size, err := strconv.Atoi(sizeValue)
	if err != nil || size < 1 {
		size = 20
	}

	req := &model.AdminListAllClientsRequest{
		ClientID:          ctx.Query("client_id"),
		Name:              ctx.Query("name"),
		Slug:              ctx.Query("slug"),
		Active:            ctx.Query("active"),
		SubscriptionTier:  ctx.Query("subscription_tier"),
		Page:              page,
		Size:              size,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin client list validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	clients, total, err := c.clientUC.ListAllClients(ctx.Context(), req)
	if err != nil {
		log.Error("admin client list failed", zap.Error(err))
		return err
	}

	return helper.SuccessWithMeta(ctx, "Daftar tenant berhasil diambil", clients, helper.NewMeta(page, size, total))
}

// GetByID godoc
// GET /admin/v1/clients/:id
func (c *AdminClientController) GetByID(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client get request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	client, err := c.clientUC.GetClientDetail(ctx.Context(), &model.AdminGetClientDetailRequest{
		ClientID:          id,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	})
	if err != nil {
		log.Error("admin client get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Detail tenant berhasil diambil", client)
}

// Create godoc
// POST /admin/v1/clients
func (c *AdminClientController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client create request")

	var req model.AdminCreateClientRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin client create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin client create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	client, err := c.clientUC.CreateClient(ctx.Context(), &req)
	if err != nil {
		log.Error("admin client create failed", zap.Error(err))
		return err
	}

	return helper.Created(ctx, "Tenant berhasil dibuat", client)
}

// Update godoc
// PUT /admin/v1/clients/:id
func (c *AdminClientController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client update request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	var req model.AdminUpdateClientRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin client update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin client update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.ClientID = id
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	client, err := c.clientUC.UpdateClient(ctx.Context(), &req)
	if err != nil {
		log.Error("admin client update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Tenant berhasil diperbarui", client)
}

// Delete godoc
// DELETE /admin/v1/clients/:id
func (c *AdminClientController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client delete request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	if err := c.clientUC.DeleteClient(ctx.Context(), &model.AdminDeleteClientRequest{
		ClientID:          id,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}); err != nil {
		log.Error("admin client delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Tenant berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /admin/v1/clients/bulk
func (c *AdminClientController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin client bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin client bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.clientUC.BulkDeleteClients(ctx.Context(), req.IDs, middleware.GetAdminPermissions(ctx))
	log.Info("admin client bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete tenant selesai", result)
}

func (c *AdminClientController) Activate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client activate request")
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}
	if err := c.clientUC.ActivateClient(ctx.Context(), &model.AdminActivateClientRequest{
		ClientID:          id,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}); err != nil {
		log.Error("admin client activate failed", zap.Error(err))
		return err
	}
	return helper.Success(ctx, "Tenant berhasil diaktifkan", nil)
}

func (c *AdminClientController) Deactivate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin client deactivate request")
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}
	if err := c.clientUC.DeactivateClient(ctx.Context(), &model.AdminDeactivateClientRequest{
		ClientID:          id,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}); err != nil {
		log.Error("admin client deactivate failed", zap.Error(err))
		return err
	}
	return helper.Success(ctx, "Tenant berhasil dinonaktifkan", nil)
}
