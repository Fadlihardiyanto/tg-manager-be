package route

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/controller"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TenantRouteConfig struct {
	App *fiber.App
	Log *zap.Logger

	// Auth
	TenantAuthController *controller.TenantAuthController

	// Bot Management
	TelegramBotController *controller.TelegramBotController

	// Group Management
	TelegramGroupController *controller.TelegramGroupController

	// Packages and Discounts
	PackageController        *controller.PackageController
	MemberDiscountController *controller.MemberDiscountController

	// Billing (Self-Service)
	ClientBillingController *controller.ClientBillingController

	// Analytics & Audit
	TenantAnalyticsController *controller.TenantAnalyticsController
	AuditLogController        *controller.AuditLogController

	// Middleware
	TenantAuthMiddleware fiber.Handler // JWT validation for tenant
}

func (c *TenantRouteConfig) Setup() {
	api := c.App.Group("/api/v1")
	api.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return uuid.New().String()
		},
	}))
	api.Use(middleware.RequestContext())

	c.setupPublicRoutes(api)
	c.setupProtectedRoutes(api)
}

func (c *TenantRouteConfig) setupPublicRoutes(api fiber.Router) {
	auth := api.Group("/auth")
	auth.Post("/register", c.TenantAuthController.Register)
	auth.Post("/login", c.TenantAuthController.Login)
	auth.Get("/verify-email", c.TenantAuthController.VerifyEmail)
}

func (c *TenantRouteConfig) setupProtectedRoutes(api fiber.Router) {
	// All protected tenant routes go through TenantAuth middleware
	protected := api.Group("/tenant", c.TenantAuthMiddleware)

	// ── Me (current user profile) ────────────────────────────────────
	// TODO: protected.Get("/me", c.TenantProfileController.Me)

	// ── Bots ─────────────────────────────────────────────────────────
	bots := protected.Group("/bots")
	bots.Get("/", middleware.TenantRequirePermission("bots.read"), c.TelegramBotController.List)
	bots.Get("/:id", middleware.TenantRequirePermission("bots.read"), c.TelegramBotController.Get)
	bots.Post("/", middleware.TenantRequirePermission("bots.create"), c.TelegramBotController.Create)
	bots.Put("/:id", middleware.TenantRequirePermission("bots.update"), c.TelegramBotController.Update)
	bots.Delete("/:id", middleware.TenantRequirePermission("bots.delete"), c.TelegramBotController.Delete)

	// ── Groups ───────────────────────────────────────────────────────
	groups := protected.Group("/groups")
	groups.Get("/", middleware.TenantRequirePermission("groups.read"), c.TelegramGroupController.List)
	groups.Get("/:id", middleware.TenantRequirePermission("groups.read"), c.TelegramGroupController.Get)
	groups.Post("/", middleware.TenantRequirePermission("groups.create"), c.TelegramGroupController.Create)
	groups.Put("/:id", middleware.TenantRequirePermission("groups.update"), c.TelegramGroupController.Update)
	groups.Delete("/:id", middleware.TenantRequirePermission("groups.delete"), c.TelegramGroupController.Delete)

	// ── Packages ─────────────────────────────────────────────────────
	packages := protected.Group("/packages")
	packages.Get("/", middleware.TenantRequirePermission("packages.read"), c.PackageController.List)
	packages.Get("/:id", middleware.TenantRequirePermission("packages.read"), c.PackageController.Get)
	packages.Post("/", middleware.TenantRequirePermission("packages.create"), c.PackageController.Create)
	packages.Put("/:id", middleware.TenantRequirePermission("packages.update"), c.PackageController.Update)
	packages.Delete("/:id", middleware.TenantRequirePermission("packages.delete"), c.PackageController.Delete)
	packages.Post("/:id/groups", middleware.TenantRequirePermission("packages.update"), c.PackageController.AssociateGroups)

	// ── Discounts ────────────────────────────────────────────────────
	discounts := protected.Group("/discounts")
	discounts.Get("/", middleware.TenantRequirePermission("packages.read"), c.MemberDiscountController.List)
	discounts.Post("/", middleware.TenantRequirePermission("packages.create"), c.MemberDiscountController.Create)
	discounts.Put("/:id", middleware.TenantRequirePermission("packages.update"), c.MemberDiscountController.Update)
	discounts.Delete("/:id", middleware.TenantRequirePermission("packages.delete"), c.MemberDiscountController.Delete)

	// ── Analytics & Audit ────────────────────────────────────────────────────
	analytics := protected.Group("/analytics")
	analytics.Get("/overview", c.TenantAnalyticsController.GetOverview)

	audit := protected.Group("/audit-logs")
	audit.Get("/", c.AuditLogController.ListTenantLogs)

	// ── Billing (Self-Service) ────────────────────────────────────────────
	billing := protected.Group("/billing")
	billing.Get("/active", c.ClientBillingController.ClientGetActiveBilling)
	billing.Post("/checkout", c.ClientBillingController.ClientCheckout)

	// ── Team ─────────────────────────────────────────────────────────
	// TODO: Wire team management routes here
	// team := protected.Group("/team")
	// team.Get("/", middleware.TenantRequirePermission("team.read"), c.TeamController.List)
	// team.Post("/invite", middleware.TenantRequireRole("owner", "admin"), c.TeamController.Invite)
}
