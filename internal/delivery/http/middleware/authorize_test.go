package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func TestAuthorize_SuperAdminBypass(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"superadmin"})
		c.Locals(LocalsAdminID, uuid.New())
		return c.Next()
	}, Authorize("anything.restricted"), func(c fiber.Ctx) error {
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

func TestAuthorize_HasPermission(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin"})
		c.Locals(LocalsAdminPermissions, []string{"roles.read"})
		c.Locals(LocalsAdminID, uuid.New())
		return c.Next()
	}, Authorize("roles.read"), func(c fiber.Ctx) error {
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

func TestAuthorize_MissingPermission(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		c.Locals(LocalsAdminRoles, []string{"admin"})
		c.Locals(LocalsAdminPermissions, []string{"roles.read"})
		c.Locals(LocalsAdminID, uuid.New())
		return c.Next()
	}, Authorize("roles.delete"), func(c fiber.Ctx) error {
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

func TestGetAdminID_NotFound(t *testing.T) {
	app := testApp()
	app.Get("/test", func(c fiber.Ctx) error {
		id := GetAdminID(c)
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
