package middleware

import (
	"strings"

	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/gofiber/fiber/v3"
)

const (
	LocalsAdminClaims      = "admin_claims"
	LocalsAdminID          = "admin_id"
	LocalsAdminRoles       = "admin_roles"
	LocalsAdminPermissions = "admin_permissions"
	LocalsClientID         = "client_id"
)

// AdminAuth memvalidasi JWT admin dan menyimpan claims ke context.
// Ini adalah layer 1 RBAC — cek dari JWT claims tanpa hit DB.
func AdminAuth(jwtConfig *pkg_jwt.JWTConfig) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		authHeader := ctx.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "missing or invalid authorization header")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := pkg_jwt.ParseAdminToken(tokenStr, jwtConfig.SecretKey)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
		}

		// Hardened: pastikan type claim = "admin"
		if claims.Type != "admin" {
			return fiber.NewError(fiber.StatusForbidden, "forbidden: not an admin token")
		}

		ctx.Locals(LocalsAdminClaims, claims)
		ctx.Locals(LocalsAdminID, claims.AdminID)
		ctx.Locals(LocalsAdminRoles, claims.Roles)
		ctx.Locals(LocalsAdminPermissions, claims.Permissions)

		requestCtx := logger.WithClientID(ctx.Context(), claims.AdminID.String())
		ctx.SetContext(requestCtx)

		return ctx.Next()
	}
}

// RequirePermission adalah layer 1 RBAC untuk permission granular.
// Superadmin bypass semua permission check.
func RequirePermission(permission string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		roles := getAdminRoles(ctx)

		// Superadmin bypass
		if rbac.IsSuperAdmin(roles) {
			return ctx.Next()
		}

		permissions := getAdminPermissions(ctx)
		if !rbac.HasPermission(permissions, permission) {
			return fiber.NewError(fiber.StatusForbidden, "forbidden: insufficient permissions")
		}

		return ctx.Next()
	}
}

// RequireAnyPermission lolos jika punya setidaknya satu dari permissions.
func RequireAnyPermission(permissions ...string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		roles := getAdminRoles(ctx)
		if rbac.IsSuperAdmin(roles) {
			return ctx.Next()
		}

		adminPermissions := getAdminPermissions(ctx)
		if !rbac.HasAnyPermission(adminPermissions, permissions...) {
			return fiber.NewError(fiber.StatusForbidden, "forbidden: insufficient permissions")
		}

		return ctx.Next()
	}
}

// RequireRole lolos jika punya setidaknya satu dari roles yang dibutuhkan.
func RequireRole(roles ...string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		adminRoles := getAdminRoles(ctx)
		if !rbac.HasAnyPermission(adminRoles, roles...) {
			return fiber.NewError(fiber.StatusForbidden, "forbidden: insufficient role")
		}
		return ctx.Next()
	}
}

// ── Context helpers ───────────────────────────────────────────────────────────

func getAdminRoles(ctx fiber.Ctx) []string {
	roles, _ := ctx.Locals(LocalsAdminRoles).([]string)
	return roles
}

func getAdminPermissions(ctx fiber.Ctx) []string {
	perms, _ := ctx.Locals(LocalsAdminPermissions).([]string)
	return perms
}
