package route

import (
	"strings"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/controller"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type PublicRouteConfig struct {
	App *fiber.App
	Log *zap.Logger

	TelegramWebhookController *controller.TelegramWebhookController
	// Midtrans webhook — public endpoint, dipanggil oleh server Midtrans (bukan admin)
	ClientBillingController *controller.ClientBillingController
	MemberOrderController   *controller.MemberOrderController
	PlatformPlanController  *controller.PlatformPlanController
}

func (c *PublicRouteConfig) Setup() {
	c.App.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	webhooks := c.App.Group("/webhooks")
	webhooks.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return uuid.New().String()
		},
	}))
	webhooks.Use(middleware.RequestContext())

	// Telegram Bot Webhooks
	telegram := webhooks.Group("/telegram")
	telegram.Post("/:bot_id", c.TelegramWebhookController.HandleIncomingUpdate)

	// Midtrans Payment Webhook — PUBLIC, divalidasi via SHA512 signature (bukan JWT)
	// Midtrans server akan POST ke endpoint ini setiap ada perubahan status pembayaran
	handleMidtrans := func(ctx fiber.Ctx) error {
		rawBody := ctx.Body()
		ctx.Locals("midtrans_raw_body", string(rawBody))

		var req struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(rawBody, &req); err != nil {
			c.Log.Warn("midtrans webhook: invalid request body", zap.Error(err))
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Format request tidak valid"})
		}

		if strings.HasPrefix(req.OrderID, "BILLING-") {
			return c.ClientBillingController.Webhook(ctx)
		} else {
			return c.MemberOrderController.Webhook(ctx)
		}
	}

	webhooks.Post("/midtrans", handleMidtrans)
	webhooks.Post("/midtrans/billing", handleMidtrans)
	webhooks.Post("/midtrans/member", handleMidtrans)

	// Public API routes
	api := c.App.Group("/api/v1")
	api.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return uuid.New().String()
		},
	}))
	api.Use(middleware.RequestContext())
	api.Post("/member/checkout", c.MemberOrderController.Checkout)

	public := api.Group("/public")
	public.Get("/plans", c.PlatformPlanController.ListPublic)
	public.Get("/checkout/:order_id", c.MemberOrderController.GetCheckoutDetail)
	public.Post("/checkout/:order_id/cancel", c.MemberOrderController.CancelCheckout)
}
