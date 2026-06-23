package controller

import (
	"strconv"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PlatformPlanController struct {
	planUC    usecase.IPlatformPlanUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewPlatformPlanController(uc usecase.IPlatformPlanUseCase, log *zap.Logger, v *validator.Validate) *PlatformPlanController {
	return &PlatformPlanController{planUC: uc, log: log, validator: v}
}

// List godoc
// GET /admin/v1/billing/plans
func (c *PlatformPlanController) List(ctx fiber.Ctx) error {
	var req model.PlatformPlanFilterRequest

	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	req.Page = page
	req.Limit = limit

	if isActiveStr := ctx.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		req.IsActive = &isActive
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, total, err := c.planUC.List(ctx.Context(), &req, callerPermissions)
	if err != nil {
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Daftar plan berhasil diambil", result, meta)
}

// ListPublic godoc
// GET /api/v1/public/plans
func (c *PlatformPlanController) ListPublic(ctx fiber.Ctx) error {
	result, err := c.planUC.ListPublic(ctx.Context())
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Daftar plan publik berhasil diambil", result)
}

// GetByID godoc
// GET /admin/v1/billing/plans/:id
func (c *PlatformPlanController) GetByID(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID plan tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, err := c.planUC.GetByID(ctx.Context(), id, callerPermissions)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Detail plan berhasil diambil", result)
}

// Create godoc
// POST /admin/v1/billing/plans
func (c *PlatformPlanController) Create(ctx fiber.Ctx) error {
	var req model.CreatePlatformPlanRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, err := c.planUC.Create(ctx.Context(), &req, callerPermissions)
	if err != nil {
		return err
	}

	return helper.Created(ctx, "Plan berhasil dibuat", result)
}

// Update godoc
// PUT /admin/v1/billing/plans/:id
func (c *PlatformPlanController) Update(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID plan tidak valid")
	}

	var req model.UpdatePlatformPlanRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, err := c.planUC.Update(ctx.Context(), id, &req, callerPermissions)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Plan berhasil diperbarui", result)
}

// Delete godoc
// DELETE /admin/v1/billing/plans/:id
func (c *PlatformPlanController) Delete(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID plan tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	if err := c.planUC.Delete(ctx.Context(), id, callerPermissions); err != nil {
		return err
	}

	return helper.Success(ctx, "Plan berhasil dihapus", nil)
}
