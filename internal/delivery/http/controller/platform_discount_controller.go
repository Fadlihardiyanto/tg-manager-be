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

type PlatformDiscountController struct {
	discountUC usecase.IPlatformDiscountUseCase
	log        *zap.Logger
	validator  *validator.Validate
}

func NewPlatformDiscountController(uc usecase.IPlatformDiscountUseCase, log *zap.Logger, v *validator.Validate) *PlatformDiscountController {
	return &PlatformDiscountController{discountUC: uc, log: log, validator: v}
}

// List godoc
// GET /admin/v1/billing/discounts
func (c *PlatformDiscountController) List(ctx fiber.Ctx) error {
	onlyActive := true
	if onlyActiveStr := ctx.Query("only_active"); onlyActiveStr == "false" {
		onlyActive = false
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, err := c.discountUC.List(ctx.Context(), onlyActive, callerPermissions)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Daftar diskon berhasil diambil", result)
}

// GetByID godoc
// GET /admin/v1/billing/discounts/:id
func (c *PlatformDiscountController) GetByID(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID diskon tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, err := c.discountUC.GetByID(ctx.Context(), id, callerPermissions)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Detail diskon berhasil diambil", result)
}

// Create godoc
// POST /admin/v1/billing/discounts
func (c *PlatformDiscountController) Create(ctx fiber.Ctx) error {
	var req model.CreatePlatformDiscountRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.AdminID = middleware.GetAdminID(ctx)
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	result, err := c.discountUC.Create(ctx.Context(), &req)
	if err != nil {
		return err
	}

	return helper.Created(ctx, "Diskon berhasil dibuat", result)
}

// Update godoc
// PUT /admin/v1/billing/discounts/:id
func (c *PlatformDiscountController) Update(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID diskon tidak valid")
	}

	var req model.UpdatePlatformDiscountRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.DiscountID = id
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	result, err := c.discountUC.Update(ctx.Context(), &req)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Diskon berhasil diperbarui", result)
}

// Delete godoc
// DELETE /admin/v1/billing/discounts/:id
func (c *PlatformDiscountController) Delete(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID diskon tidak valid")
	}

	callerPermissions := middleware.GetAdminPermissions(ctx)

	if err := c.discountUC.Delete(ctx.Context(), id, callerPermissions); err != nil {
		return err
	}

	return helper.Success(ctx, "Diskon berhasil dihapus", nil)
}

// BulkDelete godoc
// DELETE /admin/v1/billing/discounts/bulk
func (c *PlatformDiscountController) BulkDelete(ctx fiber.Ctx) error {
	var req model.BulkDeleteRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	result := c.discountUC.BulkDelete(ctx.Context(), req.IDs, middleware.GetAdminPermissions(ctx))
	return helper.Success(ctx, "Bulk delete diskon selesai", result)
}
