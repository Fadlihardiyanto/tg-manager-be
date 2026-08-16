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
		return helper.BadRequest(ctx, "ID tenant tidak ditemukan di konteks")
	}

	var req model.PaymentSettingsUpdateRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Isi request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validate, &req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	res, err := c.uc.UpdatePaymentSettings(ctx.Context(), clientID, &req)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Pengaturan pembayaran berhasil diperbarui", res)
}

// GetPaymentSettings godoc
// GET /api/v1/tenant/settings/payment
func (c *TenantProfileController) GetPaymentSettings(ctx fiber.Ctx) error {
	clientID := middleware.GetTenantClientID(ctx)
	if clientID == uuid.Nil {
		return helper.BadRequest(ctx, "ID tenant tidak ditemukan di konteks")
	}

	res, err := c.uc.GetPaymentSettings(ctx.Context(), clientID)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Pengaturan pembayaran berhasil diambil", res)
}

// PostKeyExchange godoc
// POST /api/v1/tenant/settings/payment/key-exchange/initiate
func (c *TenantProfileController) PostKeyExchange(ctx fiber.Ctx) error {
	clientID := middleware.GetTenantClientID(ctx)
	if clientID == uuid.Nil {
		return helper.BadRequest(ctx, "ID tenant tidak ditemukan di konteks")
	}

	var req model.KeyExchangeRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Isi request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validate, &req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	res, err := c.uc.InitiateKeyExchange(ctx.Context(), clientID, req.ClientPublicKey)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Key exchange berhasil", res)
}

// PutPaymentEncrypted godoc
// PUT /api/v1/tenant/settings/payment/encrypted
func (c *TenantProfileController) PutPaymentEncrypted(ctx fiber.Ctx) error {
	clientID := middleware.GetTenantClientID(ctx)
	if clientID == uuid.Nil {
		return helper.BadRequest(ctx, "ID tenant tidak ditemukan di konteks")
	}

	var req model.PaymentSettingsUpdateEncryptedRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return helper.BadRequest(ctx, "Isi request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validate, &req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	res, err := c.uc.UpdatePaymentSettingsEncrypted(ctx.Context(), clientID, &req)
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

	if errs := helper.ValidateStruct(c.validate, &req); errs != nil {
		return helper.UnprocessableEntity(ctx, errs)
	}

	res, err := c.uc.UpdateProfile(ctx.Context(), clientID, &req)
	if err != nil {
		return err
	}

	return helper.Success(ctx, "Profil bisnis berhasil diperbarui", res)
}
