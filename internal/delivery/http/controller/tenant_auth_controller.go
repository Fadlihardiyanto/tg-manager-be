package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"time"

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

	return helper.Created(ctx, "Registrasi akun berhasil, silakan cek email untuk verifikasi", result)
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
	if result == nil {
		log.Error("tenant auth login returned nil result")
		return helper.InternalError(ctx, "Terjadi kesalahan pada server")
	}
	log.Info("tenant auth login succeeded")

	ctx.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		Path:     "/",
	})

	return helper.Success(ctx, "Login berhasil", result)
}

// Refresh godoc
// POST /api/v1/auth/refresh
func (c *TenantAuthController) Refresh(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant auth refresh request")

	refreshToken := ctx.Cookies("refresh_token")
	if refreshToken == "" {
		return helper.NewUnauthorized("Sesi tidak valid, silakan login kembali")
	}

	result, err := c.tenantAuthUC.Refresh(ctx.Context(), refreshToken)
	if err != nil {
		log.Error("tenant auth refresh failed", zap.Error(err))
		return err
	}
	if result == nil {
		log.Error("tenant auth refresh returned nil result")
		return helper.InternalError(ctx, "Terjadi kesalahan pada server")
	}

	ctx.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    result.RefreshToken,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		Path:     "/",
	})

	log.Info("tenant auth refresh succeeded")

	return helper.Success(ctx, "Sesi berhasil diperbarui", result)
}

// Logout godoc
// POST /api/v1/auth/logout
func (c *TenantAuthController) Logout(ctx fiber.Ctx) error {
	ctx.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		Expires:  time.Now().Add(-1 * time.Hour), // Expire immediately
		Path:     "/",
	})
	return helper.Success(ctx, "Logout berhasil", nil)
}

// Me godoc
// GET /api/v1/auth/me
func (c *TenantAuthController) Me(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)

	userID := middleware.GetTenantUserID(ctx)
	clientID := middleware.GetTenantClientID(ctx)

	log.Info("tenant auth get profile request", zap.String("user_id", userID.String()), zap.String("client_id", clientID.String()))

	res, err := c.tenantAuthUC.GetProfile(ctx.Context(), userID, clientID)
	if err != nil {
		log.Error("tenant auth get profile failed", zap.Error(err))
		return err
	}

	return helper.Success(ctx, "Profil user berhasil diambil", res)
}

// VerifyEmail godoc
// GET /api/v1/auth/verify-email?token=...
// After successful verification, auto-logs the user in and returns JWT tokens.
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

	result, err := c.tenantAuthUC.VerifyEmail(ctx.Context(), &req)
	if err != nil {
		log.Error("tenant auth verify email failed", zap.Error(err))
		return err
	}
	log.Info("tenant auth verify email succeeded")

	return helper.Success(ctx, "Email berhasil diverifikasi", result)
}

// ResendVerification godoc
// POST /api/v1/auth/resend-verification
// Resends the email verification link to the user's email address.
func (c *TenantAuthController) ResendVerification(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant auth resend verification request")

	var req model.TenantResendVerificationRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("tenant auth resend verification bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("tenant auth resend verification validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	err := c.tenantAuthUC.ResendVerification(ctx.Context(), &req)
	if err != nil {
		log.Error("tenant auth resend verification failed", zap.Error(err))
		return err
	}
	log.Info("tenant auth resend verification succeeded")

	return helper.Success(ctx, "Link verifikasi telah dikirim ulang ke email Anda", nil)
}

// Onboarding godoc
// POST /api/v1/clients/onboarding
// Creates a new business (client) for the authenticated user.
// Requires TenantAuth middleware — user must be logged in.
func (c *TenantAuthController) Onboarding(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant onboarding request")

	userID := middleware.GetTenantUserID(ctx)

	var req model.TenantOnboardingRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("tenant onboarding bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("tenant onboarding validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.tenantAuthUC.Onboarding(ctx.Context(), userID, &req)
	if err != nil {
		log.Error("tenant onboarding failed", zap.Error(err))
		return err
	}
	log.Info("tenant onboarding succeeded")

	return helper.Created(ctx, "Bisnis berhasil dibuat", result)
}
