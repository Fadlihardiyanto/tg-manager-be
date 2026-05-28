package controller

import (
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

	// Default pagination
	req.Page = 1
	req.Limit = 20

	if isActiveStr := ctx.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		req.IsActive = &isActive
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, err := c.planUC.List(ctx.Context(), &req, callerPermissions)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Daftar plan berhasil diambil", result)
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
