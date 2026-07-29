package route

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/controller"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/ratelimit"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AdminRouteConfig struct {
	App *fiber.App
	Log *zap.Logger

	// Rate Limiter
	AdminAuthRateLimiter *ratelimit.RateLimiter

	// Auth
	AdminAuthController *controller.AdminAuthController

	// RBAC
	AdminRoleController       *controller.AdminRoleController
	AdminPermissionController *controller.AdminPermissionController
	AdminUserController       *controller.AdminUserController
	AdminClientController     *controller.AdminClientController
	AdminClientUserController *controller.AdminClientUserController

	// platform plan & billing
	PlatformPlanController      *controller.PlatformPlanController
	ClientBillingController     *controller.ClientBillingController
	PlatformDiscountController  *controller.PlatformDiscountController
	AuditLogController          *controller.AuditLogController

	// impersonation
	AdminImpersonationController *controller.AdminImpersonationController

	// Middleware
	AdminAuthMiddleware fiber.Handler // JWT validation
}

func (c *AdminRouteConfig) Setup() {
	c.App.Use(requestid.New(requestid.Config{
		Generator: func() string {
			return uuid.New().String()
		},
	}))
	c.App.Use(middleware.RequestContext())

	admin := c.App.Group("/admin/v1")

	c.setupPublicRoutes(admin)
	c.setupProtectedRoutes(admin)
}

func (c *AdminRouteConfig) setupPublicRoutes(admin fiber.Router) {
	// health check
	admin.Get("/health", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"status": "ok",
		})
	})

	auth := admin.Group("/auth")
	auth.Use(c.AdminAuthRateLimiter.Middleware())
	auth.Post("/login", c.AdminAuthController.Login)
	auth.Post("/otp/verify", c.AdminAuthController.VerifyOTP)
	auth.Post("/otp/resend", c.AdminAuthController.ResendOTP)
	auth.Post("/refresh", c.AdminAuthController.RefreshToken)
	auth.Get("/verify-email", c.AdminAuthController.VerifyEmail)

}

