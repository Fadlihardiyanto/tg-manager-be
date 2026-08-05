package middleware

import (
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/gofiber/fiber/v3"
)

func EnforceFeature(fgUC usecase.IFeatureGateUseCase, featureKey string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		clientID := GetTenantClientID(ctx)

		allowed, err := fgUC.CanUseFeature(ctx.Context(), clientID, featureKey)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Gagal memeriksa batasan fitur")
		}
		if !allowed {
			fmt.Println("test")
			return helper.NewForbidden("Fitur ini tidak tersedia di paket Anda. Silakan upgrade untuk mengakses fitur ini.")
		}

		return ctx.Next()
	}
}

func EnforceQuota(fgUC usecase.IFeatureGateUseCase, resourceType string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		clientID := GetTenantClientID(ctx)

		allowed, _, _, err := fgUC.CheckQuota(ctx.Context(), clientID, resourceType)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "Gagal memeriksa kuota")
		}
		if !allowed {
			return helper.NewForbidden("Kuota resource Anda sudah penuh. Silakan upgrade paket untuk menambah kuota.")
		}

		return ctx.Next()
	}
}
