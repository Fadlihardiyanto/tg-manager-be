package controller

import (
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

type AdminRoleController struct {
	adminRoleUC usecase.IAdminRoleUseCase
	log         *zap.Logger
	validator   *validator.Validate
}

func NewAdminRoleController(uc usecase.IAdminRoleUseCase, log *zap.Logger, v *validator.Validate) *AdminRoleController {
	return &AdminRoleController{
		adminRoleUC: uc,
		log:         log,
		validator:   v,
	}
}

// List godoc
// GET /admin/v1/roles
func (c *AdminRoleController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin role list request")

	callerPermissions := middleware.GetAdminPermissions(ctx)
	callerRoles := middleware.GetAdminRoles(ctx)

	ucReq := &model.AdminRoleListRequest{
		CallerPermissions: callerPermissions,
		CallerRoles:       callerRoles,
	}

	result, err := c.adminRoleUC.ListRoles(ctx.Context(), ucReq)
	if err != nil {
		log.Error("admin role list failed", zap.Error(err))
		return err
	}
	log.Info("admin role list succeeded")

	return helper.Success(ctx, "Daftar role berhasil diambil", result)
}

// GetByID godoc
// GET /admin/v1/roles/:id
func (c *AdminRoleController) GetByID(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin role get by id request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin role get by id invalid id")
		return helper.BadRequest(ctx, "ID role tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	callerRoles := middleware.GetAdminRoles(ctx)
	ucReq := &model.AdminRoleGetRequest{
		RoleID:            id,
		CallerPermissions: callerPermissions,
		CallerRoles:       callerRoles,
	}

	result, err := c.adminRoleUC.GetRole(ctx.Context(), ucReq)
	if err != nil {
		log.Error("admin role get by id failed", zap.Error(err))
		return err
	}
	log.Info("admin role get by id succeeded")

	return helper.Success(ctx, "Detail role berhasil diambil", result)
}

// Create godoc
// POST /admin/v1/roles
func (c *AdminRoleController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin role create request")

	var req model.AdminRoleCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin role create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin role create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	callerRoles := middleware.GetAdminRoles(ctx)
	req.CallerPermissions = callerPermissions
	req.CallerRoles = callerRoles

	result, err := c.adminRoleUC.CreateRole(ctx.Context(), &req)
	if err != nil {
		log.Error("admin role create failed", zap.Error(err))
		return err
	}
	log.Info("admin role create succeeded")

	return helper.Created(ctx, "Role berhasil dibuat", result)
}

// Update godoc
// PUT /admin/v1/roles/:id
func (c *AdminRoleController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin role update request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin role update invalid id")
		return helper.BadRequest(ctx, "ID role tidak valid")
	}

	var req model.AdminRoleUpdateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin role update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin role update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	callerRoles := middleware.GetAdminRoles(ctx)
	req.RoleID = id
	req.CallerPermissions = callerPermissions
	req.CallerRoles = callerRoles

	result, err := c.adminRoleUC.UpdateRole(ctx.Context(), &req)
	if err != nil {
		log.Error("admin role update failed", zap.Error(err))
		return err
	}
	log.Info("admin role update succeeded")

	return helper.Success(ctx, "Role berhasil diperbarui", result)
}

// Delete godoc
// DELETE /admin/v1/roles/:id
func (c *AdminRoleController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin role delete request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin role delete invalid id")
		return helper.BadRequest(ctx, "ID role tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	callerRoles := middleware.GetAdminRoles(ctx)
	ucReq := &model.AdminRoleDeleteRequest{
		RoleID:            id,
		CallerPermissions: callerPermissions,
		CallerRoles:       callerRoles,
	}

	if err := c.adminRoleUC.DeleteRole(ctx.Context(), ucReq); err != nil {
		log.Error("admin role delete failed", zap.Error(err))
		return err
	}
	log.Info("admin role delete succeeded")

	return helper.Success(ctx, "Role berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /admin/v1/roles/bulk
func (c *AdminRoleController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin role bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin role bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.adminRoleUC.BulkDeleteRoles(ctx.Context(), req.IDs, middleware.GetAdminPermissions(ctx))
	log.Info("admin role bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete role selesai", result)
}

// SyncPermissions godoc
// PUT /admin/v1/roles/:id/permissions
func (c *AdminRoleController) SyncPermissions(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin role sync permissions request")

	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin role sync permissions invalid id")
		return helper.BadRequest(ctx, "ID role tidak valid")
	}

	var req model.AdminSyncPermissionsRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin role sync permissions bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin role sync permissions validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	callerRoles := middleware.GetAdminRoles(ctx)
	req.RoleID = id
	req.CallerPermissions = callerPermissions
	req.CallerRoles = callerRoles

	if err := c.adminRoleUC.SyncRolePermissions(ctx.Context(), &req); err != nil {
		log.Error("admin role sync permissions failed", zap.Error(err))
		return err
	}
	log.Info("admin role sync permissions succeeded")

	return helper.Success(ctx, "Permissions role berhasil disinkronkan", nil)
}