func (c *AdminRouteConfig) setupProtectedRoutes(admin fiber.Router) {
	// Semua route di bawah ini wajib melewati JWT middleware
	protected := admin.Group("/", c.AdminAuthMiddleware)

	// Auth actions yang butuh login
	protected.Post("/auth/logout", c.AdminAuthController.Logout)
	protected.Post("/auth/2fa/setup", c.AdminAuthController.Setup2FA)
	protected.Post("/auth/2fa/enable", c.AdminAuthController.Enable2FA)
	protected.Get("/auth/me", c.AdminAuthController.Me)

	// Role management
	roles := protected.Group("/roles")
	roles.Get("/", middleware.Authorize("roles.read"), c.AdminRoleController.List)
	roles.Post("/", middleware.Authorize("roles.create"), c.AdminRoleController.Create)
	roles.Get("/:id", middleware.Authorize("roles.read"), c.AdminRoleController.GetByID)
	roles.Put("/:id", middleware.Authorize("roles.update"), c.AdminRoleController.Update)
	roles.Delete("/:id", middleware.Authorize("roles.delete"), c.AdminRoleController.Delete)
	roles.Put("/:id/permissions", middleware.Authorize("roles.update"), c.AdminRoleController.SyncPermissions)

	// Permission management (read-only, permissions dibuat via seeder)
	permissions := protected.Group("/permissions")
	permissions.Get("/", middleware.Authorize("roles.read"), c.AdminPermissionController.List)
	permissions.Get("/:id", middleware.Authorize("roles.read"), c.AdminPermissionController.GetByID)

	// Admin user management
	admins := protected.Group("/admins")
	admins.Get("/", middleware.Authorize("admins.read"), c.AdminUserController.List)
	admins.Post("/", middleware.Authorize("admins.create"), c.AdminUserController.Create)
	admins.Get("/:id", middleware.Authorize("admins.read"), c.AdminUserController.GetByID)
	admins.Put("/:id", middleware.Authorize("admins.update"), c.AdminUserController.Update)
	admins.Delete("/:id", middleware.Authorize("admins.delete"), c.AdminUserController.Delete)
	admins.Patch("/:id/activate", middleware.Authorize("admins.update"), c.AdminUserController.Activate)
	admins.Patch("/:id/deactivate", middleware.Authorize("admins.update"), c.AdminUserController.Deactivate)
	admins.Put("/:id/roles", middleware.Authorize("admins.update"), c.AdminUserController.SyncRoles)

	// Client (tenant) management
	clients := protected.Group("/clients")
	clients.Get("/", middleware.Authorize("clients.read"), c.AdminClientController.List)
	clients.Post("/", middleware.Authorize("clients.create"), c.AdminClientController.Create)
	clients.Get("/:id", middleware.Authorize("clients.read"), c.AdminClientController.GetByID)
	clients.Put("/:id", middleware.Authorize("clients.update"), c.AdminClientController.Update)
	clients.Delete("/:id", middleware.Authorize("clients.delete"), c.AdminClientController.Delete)
	clients.Patch("/:id/activate", middleware.Authorize("clients.update"), c.AdminClientController.Activate)
	clients.Patch("/:id/deactivate", middleware.Authorize("clients.update"), c.AdminClientController.Deactivate)

	clientUsers := clients.Group("/:client_id/users")
	clientUsers.Get("/", middleware.Authorize("clients.read"), c.AdminClientUserController.List)
	clientUsers.Post("/", middleware.Authorize("clients.update"), c.AdminClientUserController.Create)
	clientUsers.Get("/:user_id", middleware.Authorize("clients.read"), c.AdminClientUserController.GetByID)
	clientUsers.Put("/:user_id", middleware.Authorize("clients.update"), c.AdminClientUserController.Update)
	clientUsers.Delete("/:user_id", middleware.Authorize("clients.update"), c.AdminClientUserController.Delete)

	// Impersonation
	clients.Post("/:id/impersonate", middleware.Authorize("clients.impersonate"), c.AdminImpersonationController.Start)
	clients.Get("/:id/impersonation-logs", middleware.Authorize("clients.read"), c.AdminImpersonationController.ListByClient)

	plans := protected.Group("/billing/plans")
	plans.Get("/", middleware.Authorize("billing.read"), c.PlatformPlanController.List)
	plans.Post("/", middleware.Authorize("billing.manage"), c.PlatformPlanController.Create)
	plans.Get("/:id", middleware.Authorize("billing.read"), c.PlatformPlanController.GetByID)
	plans.Put("/:id", middleware.Authorize("billing.manage"), c.PlatformPlanController.Update)
	plans.Delete("/:id", middleware.Authorize("billing.manage"), c.PlatformPlanController.Delete)

	billing := protected.Group("/billing")
	billing.Get("/", middleware.Authorize("billing.read"), c.ClientBillingController.AdminList)
	billing.Post("/assign", middleware.Authorize("billing.manage"), c.ClientBillingController.AdminAssignPlan)
	billing.Delete("/:id", middleware.Authorize("billing.manage"), c.ClientBillingController.CancelBilling)

	discounts := billing.Group("/discounts")
	discounts.Get("/", middleware.Authorize("billing.read"), c.PlatformDiscountController.List)
	discounts.Post("/", middleware.Authorize("billing.manage"), c.PlatformDiscountController.Create)
	discounts.Get("/:id", middleware.Authorize("billing.read"), c.PlatformDiscountController.GetByID)
	discounts.Put("/:id", middleware.Authorize("billing.manage"), c.PlatformDiscountController.Update)
	discounts.Delete("/:id", middleware.Authorize("billing.manage"), c.PlatformDiscountController.Delete)

	// ── Audit Logs ───────────────────────────────────────────────────
	audit := protected.Group("/audit-logs")
	audit.Get("/", middleware.Authorize("analytics.read"), c.AuditLogController.ListPlatformLogs)

	// ── Impersonation ────────────────────────────────────────────────
	impersonation := protected.Group("/impersonation")
	impersonation.Post("/end", middleware.Authorize("clients.impersonate"), c.AdminImpersonationController.End)
	impersonation.Get("/logs", middleware.Authorize("clients.read"), c.AdminImpersonationController.ListByAdmin)
}
