package middleware

import (
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// Authorize adalah layer 1 RBAC — cek permission dari JWT claims.
// Superadmin otomatis bypass.
// Dipanggil per-route, contoh: middleware.Authorize("roles.read")
func Authorize(permission string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		roles := GetAdminRoles(ctx)

		// Superadmin bypass semua permission check
		if rbac.IsSuperAdmin(roles) {
			return ctx.Next()
		}

		permissions := GetAdminPermissions(ctx)
		if !rbac.HasPermission(permissions, permission) {
			return fiber.NewError(fiber.StatusForbidden, "forbidden: insufficient permissions")
		}

		return ctx.Next()
	}
}

// GetAdminID mengambil admin ID dari context locals
func GetAdminID(ctx fiber.Ctx) uuid.UUID {
	id, _ := ctx.Locals(LocalsAdminID).(uuid.UUID)
	return id
}

// GetAdminRoles mengambil roles dari context locals
func GetAdminRoles(ctx fiber.Ctx) []string {
	roles, _ := ctx.Locals(LocalsAdminRoles).([]string)
	return roles
}

// GetAdminPermissions mengambil permissions dari context locals
func GetAdminPermissions(ctx fiber.Ctx) []string {
	perms, _ := ctx.Locals(LocalsAdminPermissions).([]string)
	return perms
}

func GetClientID(ctx fiber.Ctx) uuid.UUID {
	id, _ := ctx.Locals(LocalsClientID).(uuid.UUID)
	return id
}
