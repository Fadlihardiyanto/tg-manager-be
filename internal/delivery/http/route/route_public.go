package route

import (
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
	webhooks.Post("/midtrans/billing", c.ClientBillingController.Webhook)
	webhooks.Post("/midtrans/member", c.MemberOrderController.Webhook)

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
}
