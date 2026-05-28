package middleware

import (
	"fmt"
	"strings"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// Locals key constants for tenant context.
// These are separate from admin locals to prevent cross-contamination.
const (
	LocalsTenantClaims      = "tenant_claims"
	LocalsTenantUserID      = "tenant_user_id"
	LocalsTenantClientID    = "tenant_client_id"
	LocalsTenantRole        = "tenant_role"
	LocalsTenantPermissions = "tenant_permissions"
)

func TenantAuth(jwtConfig *pkg_jwt.JWTConfig) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		authHeader := ctx.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			fmt.Println("Authorization header missing or does not start with Bearer")
			return helper.NewUnauthorized("Token autentikasi diperlukan")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := pkg_jwt.ParseTenantToken(tokenStr, jwtConfig.SecretKey)
		if err != nil {
			return helper.NewUnauthorized("Token tidak valid atau sudah kedaluwarsa")
		}

		// Hardened: pastikan type claim = "tenant"
		if claims.Type != pkg_jwt.TokenTypeTenant {
			return helper.NewForbidden("Token bukan untuk tenant")
		}

		// Store claims in Fiber Locals
		ctx.Locals(LocalsTenantClaims, claims)
		ctx.Locals(LocalsTenantUserID, claims.UserID)
		ctx.Locals(LocalsTenantClientID, claims.ClientID)
		ctx.Locals(LocalsTenantRole, claims.Role)
		ctx.Locals(LocalsTenantPermissions, claims.Permissions)

		// Inject user_id and client_id into context for structured logging
		requestCtx := logger.WithClientID(ctx.Context(), claims.ClientID.String())
		ctx.SetContext(requestCtx)

		return ctx.Next()
	}
}

// TenantRequirePermission checks if the tenant user has a specific permission.
// Owner role bypasses all permission checks (equivalent to superadmin in admin layer).
//
// Usage:
//
//	bots.Post("/", TenantAuth(jwt), TenantRequirePermission("bots.create"), handler)
func TenantRequirePermission(permission string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		role := getTenantRole(ctx)

		// Owner bypass — owner has all permissions implicitly
		if role == "owner" {
			return ctx.Next()
		}

		permissions := getTenantPermissions(ctx)
		if !rbac.HasPermission(permissions, permission) {
			return helper.NewForbidden("Anda tidak memiliki izin untuk melakukan aksi ini")
		}

		return ctx.Next()
	}
}

// TenantRequireAnyPermission passes if the user has at least one of the required permissions.
// Owner role bypasses.
func TenantRequireAnyPermission(permissions ...string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		role := getTenantRole(ctx)
		if role == "owner" {
			return ctx.Next()
		}

		tenantPermissions := getTenantPermissions(ctx)
		if !rbac.HasAnyPermission(tenantPermissions, permissions...) {
			return helper.NewForbidden("Anda tidak memiliki izin untuk melakukan aksi ini")
		}

		return ctx.Next()
	}
}

// TenantRequireRole passes if the user has at least one of the required roles.
// Useful for route-level gating (e.g., only owner and admin can invite).
//
// Usage:
//
//	team.Post("/invite", TenantAuth(jwt), TenantRequireRole("owner", "admin"), handler)
func TenantRequireRole(roles ...string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		role := getTenantRole(ctx)

		for _, r := range roles {
			if role == r {
				return ctx.Next()
			}
		}

		return helper.NewForbidden("Role Anda tidak memiliki akses ke fitur ini")
	}
}

// ── Context helpers ───────────────────────────────────────────────────────────

func getTenantRole(ctx fiber.Ctx) string {
	role, _ := ctx.Locals(LocalsTenantRole).(string)
	return role
}

func getTenantPermissions(ctx fiber.Ctx) []string {
	perms, _ := ctx.Locals(LocalsTenantPermissions).([]string)
	return perms
}

// GetTenantUserID is a public helper for controllers/usecases to extract user ID from context.
func GetTenantUserID(ctx fiber.Ctx) uuid.UUID {
	id, _ := ctx.Locals(LocalsTenantUserID).(uuid.UUID)
	return id
}

// GetTenantClientID is a public helper for controllers/usecases to extract client ID from context.
func GetTenantClientID(ctx fiber.Ctx) uuid.UUID {
	id, _ := ctx.Locals(LocalsTenantClientID).(uuid.UUID)
	return id
}

// GetTenantClaims is a public helper to get the full claims struct.
func GetTenantClaims(ctx fiber.Ctx) *pkg_jwt.TenantClaims {
	claims, _ := ctx.Locals(LocalsTenantClaims).(*pkg_jwt.TenantClaims)
	return claims
}
