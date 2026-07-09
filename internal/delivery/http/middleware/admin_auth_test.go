package middleware

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func testJWTConfig() *pkg_jwt.JWTConfig {
	return &pkg_jwt.JWTConfig{
		AdminSecretKey:      "admin-secret-key-for-testing-purposes-123",
		AdminAccessExpiry:   time.Hour,
		AdminRefreshExpiry:  24 * time.Hour,
		TenantSecretKey:     "tenant-secret-key-for-testing-purposes-456",
		TenantAccessExpiry:  time.Hour,
		TenantRefreshExpiry: 24 * time.Hour,
		Issuer:              "test-issuer",
	}
}

func testApp() *fiber.App {
	return fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			switch e := err.(type) {
			case *helper.ErrForbidden:
				return c.Status(fiber.StatusForbidden).SendString(err.Error())
			case *helper.ErrUnauthorized:
				return c.Status(fiber.StatusUnauthorized).SendString(err.Error())
			case *fiber.Error:
				return c.Status(e.Code).SendString(e.Message)
			default:
				return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
			}
		},
	})
}

func generateTestAdminToken(t *testing.T, cfg *pkg_jwt.JWTConfig) string {
	t.Helper()
	adminID := uuid.New()
	token, _, _, err := pkg_jwt.GenerateAdminTokens(context.Background(), adminID, "admin@test.com", []string{"superadmin"}, []string{"all"}, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}
	return token
}

func TestAdminAuth_MissingHeader(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", AdminAuth(cfg))

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestAdminAuth_InvalidToken(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", AdminAuth(cfg))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestAdminAuth_ValidToken(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", AdminAuth(cfg), func(c fiber.Ctx) error {
		claims := c.Locals(LocalsAdminClaims).(*pkg_jwt.AdminClaims)
		if claims == nil {
			t.Error("admin_claims should be set")
		}
		return c.SendString("ok")
	})

	token := generateTestAdminToken(t, cfg)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestAdminAuth_TenantTokenOnAdminRoute(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", AdminAuth(cfg))

	tenantToken, _, _, err := pkg_jwt.GenerateTenantTokens(context.Background(), uuid.New(), uuid.New(), "owner", nil, cfg)
	if err != nil {
		t.Fatalf("GenerateTenantTokens() error = %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tenantToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	// token signed with tenant secret → signature verification fails at admin parser
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestRequirePermission_SuperAdminBypass(t *testing.T) {
	app := testApp()
	handler := RequirePermission("some.restricted.action")
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"superadmin"})
		c.Locals(LocalsAdminPermissions, []string{})
		return c.Next()
	}, handler, func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestRequirePermission_HasPermission(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin"})
		c.Locals(LocalsAdminPermissions, []string{"packages.read"})
		return c.Next()
	}, RequirePermission("packages.read"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestRequirePermission_MissingPermission(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin"})
		c.Locals(LocalsAdminPermissions, []string{"packages.read"})
		return c.Next()
	}, RequirePermission("packages.delete"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusForbidden)
	}
}

func TestRequireAnyPermission_SuperAdminBypass(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"superadmin"})
		c.Locals(LocalsAdminPermissions, []string{})
		return c.Next()
	}, RequireAnyPermission("anything"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestRequireAnyPermission_HasOne(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin"})
		c.Locals(LocalsAdminPermissions, []string{"bots.create"})
		return c.Next()
	}, RequireAnyPermission("bots.read", "bots.create"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestRequireAnyPermission_HasNone(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin"})
		c.Locals(LocalsAdminPermissions, []string{"bots.read"})
		return c.Next()
	}, RequireAnyPermission("bots.delete", "bots.create"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusForbidden)
	}
}

func TestRequireRole_HasRole(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin", "manager"})
		return c.Next()
	}, RequireRole("manager", "viewer"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
}

func TestRequireRole_MissingRole(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"viewer"})
		return c.Next()
	}, RequireRole("admin", "manager"), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusForbidden)
	}
}
