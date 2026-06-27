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
	TelegramBotController   *controller.TelegramBotController
	CustomCommandController *controller.CustomCommandController

	// Group Management
	TelegramGroupController *controller.TelegramGroupController

	// Packages and Discounts
	PackageController        *controller.PackageController
	MemberDiscountController *controller.MemberDiscountController

	// Members
	MemberController *controller.MemberController

	// Transactions
	TenantTransactionController *controller.TenantTransactionController

	// Billing (Self-Service)
	ClientBillingController *controller.ClientBillingController

	// Analytics & Audit
	TenantAnalyticsController *controller.TenantAnalyticsController
	AuditLogController        *controller.AuditLogController

	// Profile & Settings
	TenantProfileController *controller.TenantProfileController

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
	c.setupOnboardingRoutes(api)
	c.setupProtectedRoutes(api)
}

func (c *TenantRouteConfig) setupPublicRoutes(api fiber.Router) {
	auth := api.Group("/auth")
	auth.Post("/register", c.TenantAuthController.Register)
	auth.Post("/login", c.TenantAuthController.Login)
	auth.Get("/verify-email", c.TenantAuthController.VerifyEmail)
	auth.Post("/resend-verification", c.TenantAuthController.ResendVerification)
}

// setupOnboardingRoutes registers the onboarding endpoint.
// This is separate from the main protected routes because users accessing
// onboarding have a valid JWT but no client_id yet.
func (c *TenantRouteConfig) setupOnboardingRoutes(api fiber.Router) {
	clients := api.Group("/clients", c.TenantAuthMiddleware)
	clients.Post("/onboarding", c.TenantAuthController.Onboarding)
}

func (c *TenantRouteConfig) setupProtectedRoutes(api fiber.Router) {
	// All protected tenant routes go through TenantAuth middleware
	protected := api.Group("/tenant", c.TenantAuthMiddleware)

	// ── Auth (Protected) ────────────────────────────────────
	authProtected := api.Group("/auth", c.TenantAuthMiddleware)
	authProtected.Get("/me", c.TenantAuthController.Me)

	// ── Bots ─────────────────────────────────────────────────────────
	bots := protected.Group("/bots")
	bots.Get("/", middleware.TenantRequirePermission("bots.read"), c.TelegramBotController.List)
	bots.Get("/:id", middleware.TenantRequirePermission("bots.read"), c.TelegramBotController.Get)
	bots.Post("/", middleware.TenantRequirePermission("bots.create"), c.TelegramBotController.Create)
	bots.Put("/:id", middleware.TenantRequirePermission("bots.update"), c.TelegramBotController.Update)
	bots.Delete("/:id", middleware.TenantRequirePermission("bots.delete"), c.TelegramBotController.Delete)

	// ── Custom Commands ──────────────────────────────────────────────
	commands := protected.Group("/commands")
	commands.Get("/", middleware.TenantRequirePermission("bots.read"), c.CustomCommandController.List)
	commands.Get("/:id", middleware.TenantRequirePermission("bots.read"), c.CustomCommandController.Get)
	commands.Post("/", middleware.TenantRequirePermission("bots.create"), c.CustomCommandController.Create)
	commands.Put("/:id", middleware.TenantRequirePermission("bots.update"), c.CustomCommandController.Update)
	commands.Delete("/:id", middleware.TenantRequirePermission("bots.delete"), c.CustomCommandController.Delete)

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

	// ── Members ──────────────────────────────────────────────────────
	members := protected.Group("/members")
	members.Get("/", middleware.TenantRequirePermission("members.read"), c.MemberController.List)
	members.Get("/:id", middleware.TenantRequirePermission("members.read"), c.MemberController.Get)
	members.Post("/:id/kick", middleware.TenantRequirePermission("members.update"), c.MemberController.Kick)
	members.Post("/:id/extend", middleware.TenantRequirePermission("members.update"), c.MemberController.Extend)
	members.Post("/:id/sync", middleware.TenantRequirePermission("members.update"), c.MemberController.Sync)
	members.Post("/:id/resend-link", middleware.TenantRequirePermission("members.update"), c.MemberController.ResendLink)

	// ── Transactions ─────────────────────────────────────────────────
	transactions := protected.Group("/transactions")
	transactions.Get("/", middleware.TenantRequirePermission("members.read"), c.TenantTransactionController.List)

	// ── Analytics & Audit ────────────────────────────────────────────────────
	analytics := protected.Group("/analytics")
	analytics.Get("/overview", c.TenantAnalyticsController.GetOverview)

	audit := protected.Group("/audit-logs")
	audit.Get("/", c.AuditLogController.ListTenantLogs)

	// ── Settings ─────────────────────────────────────────────────────────────
	settings := protected.Group("/settings")
	settings.Put("/payment", middleware.TenantRequireRole("owner", "admin"), c.TenantProfileController.UpdatePaymentSettings)
	settings.Put("/profile", middleware.TenantRequireRole("owner", "admin"), c.TenantProfileController.UpdateProfile)

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
