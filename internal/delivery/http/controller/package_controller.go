package controller

import (
	"strconv"
	"strings"

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

type PackageController struct {
	packageUC usecase.IPackageUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewPackageController(uc usecase.IPackageUseCase, log *zap.Logger, v *validator.Validate) *PackageController {
	return &PackageController{
		packageUC: uc,
		log:       log,
		validator: v,
	}
}

// Create godoc
// POST /api/v1/tenant/packages
func (c *PackageController) Create(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller create request")

	clientID := middleware.GetTenantClientID(ctx)

	var req model.PackageCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("package controller create bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("package controller create validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.packageUC.Create(ctx.Context(), clientID, &req)
	if err != nil {
		log.Error("package controller create failed", zap.Error(err))
		return err
	}
	if result == nil {
		log.Error("package controller create returned nil result")
		return helper.InternalError(ctx, "Terjadi kesalahan pada server")
	}

	log.Info("package controller create succeeded", zap.String("package_id", result.ID.String()))
	return helper.Created(ctx, "Paket berhasil dibuat", result)
}

// List godoc
// GET /api/v1/tenant/packages
func (c *PackageController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	page = clampPage(page)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filter := model.PackageFilterRequest{
		Page:        page,
		Limit:       limit,
		Search:      ctx.Query("search", ""),
		IsAllAccess: parseBoolSlice(ctx.Query("is_all_access", "")),
		IsActive:    parseBoolSlice(ctx.Query("is_active", "")),
	}

	result, total, err := c.packageUC.FindAllByClient(ctx.Context(), clientID, filter)
	if err != nil {
		log.Error("package controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar paket", result, meta)
}

// Get godoc
// GET /api/v1/tenant/packages/:id
func (c *PackageController) Get(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller get request")

	clientID := middleware.GetTenantClientID(ctx)
	packageID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("package controller get invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID paket tidak valid")
	}

	result, err := c.packageUC.FindByID(ctx.Context(), clientID, packageID)
	if err != nil {
		log.Error("package controller get failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Berhasil mengambil detail paket", result)
}

// Update godoc
// PUT /api/v1/tenant/packages/:id
func (c *PackageController) Update(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller update request")

	clientID := middleware.GetTenantClientID(ctx)
	packageID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("package controller update invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID paket tidak valid")
	}

	var req model.PackageUpdateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("package controller update bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("package controller update validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.packageUC.Update(ctx.Context(), clientID, packageID, &req)
	if err != nil {
		log.Error("package controller update failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Paket berhasil diperbarui", result)
}

// Delete godoc
// DELETE /api/v1/tenant/packages/:id
func (c *PackageController) Delete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller delete request")

	clientID := middleware.GetTenantClientID(ctx)
	packageID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("package controller delete invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID paket tidak valid")
	}

	if err := c.packageUC.Delete(ctx.Context(), clientID, packageID); err != nil {
		log.Error("package controller delete failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Paket berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /api/v1/tenant/packages/bulk
func (c *PackageController) BulkDelete(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	clientID := middleware.GetTenantClientID(ctx)

	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("package controller bulk delete bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("package controller bulk delete validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.packageUC.BulkDelete(ctx.Context(), clientID, req.IDs)
	log.Info("package controller bulk delete finished", zap.Int("deleted", result.Deleted), zap.Int("failed", len(result.Failed)))
	return helper.Success(ctx, "Bulk delete paket selesai", result)
}

// AssociateGroups godoc
// POST /api/v1/tenant/packages/:id/groups
func (c *PackageController) AssociateGroups(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller associate request")

	clientID := middleware.GetTenantClientID(ctx)
	packageID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("package controller associate invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID paket tidak valid")
	}

	var req model.PackageGroupAssociateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("package controller associate bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("package controller associate validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	if err := c.packageUC.AssociateGroups(ctx.Context(), clientID, packageID, &req); err != nil {
		log.Error("package controller associate failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Grup berhasil dikaitkan dengan paket", nil)
}

func parseBoolSlice(val string) []bool {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var result []bool
	for _, p := range parts {
		b, err := strconv.ParseBool(strings.TrimSpace(p))
		if err == nil {
			result = append(result, b)
		}
	}
	return result
}

// Activate godoc
// PATCH /api/v1/tenant/packages/:id/activate
func (c *PackageController) Activate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller activate request")

	clientID := middleware.GetTenantClientID(ctx)
	packageID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("package controller activate invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID paket tidak valid")
	}

	isActive := true
	result, err := c.packageUC.Update(ctx.Context(), clientID, packageID, &model.PackageUpdateRequest{
		IsActive: &isActive,
	})
	if err != nil {
		log.Error("package controller activate failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Paket berhasil diaktifkan", result)
}

// Deactivate godoc
// PATCH /api/v1/tenant/packages/:id/deactivate
func (c *PackageController) Deactivate(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("package controller deactivate request")

	clientID := middleware.GetTenantClientID(ctx)
	packageID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		log.Warn("package controller deactivate invalid id", zap.Error(err))
		return helper.BadRequest(ctx, "ID paket tidak valid")
	}

	isActive := false
	result, err := c.packageUC.Update(ctx.Context(), clientID, packageID, &model.PackageUpdateRequest{
		IsActive: &isActive,
	})
	if err != nil {
		log.Error("package controller deactivate failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Paket berhasil dinonaktifkan", result)
}
