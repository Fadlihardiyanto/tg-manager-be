package controller

import (
	"fmt"

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
		return helper.BadRequest(ctx, "ID tenant tidak ditemukan di konteks")
	}

	var req model.PaymentSettingsUpdateRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Isi request tidak valid")
	}

	if err := c.validate.Struct(&req); err != nil {
		return helper.BadRequest(ctx, "Validasi gagal: "+err.Error())
	}

	res, err := c.uc.UpdatePaymentSettings(ctx.Context(), clientID, &req)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Pengaturan pembayaran berhasil diperbarui", res)
}

func (c *TenantProfileController) UpdateProfile(ctx fiber.Ctx) error {
	clientID := middleware.GetTenantClientID(ctx)
	if clientID == uuid.Nil {
		return helper.BadRequest(ctx, "ID tenant tidak ditemukan di konteks")
	}

	var req model.ClientUpdateRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Isi request tidak valid")
	}

	if err := c.validate.Struct(&req); err != nil {
		return helper.BadRequest(ctx, "Validasi gagal: "+err.Error())
	}

	fmt.Println("UpdateProfile request: ", req)

	res, err := c.uc.UpdateProfile(ctx.Context(), clientID, &req)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Profil bisnis berhasil diperbarui", res)
}
