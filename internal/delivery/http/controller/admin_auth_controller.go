package controller

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type AdminAuthController struct {
	adminAuthUC usecase.IAdminAuthUseCase
	log         *zap.Logger
	validator   *validator.Validate
}

func NewAdminAuthController(uc usecase.IAdminAuthUseCase, log *zap.Logger, v *validator.Validate) *AdminAuthController {
	return &AdminAuthController{
		adminAuthUC: uc,
		log:         log,
		validator:   v,
	}
}

// Login godoc
// POST /admin/v1/auth/login
func (c *AdminAuthController) Login(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth login request")

	var req model.AdminLoginRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin auth login bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin auth login validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.ClientIP = ctx.IP()

	result, err := c.adminAuthUC.Login(ctx.Context(), &req)
	if err != nil {
		log.Error("admin auth login failed", zap.Error(err))
		return err
	}
	log.Info("admin auth login succeeded")

	// 202 Accepted jika butuh OTP, 200 OK jika langsung dapat token
	if result.Requires2FA {
		return ctx.Status(fiber.StatusAccepted).JSON(helper.Response{
			Success:   true,
			Code:      fiber.StatusAccepted,
			Message:   "Kode OTP telah dikirim ke email Anda",
			Data:      result,
			RequestID: ctx.GetRespHeader("X-Request-ID"),
		})
	}

	return helper.Success(ctx, "Login berhasil", result)
}

// VerifyOTP godoc
// POST /admin/v1/auth/otp/verify
func (c *AdminAuthController) VerifyOTP(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth otp verify request")

	var req model.AdminVerify2FARequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin auth otp verify bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin auth otp verify validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.ClientIP = ctx.IP()

	result, err := c.adminAuthUC.Verify2FA(ctx.Context(), &req)
	if err != nil {
		log.Error("admin auth otp verify failed", zap.Error(err))
		return err
	}
	log.Info("admin auth otp verify succeeded")

	return helper.Success(ctx, "Verifikasi OTP berhasil", result)
}

// ResendOTP godoc
// POST /admin/v1/auth/otp/resend
func (c *AdminAuthController) ResendOTP(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth otp resend request")

	var req model.AdminResendOTPRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin auth otp resend bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin auth otp resend validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	if err := c.adminAuthUC.ResendOTP(ctx.Context(), &req); err != nil {
		log.Error("admin auth otp resend failed", zap.Error(err))
		return err
	}
	log.Info("admin auth otp resend succeeded")

	return helper.Success(ctx, "Kode OTP baru telah dikirimkan ke email Anda", nil)
}

// RefreshToken godoc
// POST /admin/v1/auth/refresh
func (c *AdminAuthController) RefreshToken(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth refresh token request")

	var req model.AdminRefreshTokenRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin auth refresh token bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin auth refresh token validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	req.ClientIP = ctx.IP()

	result, err := c.adminAuthUC.RefreshToken(ctx.Context(), &req)
	if err != nil {
		log.Error("admin auth refresh token failed", zap.Error(err))
		return err
	}
	log.Info("admin auth refresh token succeeded")

	return helper.Success(ctx, "Token berhasil diperbarui", result)
}

// Logout godoc
// POST /admin/v1/auth/logout
func (c *AdminAuthController) Logout(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth logout request")

	adminID := middleware.GetAdminID(ctx)
	authHeader := ctx.Get("Authorization")
	tokenStr := ""
	if len(authHeader) > 7 {
		tokenStr = authHeader[7:] // trim "Bearer "
	}
	req := &model.AdminLogoutRequest{
		AdminID:     adminID,
		AccessToken: tokenStr,
	}

	if err := c.adminAuthUC.Logout(ctx.Context(), req); err != nil {
		log.Error("admin auth logout failed", zap.Error(err))
		return err
	}
	log.Info("admin auth logout succeeded")

	return helper.Success(ctx, "Logout berhasil", nil)
}

// Setup2FA godoc
// POST /admin/v1/auth/2fa/setup
func (c *AdminAuthController) Setup2FA(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth 2fa setup request")

	adminID := middleware.GetAdminID(ctx)
	req := &model.AdminSetup2FARequest{AdminID: adminID}

	result, err := c.adminAuthUC.Setup2FA(ctx.Context(), req)
	if err != nil {
		log.Error("admin auth 2fa setup failed", zap.Error(err))
		return err
	}
	log.Info("admin auth 2fa setup succeeded")

	return helper.Success(ctx, "Kode OTP setup 2FA telah dikirim ke email Anda", result)
}

// Enable2FA godoc
// POST /admin/v1/auth/2fa/enable
func (c *AdminAuthController) Enable2FA(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin auth 2fa enable request")

	adminID := middleware.GetAdminID(ctx)

	var req model.AdminEnable2FARequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin auth 2fa enable bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin auth 2fa enable validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}
	req.AdminID = adminID

	if err := c.adminAuthUC.Enable2FA(ctx.Context(), &req); err != nil {
		log.Error("admin auth 2fa enable failed", zap.Error(err))
		return err
	}
	log.Info("admin auth 2fa enable succeeded")

	return helper.Success(ctx, "2FA berhasil diaktifkan", nil)
}

// Register godoc
// POST /admin/v1/auth/register
func (c *AdminAuthController) Register(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin registration request")

	var req model.AdminUserCreateRequest
	if err := ctx.Bind().JSON(&req); err != nil {
		log.Warn("admin registration bind failed", zap.Error(err))
		return helper.BadRequest(ctx, "Format request tidak valid")
	}

	if errs := helper.ValidateStruct(c.validator, req); errs != nil {
		log.Warn("admin registration validation failed", zap.Any("errors", errs))
		return helper.UnprocessableEntity(ctx, errs)
	}

	result, err := c.adminAuthUC.Register(ctx.Context(), &req)
	if err != nil {
		log.Error("admin registration failed", zap.Error(err))
		return err
	}
	log.Info("admin registration succeeded")

	return helper.Created(ctx, "Akun admin berhasil dibuat, silakan verifikasi email Anda", result)
}

// VerifyEmail godoc
// GET /admin/v1/auth/verify-email?token=xxx
func (c *AdminAuthController) VerifyEmail(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("admin verify email request")

	token := ctx.Query("token")
	if token == "" {
		log.Warn("admin verify email missing token")
		return helper.BadRequest(ctx, "Token verifikasi wajib diisi")
	}
	req := &model.AdminVerifyEmailRequest{Token: token}

	if err := c.adminAuthUC.VerifyEmail(ctx.Context(), req); err != nil {
		log.Error("admin verify email failed", zap.Error(err))
		return err
	}
	log.Info("admin verify email succeeded")

	return helper.Success(ctx, "Email berhasil diverifikasi", nil)
}

// GET /admin/v1/auth/me
func (c *AdminAuthController) Me(ctx fiber.Ctx) error {
	adminID := middleware.GetAdminID(ctx)
	result, err := c.adminAuthUC.GetMe(ctx.Context(), adminID)
	if err != nil {
		return err
	}
	return helper.Success(ctx, "Profil admin berhasil diambil", result)
}
