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

// AdminUserController handles admin user management endpoints.
type AdminUserController struct {
	adminRoleUC usecase.IAdminRoleUseCase
	adminUserUC usecase.IAdminUserManagementUseCase
	log         *zap.Logger
	validator   *validator.Validate
}

func NewAdminUserController(roleUC usecase.IAdminRoleUseCase, userUC usecase.IAdminUserManagementUseCase, log *zap.Logger, v *validator.Validate) *AdminUserController {
	return &AdminUserController{
		adminRoleUC: roleUC,
		adminUserUC: userUC,
		log:         log,
		validator:   v,
	}
}

// List godoc
// GET /admin/v1/admins
func (c *AdminUserController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user list request")

	page, err := strconv.Atoi(ctx.Query("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err := strconv.Atoi(ctx.Query("limit", "20"))
	if err != nil || limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	req := &model.AdminUserListRequest{
		Offset:            offset,
		Limit:             limit,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	admins, total, err := c.adminUserUC.ListAdmins(ctx.Context(), req)
	if err != nil {
		log.Error("admin user list failed", zap.Error(err))
		return err
	}

	return helper.SuccessWithMeta(ctx, "Daftar admin berhasil diambil", admins, helper.NewMeta(page, limit, total))
}

// GetByID godoc
// GET /admin/v1/admins/:id
func (c *AdminUserController) GetByID(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user get request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user get invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	req := &model.AdminUserGetRequest{
		AdminID:           adminID,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	admin, err := c.adminUserUC.GetAdmin(ctx.Context(), req)
	if err != nil {
		log.Error("admin user get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Detail admin berhasil diambil", admin)
}

// Create godoc
// POST /admin/v1/admins
func (c *AdminUserController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user create request")

	var req model.AdminUserCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin user create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin user create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	admin, err := c.adminUserUC.CreateAdmin(ctx.Context(), &req)
	if err != nil {
		log.Error("admin user create failed", zap.Error(err))
		return err
	}

	return helper.Created(ctx, "Admin berhasil dibuat", admin)
}

// Update godoc
// PUT /admin/v1/admins/:id
func (c *AdminUserController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user update request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user update invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	var req model.AdminUserUpdateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin user update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin user update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.AdminID = adminID
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	admin, err := c.adminUserUC.UpdateAdmin(ctx.Context(), &req)
	if err != nil {
		log.Error("admin user update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Admin berhasil diperbarui", admin)
}

// Delete godoc
// DELETE /admin/v1/admins/:id
func (c *AdminUserController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user delete request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user delete invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	req := &model.AdminUserDeleteRequest{
		AdminID:           adminID,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if err := c.adminUserUC.DeleteAdmin(ctx.Context(), req); err != nil {
		log.Error("admin user delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Admin berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /admin/v1/admins/bulk
func (c *AdminUserController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin user bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin user bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.adminUserUC.BulkDeleteAdmins(ctx.Context(), req.IDs, middleware.GetAdminPermissions(ctx))
	log.Info("admin user bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete admin selesai", result)
}

// Activate godoc
// PATCH /admin/v1/admins/:id/activate
func (c *AdminUserController) Activate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user activate request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user activate invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	req := &model.AdminUserActivateRequest{
		AdminID:           adminID,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if err := c.adminUserUC.ActivateAdmin(ctx.Context(), req); err != nil {
		log.Error("admin user activate failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Admin berhasil diaktifkan", nil)
}

// Deactivate godoc
// PATCH /admin/v1/admins/:id/deactivate
func (c *AdminUserController) Deactivate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user deactivate request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user deactivate invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	req := &model.AdminUserDeactivateRequest{
		AdminID:           adminID,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if err := c.adminUserUC.DeactivateAdmin(ctx.Context(), req); err != nil {
		log.Error("admin user deactivate failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Admin berhasil dinonaktifkan", nil)
}

// SyncRoles godoc
// PUT /admin/v1/admins/:id/roles
func (c *AdminUserController) SyncRoles(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user sync roles request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user sync roles invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	var req model.AdminAssignRolesRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin user sync roles bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin user sync roles validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerRoles := middleware.GetAdminRoles(ctx)
	req.AdminID = adminID
	req.CallerRoles = callerRoles

	if err := c.adminRoleUC.AssignRolesToAdmin(ctx.Context(), &req); err != nil {
		log.Error("admin user sync roles failed", zap.Error(err))
		return err
	}
	log.Info("admin user sync roles succeeded")

	return helper.Success(ctx, "Roles admin berhasil diperbarui", nil)
}

// AssignRole godoc
// POST /admin/v1/admins/:id/roles
func (c *AdminUserController) AssignRole(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user assign role request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user assign role invalid id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	var req model.AdminAssignRoleRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin user assign role bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin user assign role validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	req.AdminID = adminID
	req.CallerPermissions = callerPermissions

	if err := c.adminRoleUC.AssignRoleToAdmin(ctx.Context(), &req); err != nil {
		log.Error("admin user assign role failed", zap.Error(err))
		return err
	}
	log.Info("admin user assign role succeeded")

	return helper.Success(ctx, "Role berhasil ditambahkan ke admin", nil)
}

// RevokeRole godoc
// DELETE /admin/v1/admins/:id/roles/:role_id
func (c *AdminUserController) RevokeRole(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin user revoke role request")

	adminID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("admin user revoke role invalid admin id")
		return helper.BadRequest(ctx, "ID admin tidak valid")
	}

	roleID, err := uuid.Parse(ctx.Params("role_id"))
	if err != nil {
		log.Warn("admin user revoke role invalid role id")
		return helper.BadRequest(ctx, "ID role tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)
	req := &model.AdminRevokeRoleRequest{
		AdminID:           adminID,
		RoleID:            roleID,
		CallerPermissions: callerPermissions,
	}

	if err := c.adminRoleUC.RevokeRoleFromAdmin(ctx.Context(), req); err != nil {
		log.Error("admin user revoke role failed", zap.Error(err))
		return err
	}
	log.Info("admin user revoke role succeeded")

	return helper.Success(ctx, "Role berhasil dihapus dari admin", nil)
}
