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

type AdminImpersonationController struct {
	impersonationUC usecase.IAdminImpersonationUseCase
	log             *zap.Logger
	validator       *validator.Validate
}

func NewAdminImpersonationController(uc usecase.IAdminImpersonationUseCase, log *zap.Logger, v *validator.Validate) *AdminImpersonationController {
	return &AdminImpersonationController{impersonationUC: uc, log: log, validator: v}
}

// Start godoc
// POST /admin/v1/clients/:id/impersonate
func (c *AdminImpersonationController) Start(ctx fiber.Ctx) error {
	clientID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	var payload model.AdminImpersonateRequest
	if err := ctx.Bind().JSON(&payload); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, payload); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	payload.CallerPermissions = middleware.GetAdminPermissions(ctx)
	payload.IPAddress = ctx.IP()

	result, err := c.impersonationUC.Start(ctx.Context(), &model.AdminImpersonateClientActionRequest{
		AdminID:  middleware.GetAdminID(ctx),
		ClientID: clientID,
		Payload:  &payload,
	})
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Impersonation berhasil", result)
}

// End godoc
// POST /admin/v1/impersonation/end
func (c *AdminImpersonationController) End(ctx fiber.Ctx) error {
	var req struct {
		LogID string `json:"log_id" validate:"required,uuid"`
	}
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	logID, err := uuid.Parse(req.LogID)
	if err != nil {
		return helper.BadRequest(ctx, "log_id tidak valid")
	}

	adminID := middleware.GetAdminID(ctx)
	if err := c.impersonationUC.End(ctx.Context(), adminID, logID); err != nil {
		return err
	}

	return helper.Success(ctx, "Sesi impersonation diakhiri", nil)
}

// ListByClient godoc
// GET /admin/v1/clients/:id/impersonation-logs
func (c *AdminImpersonationController) ListByClient(ctx fiber.Ctx) error {
	clientID, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID client tidak valid")
	}

	page, limit := parsePagination(ctx)
	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, total, err := c.impersonationUC.ListByClient(ctx.Context(), clientID, page, limit, callerPermissions)
	if err != nil {
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Log impersonation berhasil diambil", result, meta)
}

// ListByAdmin godoc
// GET /admin/v1/impersonation-logs
func (c *AdminImpersonationController) ListByAdmin(ctx fiber.Ctx) error {
	adminID, err := uuid.Parse(ctx.Query("admin_id"))
	if err != nil {
		adminID = middleware.GetAdminID(ctx) // default: logged-in admin
	}

	page, limit := parsePagination(ctx)
	callerPermissions := middleware.GetAdminPermissions(ctx)

	result, total, err := c.impersonationUC.ListByAdmin(ctx.Context(), adminID, page, limit, callerPermissions)
	if err != nil {
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Log impersonation berhasil diambil", result, meta)
}

func parsePagination(ctx fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit
}
