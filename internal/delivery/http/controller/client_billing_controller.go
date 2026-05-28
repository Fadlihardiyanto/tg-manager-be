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

type ClientBillingController struct {
	billingUC usecase.IClientBillingUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewClientBillingController(uc usecase.IClientBillingUseCase, log *zap.Logger, v *validator.Validate) *ClientBillingController {
	return &ClientBillingController{billingUC: uc, log: log, validator: v}
}

// AdminList godoc
// GET /admin/v1/billing
func (c *ClientBillingController) AdminList(ctx fiber.Ctx) error {
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	req := &model.AdminListBillingRequest{
		Status:            ctx.Query("status"),
		Page:              page,
		Limit:             limit,
		CallerPermissions: middleware.GetAdminPermissions(ctx),
	}

	if clientIDStr := ctx.Query("client_id"); clientIDStr != "" {
		id, err := uuid.Parse(clientIDStr)
		if err != nil {
			return helper.BadRequest(ctx, "client_id tidak valid")
		}
		req.ClientID = &id
	}

	billings, total, err := c.billingUC.ListBillings(ctx.Context(), req)
	if err != nil {
		return err
	}

	return helper.SuccessWithMeta(ctx, "Daftar billing berhasil diambil", billings,
		helper.NewMeta(page, limit, total))
}

// AdminAssignPlan godoc
// POST /admin/v1/billing/assign
func (c *ClientBillingController) AdminAssignPlan(ctx fiber.Ctx) error {
	var req model.AdminAssignPlanRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.AdminID = middleware.GetAdminID(ctx)
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)

	result, err := c.billingUC.AdminAssignPlan(ctx.Context(), &req)
	if err != nil {
		return err
	}

	return helper.Created(ctx, "Plan berhasil di-assign ke client", result)
}

// CancelBilling godoc
// DELETE /admin/v1/billing/:id
func (c *ClientBillingController) CancelBilling(ctx fiber.Ctx) error {
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return helper.BadRequest(ctx, "ID billing tidak valid")
	}

	var req model.CancelBillingRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		req = model.CancelBillingRequest{}
	}

	req.BillingID = id
	req.CallerPermissions = middleware.GetAdminPermissions(ctx)
	req.AdminID = middleware.GetAdminID(ctx)

	if err := c.billingUC.CancelBilling(ctx.Context(), &req); err != nil {
		return err
	}

	return helper.Success(ctx, "Billing berhasil dibatalkan", nil)
}

// Webhook godoc
// POST /webhooks/midtrans/billing
// Endpoint ini PUBLIC — tidak butuh auth, tapi divalidasi via signature
func (c *ClientBillingController) Webhook(ctx fiber.Ctx) error {
	var req model.MidtransWebhookRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		c.log.Warn("billing webhook: invalid body", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if err := c.billingUC.HandleWebhook(ctx.Context(), &req); err != nil {
		c.log.Error("billing webhook: handle failed", zap.Error(err),
			zap.String("order_id", req.OrderID))
		// Selalu return 200 ke Midtrans meski gagal — Midtrans akan retry
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// ClientCheckout godoc
// POST /api/v1/billing/checkout
// Dipanggil oleh tenant user yang sudah login
func (c *ClientBillingController) ClientCheckout(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	var req model.ClientCheckoutPlanRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("client checkout: invalid body", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	log.Info("client checkout request", zap.Any("request", req))
	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	// ClientID dari JWT claims tenant
	req.ClientID = middleware.GetTenantClientID(ctx)

	log.Info("client checkout validated", zap.Any("request", req))
	result, err := c.billingUC.Checkout(ctx.Context(), &req)
	if err != nil {
		log.Error("client checkout failed", zap.Error(err), zap.Any("request", req))
		return err
	}

	log.Info("client checkout succeeded", zap.Any("response", result))
	return helper.Created(ctx, "Checkout berhasil, silakan selesaikan pembayaran", result)
}

// ClientGetActiveBilling godoc
// GET /api/v1/billing/active
// Dipanggil oleh tenant user yang sudah login untuk melihat billing aktifnya
func (c *ClientBillingController) ClientGetActiveBilling(ctx fiber.Ctx) error {
	clientID := middleware.GetTenantClientID(ctx)

	result, err := c.billingUC.GetActiveBilling(ctx.Context(), clientID)
	if err != nil {
		return err
	}

	if result == nil {
		return helper.Success(ctx, "Tidak ada billing aktif", nil)
	}

	return helper.Success(ctx, "Billing aktif berhasil diambil", result)
}
