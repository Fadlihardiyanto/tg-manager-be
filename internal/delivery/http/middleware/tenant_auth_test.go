package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func generateTestTenantToken(t *testing.T, cfg *pkg_jwt.JWTConfig) string {
	t.Helper()
	token, _, _, err := pkg_jwt.GenerateTenantTokens(context.Background(), uuid.New(), uuid.New(), "owner", []string{"all"}, cfg)
	if err != nil {
		t.Fatalf("GenerateTenantTokens() error = %v", err)
	}
	return token
}

func TestTenantAuth_MissingHeader(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", TenantAuth(cfg))

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestTenantAuth_ValidToken(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", TenantAuth(cfg), func(c fiber.Ctx) error {
		claims := c.Locals(LocalsTenantClaims).(*pkg_jwt.TenantClaims)
		if claims == nil {
			t.Error("tenant_claims should be set")
		}
		return c.SendString("ok")
	})

	token := generateTestTenantToken(t, cfg)
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

func TestTenantAuth_AdminTokenOnTenantRoute(t *testing.T) {
	app := testApp()
	cfg := testJWTConfig()
	app.Get("/test", TenantAuth(cfg))

	adminToken := generateTestAdminToken(t, cfg)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestTenantRequirePermission_OwnerBypass(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsTenantRole, "owner")
		c.Locals(LocalsTenantPermissions, []string{})
		return c.Next()
	}, TenantRequirePermission("anything"), func(c fiber.Ctx) error {
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

func TestTenantRequirePermission_HasPermission(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsTenantRole, "admin")
		c.Locals(LocalsTenantPermissions, []string{"bots.create"})
		return c.Next()
	}, TenantRequirePermission("bots.create"), func(c fiber.Ctx) error {
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

func TestTenantRequirePermission_MissingPermission(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsTenantRole, "viewer")
		c.Locals(LocalsTenantPermissions, []string{"bots.read"})
		return c.Next()
	}, TenantRequirePermission("bots.delete"), func(c fiber.Ctx) error {
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

func TestTenantRequireAnyPermission_OwnerBypass(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsTenantRole, "owner")
		c.Locals(LocalsTenantPermissions, nil)
		return c.Next()
	}, TenantRequireAnyPermission("anything"), func(c fiber.Ctx) error {
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

func TestTenantRequireRole_Matching(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsTenantRole, "admin")
		return c.Next()
	}, TenantRequireRole("owner", "admin"), func(c fiber.Ctx) error {
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

func TestTenantRequireRole_NotMatching(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsTenantRole, "viewer")
		return c.Next()
	}, TenantRequireRole("owner", "admin"), func(c fiber.Ctx) error {
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

func TestGetTenantUserID_NotFound(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		id := GetTenantUserID(c)
		if id != uuid.Nil {
			t.Errorf("expected nil UUID, got %v", id)
		}
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
