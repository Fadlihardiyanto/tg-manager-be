package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AdminPermissionController struct {
	permissionUC usecase.IAdminPermissionUseCase
	log          *zap.Logger
}

func NewAdminPermissionController(
	permissionUC usecase.IAdminPermissionUseCase,
	log *zap.Logger,
) *AdminPermissionController {
	return &AdminPermissionController{
		permissionUC: permissionUC,
		log:          log,
	}
}

// List godoc
// GET /admin/v1/permissions
func (c *AdminPermissionController) List(ctx fiber.Ctx) error {
	var req model.AdminPermissionListRequest
	permissions, err := c.permissionUC.ListPermissions(ctx.Context(), &req)
	if err != nil {
		c.log.Error("admin permission controller: list", zap.Error(err))
		return helper.InternalError(ctx, "Gagal mengambil daftar permission")
	}

	return helper.Success(ctx, "Daftar permission berhasil diambil", permissions)
}

// GetByID godoc
// GET /admin/v1/permissions/:id
func (c *AdminPermissionController) GetByID(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID permission tidak valid")
	}

	req := &model.AdminPermissionGetRequest{PermissionID: id}
	permission, err := c.permissionUC.GetPermission(ctx.Context(), req)
	if err != nil {
		c.log.Error("admin permission controller: get by id", zap.Error(err))
		return helper.NotFound(ctx, "Permission tidak ditemukan")
	}

	return helper.Success(ctx, "Detail permission berhasil diambil", permission)
}
