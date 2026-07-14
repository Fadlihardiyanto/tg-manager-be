package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type MemberOrderController struct {
	orderUC   usecase.IMemberOrderUseCase
	log       *zap.Logger
	validator *validator.Validate
}

func NewMemberOrderController(uc usecase.IMemberOrderUseCase, log *zap.Logger, v *validator.Validate) *MemberOrderController {
	return &MemberOrderController{
		orderUC:   uc,
		log:       log,
		validator: v,
	}
}

// Checkout godoc
// POST /api/v1/member/checkout
// Endpoint PUBLIC untuk member melakukan checkout paket.
func (c *MemberOrderController) Checkout(ctx fiber.Ctx) error {
	var req model.MemberCheckoutRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		c.log.Warn("member checkout: invalid request body", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		c.log.Warn("member checkout: validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.orderUC.Checkout(ctx.Context(), &req)
	if err != nil {
		return err
	}

	return helper.Created(ctx, "Checkout berhasil, silakan selesaikan pembayaran", result)
}

// Webhook godoc
// POST /webhooks/midtrans/member
// Endpoint PUBLIC untuk menerima status update pembayaran member dari Midtrans.
func (c *MemberOrderController) Webhook(ctx fiber.Ctx) error {
	var req model.MidtransWebhookRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		c.log.Warn("member webhook: invalid request body", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}
	if rawBody, ok := ctx.Locals("midtrans_raw_body").(string); ok {
		req.RawNotification = rawBody
	}

	if err := c.orderUC.HandleWebhook(ctx.Context(), &req); err != nil {
		c.log.Error("member webhook: handle failed", zap.Error(err), zap.String("order_id", req.OrderID))
		// Kembalikan 200 OK agar Midtrans tidak terus retry webhook jika data salah/error business logic
		return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
}

// GetCheckoutDetail godoc
// GET /api/v1/public/checkout/:order_id
func (c *MemberOrderController) GetCheckoutDetail(ctx fiber.Ctx) error {
	orderID := ctx.Params("order_id")
	if orderID == "" {
		return helper.BadRequest(ctx, "Order ID tidak boleh kosong")
	}

	result, err := c.orderUC.GetCheckoutDetail(ctx.Context(), orderID)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Detail checkout berhasil dimuat", result)
}

// CancelCheckout godoc
// POST /api/v1/public/checkout/:order_id/cancel
func (c *MemberOrderController) CancelCheckout(ctx fiber.Ctx) error {
	orderID := ctx.Params("order_id")
	if orderID == "" {
		return helper.BadRequest(ctx, "Order ID tidak boleh kosong")
	}

	if err := c.orderUC.CancelPendingOrder(ctx.Context(), orderID); err != nil {
		return err
	}

	return helper.Success(ctx, "Pesanan berhasil dibatalkan", nil)
}
