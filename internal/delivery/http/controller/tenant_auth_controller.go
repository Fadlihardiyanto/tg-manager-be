package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type TenantAuthController struct {
	tenantAuthUC usecase.ITenantAuthUseCase
	log          *zap.Logger
	validator    *validator.Validate
}

func NewTenantAuthController(uc usecase.ITenantAuthUseCase, log *zap.Logger, v *validator.Validate) *TenantAuthController {
	return &TenantAuthController{
		tenantAuthUC: uc,
		log:          log,
		validator:    v,
	}
}

// Register godoc
// POST /api/v1/auth/register
func (c *TenantAuthController) Register(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant registration request")

	var req model.TenantRegisterRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("tenant registration bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	req.Phone = helper.NormalizePhone(req.Phone)

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("tenant registration validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.tenantAuthUC.Register(ctx.Context(), &req)
	if err != nil {
		log.Error("tenant registration failed", zap.Error(err))
		return err
	}
	log.Info("tenant registration succeeded")

	return helper.Created(ctx, "Registrasi tenant bisnis berhasil", result)
}

// Login godoc
// POST /api/v1/auth/login
func (c *TenantAuthController) Login(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant auth login request")

	var req model.TenantLoginRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("tenant auth login bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("tenant auth login validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.tenantAuthUC.Login(ctx.Context(), &req)
	if err != nil {
		log.Error("tenant auth login failed", zap.Error(err))
		return err
	}
	log.Info("tenant auth login succeeded")

	return helper.Success(ctx, "Login berhasil", result)
}

// VerifyEmail godoc
// GET /api/v1/auth/verify-email?token=...
func (c *TenantAuthController) VerifyEmail(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant auth verify email request")

	token := ctx.Query("token")
	if token == "" {
		return helper.BadRequest(ctx, "Token verifikasi wajib diisi")
	}

	req := model.TenantVerifyEmailRequest{
		Token: token,
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("tenant auth verify email validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	if err := c.tenantAuthUC.VerifyEmail(ctx.Context(), &req); err != nil {
		log.Error("tenant auth verify email failed", zap.Error(err))
		return err
	}
	log.Info("tenant auth verify email succeeded")

	return helper.Success(ctx, "Email berhasil diverifikasi", nil)
}
