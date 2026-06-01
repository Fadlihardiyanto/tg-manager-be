package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TenantProfileController struct {
	uc       usecase.ITenantProfileUseCase
	log      *zap.Logger
	validate *validator.Validate
}

func NewTenantProfileController(uc usecase.ITenantProfileUseCase, log *zap.Logger, validate *validator.Validate) *TenantProfileController {
	return &TenantProfileController{
		uc:       uc,
		log:      log,
		validate: validate,
	}
}

func (c *TenantProfileController) UpdatePaymentSettings(ctx fiber.Ctx) error {
	clientID := middleware.GetTenantClientID(ctx)
	if clientID == uuid.Nil {
		return helper.BadRequest(ctx, "Unauthorized: Tenant ID not found in context")
	}

	var req model.PaymentSettingsUpdateRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Invalid request body")
	}

	if err := c.validate.Struct(&req); err != nil {
		return helper.BadRequest(ctx, "Validation failed: " + err.Error())
	}

	res, err := c.uc.UpdatePaymentSettings(ctx.Context(), clientID, &req)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Pengaturan pembayaran berhasil diperbarui", res)
}
