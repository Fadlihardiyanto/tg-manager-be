package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/mailer"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/otp"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// IAdminAuthUseCase handles platform admin authentication logic.
type IAdminAuthUseCase interface {
	// Login validates credentials. If 2FA is enabled, returns Requires2FA=true and TempToken.
	// Otherwise returns Access/Refresh tokens.
	Login(ctx context.Context, req *model.AdminLoginRequest) (*model.AdminLoginResponse, error)

	// Verify2FA validates the OTP code against the TempToken to finalize login.
	Verify2FA(ctx context.Context, req *model.AdminVerify2FARequest) (*model.AdminLoginResponse, error)

	// ResendOTP generates and sends a new OTP code for an existing temp_token session.
	// Respects the OTP cooldown period to prevent abuse.
	ResendOTP(ctx context.Context, req *model.AdminResendOTPRequest) error

	// Setup2FA initiates the 2FA setup process by sending an OTP to the admin's email.
	Setup2FA(ctx context.Context, req *model.AdminSetup2FARequest) (*model.Admin2FASetupResponse, error)

	// Enable2FA verifies the setup OTP and turns on 2FA for the admin.
	Enable2FA(ctx context.Context, req *model.AdminEnable2FARequest) error

	// RefreshToken issues a new access token given a valid refresh token.
	RefreshToken(ctx context.Context, req *model.AdminRefreshTokenRequest) (*model.AdminLoginResponse, error)

	// Logout invalidates the user's tokens.
	Logout(ctx context.Context, req *model.AdminLogoutRequest) error

	// Register creates a new admin account
	// Validates email uniqueness, password strength, and sends verification email
	Register(ctx context.Context, req *model.AdminUserCreateRequest) (*model.AdminUserResponse, error)

	// VerifyEmail confirms the admin's email address
	VerifyEmail(ctx context.Context, req *model.AdminVerifyEmailRequest) error
}

// =============================================================================
// Implementation
// =============================================================================

type AdminAuthUseCase struct {
	db                  *entity.Database
	adminRepo           repository.IAdminUserRepository
	adminPermissionRepo repository.IAdminPermissionRepository
	log                 *zap.Logger
	redis               *redis.Client
	otpService          *otp.EmailOTPService
	mailer              mailer.Sender
	outboxRepo          repository.IOutboxRepository
	jwtConfig           *pkg_jwt.JWTConfig
	frontendURL         string
	bcryptCost          int
}

func NewAdminAuthUseCase(
	db *entity.Database,
	adminRepo repository.IAdminUserRepository,
	adminPermissionRepo repository.IAdminPermissionRepository,
	log *zap.Logger,
	redis *redis.Client,
	otpService *otp.EmailOTPService,
	mailer mailer.Sender,
	outboxRepo repository.IOutboxRepository,
	jwtConfig *pkg_jwt.JWTConfig,
	frontendURL string,
	bcryptCost int,
) IAdminAuthUseCase {
	return &AdminAuthUseCase{
		db:                  db,
		adminRepo:           adminRepo,
		adminPermissionRepo: adminPermissionRepo,
		log:                 log,
		redis:               redis,
		otpService:          otpService,
		mailer:              mailer,
		outboxRepo:          outboxRepo,
		jwtConfig:           jwtConfig,
		frontendURL:         frontendURL,
		bcryptCost:          bcryptCost,
	}
}

func (uc *AdminAuthUseCase) Login(ctx context.Context, req *model.AdminLoginRequest) (*model.AdminLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth login usecase start", zap.String("email", req.Email))

	// 1. Find user by email
	admin, err := uc.adminRepo.FindByEmail(ctx, uc.db.Gorm, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin auth login invalid credentials", zap.String("email", req.Email))
			return nil, helper.NewUnauthorized("invalid email or password")
		}
		log.Error("admin auth login repo lookup failed", zap.Error(err))
		return nil, err
	}

	// 2. Check if locked out
	if admin.LockedUntil != nil && admin.LockedUntil.After(time.Now()) {
		log.Warn("admin auth login blocked by lockout", zap.Time("locked_until", *admin.LockedUntil))
		return nil, helper.NewForbidden(fmt.Sprintf("account is locked until %v", admin.LockedUntil.Format(time.RFC3339)))
	}

	// 3. Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		log.Warn("admin auth login password mismatch", zap.String("email", req.Email))
		uc.adminRepo.IncrementFailedLogin(ctx, uc.db.Gorm, admin.ID)
		admin, err := uc.adminRepo.FindByEmail(ctx, uc.db.Gorm, req.Email)
		if err == nil && admin.FailedLoginCount >= 5 {
			uc.adminRepo.LockAccount(ctx, uc.db.Gorm, admin.ID, time.Now().Add(15*time.Minute))
		}
		return nil, helper.NewUnauthorized("invalid email or password")
	}

	// 4. Handle 2FA if enabled
	if admin.IsTwoFAEnabled {
		log.Info("admin auth login requires 2fa", zap.String("admin_id", admin.ID.String()))
		// Generate OTP
		code, err := uc.otpService.GenerateOTP(ctx, "admin_login_2fa", admin.ID.String())
		if err != nil {
			log.Error("admin auth login otp generation failed", zap.Error(err))
			return nil, err
		}

		// Send email
		if err := uc.mailer.SendOTP(admin.Email, code, "admin_login_2fa"); err != nil {
			log.Error("admin auth login otp email failed", zap.Error(err))
			return nil, fmt.Errorf("failed to send 2FA email: %w", err)
		}

		// Generate TempToken (random UUID stored in Redis mapping to admin ID)
		tempToken := uuid.New().String()
		redisKey := fmt.Sprintf("auth:temp_token:%s", tempToken)
		uc.redis.Set(ctx, redisKey, admin.ID.String(), 5*time.Minute)
		log.Info("admin auth login temp token issued", zap.String("admin_id", admin.ID.String()))

		return &model.AdminLoginResponse{
			Requires2FA: true,
			TempToken:   tempToken,
			User:        nil,
		}, nil
	}

	// 5. Normal login (No 2FA)
	log.Info("admin auth login finalizing", zap.String("admin_id", admin.ID.String()))
	return uc.finalizeLogin(ctx, admin.ID, req.ClientIP)
}

func (uc *AdminAuthUseCase) Verify2FA(ctx context.Context, req *model.AdminVerify2FARequest) (*model.AdminLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth verify 2fa start")

	// 1. Validate TempToken
	redisKey := fmt.Sprintf("auth:temp_token:%s", req.TempToken)
	adminIDStr, err := uc.redis.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		log.Warn("admin auth verify 2fa temp token expired")
		return nil, helper.NewUnauthorized("invalid or expired temporary token")
	} else if err != nil {
		log.Error("admin auth verify 2fa redis lookup failed", zap.Error(err))
		return nil, err
	}

	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return nil, err
	}

	// 2. Verify OTP code
	if err := uc.otpService.VerifyOTP(ctx, "admin_login_2fa", adminID.String(), req.OTPCode); err != nil {
		log.Warn("admin auth verify 2fa otp invalid", zap.String("admin_id", adminID.String()))
		return nil, err
	}

	// 3. Clear TempToken
	uc.redis.Del(ctx, redisKey)
	log.Info("admin auth verify 2fa success", zap.String("admin_id", adminID.String()))

	// 4. Finalize login
	return uc.finalizeLogin(ctx, adminID, req.ClientIP)
}

func (uc *AdminAuthUseCase) ResendOTP(ctx context.Context, req *model.AdminResendOTPRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth resend otp start")

	// 1. Validate TempToken — ensure the login session is still alive
	redisKey := fmt.Sprintf("auth:temp_token:%s", req.TempToken)
	adminIDStr, err := uc.redis.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		log.Warn("admin auth resend otp temp token expired")
		return helper.NewUnauthorized("sesi login sudah kedaluwarsa, silakan login ulang")
	} else if err != nil {
		log.Error("admin auth resend otp redis lookup failed", zap.Error(err))
		return err
	}

	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return err
	}

	// 2. Fetch admin email
	admin, err := uc.adminRepo.FindByIDWithRoles(ctx, uc.db.Gorm, adminID)
	if err != nil {
		log.Error("admin auth resend otp load admin failed", zap.Error(err))
		return err
	}

	// 3. Generate new OTP (respects cooldown internally)
	code, err := uc.otpService.GenerateOTP(ctx, "admin_login_2fa", adminID.String())
	if err != nil {
		log.Warn("admin auth resend otp cooldown active or generation failed", zap.Error(err))
		return err
	}

	// 4. Send email
	if err := uc.mailer.SendOTP(admin.Email, code, "admin_login_2fa"); err != nil {
		log.Error("admin auth resend otp email failed", zap.Error(err))
		return fmt.Errorf("gagal mengirim ulang kode OTP: %w", err)
	}

	log.Info("admin auth resend otp success", zap.String("admin_id", adminID.String()))
	return nil
}

func (uc *AdminAuthUseCase) Setup2FA(ctx context.Context, req *model.AdminSetup2FARequest) (*model.Admin2FASetupResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth 2fa setup start", zap.String("admin_id", req.AdminID.String()))

	adminID := req.AdminID

	// Find admin to get email
	admin, err := uc.adminRepo.FindByIDWithRoles(ctx, uc.db.Gorm, adminID)
	if err != nil {
		log.Error("admin auth 2fa setup load admin failed", zap.Error(err))
		return nil, err
	}

	if admin.IsTwoFAEnabled {
		log.Warn("admin auth 2fa already enabled", zap.String("admin_id", adminID.String()))
		return nil, helper.NewConflict("2FA is already enabled")
	}

	// Generate OTP
	code, err := uc.otpService.GenerateOTP(ctx, "admin_2fa_setup", adminID.String())
	if err != nil {
		log.Error("admin auth 2fa setup otp generation failed", zap.Error(err))
		return nil, err
	}

	// Send email
	if err := uc.mailer.SendOTP(admin.Email, code, "admin_2fa_setup"); err != nil {
		log.Error("admin auth 2fa setup email failed", zap.Error(err))
		return nil, fmt.Errorf("failed to send setup email: %w", err)
	}
	log.Info("admin auth 2fa setup otp sent", zap.String("admin_id", adminID.String()))

	return &model.Admin2FASetupResponse{
		Message: "Kode OTP telah dikirim ke email Anda",
	}, nil
}

func (uc *AdminAuthUseCase) Enable2FA(ctx context.Context, req *model.AdminEnable2FARequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth 2fa enable start", zap.String("admin_id", req.AdminID.String()))

	// Verify OTP
	if err := uc.otpService.VerifyOTP(ctx, "admin_2fa_setup", req.AdminID.String(), req.OTPCode); err != nil {
		log.Warn("admin auth 2fa enable otp invalid", zap.String("admin_id", req.AdminID.String()))
		return err
	}

	// Update DB
	if err := uc.adminRepo.Update2FAStatus(ctx, uc.db.Gorm, req.AdminID, true); err != nil {
		log.Error("admin auth 2fa enable update failed", zap.Error(err))
		return err
	}
	log.Info("admin auth 2fa enabled", zap.String("admin_id", req.AdminID.String()))
	return nil
}

func (uc *AdminAuthUseCase) RefreshToken(ctx context.Context, req *model.AdminRefreshTokenRequest) (*model.AdminLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth refresh token start")

	// Parse refresh token (uses separate parser that handles jwt.RegisteredClaims)
	claims, err := pkg_jwt.ParseAdminRefreshToken(req.RefreshToken, uc.jwtConfig.AdminSecretKey)
	if err != nil {
		log.Warn("admin auth refresh token invalid")
		return nil, helper.NewUnauthorized("invalid refresh token")
	}

	// Ensure it's a refresh token
	isRefresh := false
	for _, aud := range claims.Audience {
		if aud == "refresh" {
			isRefresh = true
			break
		}
	}
	if !isRefresh {
		log.Warn("admin auth refresh token wrong audience")
		return nil, helper.NewUnauthorized("not a refresh token")
	}

	// Check if blacklisted
	isBlacklisted, err := uc.redis.Exists(ctx, "auth:blacklist:"+claims.ID).Result()
	if err == nil && isBlacklisted > 0 {
		log.Warn("admin auth refresh token blacklisted")
		return nil, helper.NewUnauthorized("token is blacklisted")
	}

	log.Info("admin auth refresh token finalizing", zap.String("admin_id", claims.AdminID.String()))
	return uc.finalizeLogin(ctx, claims.AdminID, req.ClientIP)
}

func (uc *AdminAuthUseCase) Logout(ctx context.Context, req *model.AdminLogoutRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth logout start")

	claims, err := pkg_jwt.ParseAdminToken(req.AccessToken, uc.jwtConfig.AdminSecretKey)
	if err != nil {
		log.Warn("admin auth logout parse token failed", zap.Error(err))
		return err
	}

	// Blacklist the JTI
	expiration := time.Until(claims.ExpiresAt.Time)
	if expiration > 0 {
		uc.redis.Set(ctx, "auth:blacklist:"+claims.ID, "logged_out", expiration)
	}
	log.Info("admin auth logout success", zap.String("admin_id", req.AdminID.String()))

	return nil
}

// finalizeLogin handles the common steps after successful authentication
func (uc *AdminAuthUseCase) finalizeLogin(ctx context.Context, adminID uuid.UUID, ip string) (*model.AdminLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth finalize login start", zap.String("admin_id", adminID.String()))

	// 1. Fetch user to ensure they exist and get email
	admin, err := uc.adminRepo.FindByIDWithRoles(ctx, uc.db.Gorm, adminID)
	if err != nil {
		log.Error("admin auth finalize login load admin failed", zap.Error(err))
		return nil, err
	}

	// 2. Reset failed attempts & update login metadata
	uc.adminRepo.ResetFailedLogin(ctx, uc.db.Gorm, adminID)
	uc.adminRepo.UpdateLoginInfo(ctx, uc.db.Gorm, adminID, ip, time.Now())

	// 3. Fetch flat permissions
	permissions, err := uc.adminPermissionRepo.FindPermissionNamesByAdminUserID(ctx, uc.db.Gorm, adminID)
	if err != nil {
		log.Error("admin auth finalize login permissions failed", zap.Error(err))
		return nil, err
	}

	// Extract role names for JWT claims
	roles := []string{}
	for _, role := range admin.Roles {
		roles = append(roles, role.Name)
	}

	// 4. Generate Tokens
	accessToken, refreshToken, _, err := pkg_jwt.GenerateAdminTokens(ctx, adminID, admin.Email, roles, permissions, uc.jwtConfig)
	if err != nil {
		log.Error("admin auth finalize login token generation failed", zap.Error(err))
		return nil, err
	}

	// Build enriched response including user and its roles via converter
	resp := converter.ToAdminLoginResponse(admin, accessToken, refreshToken, int64(uc.jwtConfig.AdminAccessExpiry.Seconds()))
	log.Info("admin auth finalize login success", zap.String("admin_id", adminID.String()))
	return &resp, nil
}

// Register creates a new admin account with validation
func (uc *AdminAuthUseCase) Register(ctx context.Context, req *model.AdminUserCreateRequest) (*model.AdminUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth register start", zap.String("email", req.Email))

	// 2. Check if email already exists
	exists, err := uc.adminRepo.EmailExists(ctx, uc.db.Gorm, req.Email)
	if err != nil {
		log.Error("admin auth register email check failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		log.Warn("admin auth register duplicate email", zap.String("email", req.Email))
		return nil, helper.NewConflict("email already exists")
	}

	// 3. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), uc.bcryptCost)
	if err != nil {
		log.Error("admin auth register hash password failed", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 4. Create admin user entity
	adminID := uuid.New()
	verificationToken := uuid.New().String()

	admin := &entity.AdminUser{
		ID:               adminID,
		Email:            req.Email,
		Name:             req.Name,
		PasswordHash:     string(hashedPassword),
		IsActive:         false, // Inactive until email verified
		IsTwoFAEnabled:   false,
		FailedLoginCount: 0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// 5. Save to database using transaction
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		if err := uc.adminRepo.Create(ctx, tx, admin); err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		// Prepare Email Verification Outbox
		verificationLink := fmt.Sprintf("%s/verify-email?token=%s", uc.frontendURL, verificationToken)

		payload := model.EmailNotificationPayload{
			Type:             model.EmailNotificationAdminVerification,
			To:               admin.Email,
			Name:             admin.Name,
			VerificationLink: verificationLink,
		}

		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal outbox payload: %w", marshalErr)
		}

		outbox := &entity.Outbox{
			ID:            uuid.New(),
			AggregateType: "admin_user",
			AggregateID:   adminID,
			EventType:     "notification.send",
			Payload:       payloadBytes,
			Status:        "pending",
			RetryCount:    0,
			MaxRetries:    3,
			ProcessAfter:  time.Now(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := uc.outboxRepo.Create(ctx, tx, outbox); err != nil {
			return fmt.Errorf("failed to create outbox event: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Error("admin auth register transaction failed", zap.Error(err))
		return nil, err
	}

	// 6. Generate verification token in Redis
	verificationKey := fmt.Sprintf("admin:verify:%s", verificationToken)
	if err := uc.redis.Set(ctx, verificationKey, adminID.String(), 24*time.Hour).Err(); err != nil {
		log.Error("admin auth register store verification token failed", zap.Error(err))
		// Log error but don't fail registration since outbox is already created
	}

	// 7. Return response
	log.Info("admin auth register success", zap.String("admin_id", adminID.String()))
	return &model.AdminUserResponse{
		ID:               admin.ID,
		Email:            admin.Email,
		Name:             admin.Name,
		IsActive:         admin.IsActive,
		IsTwoFAEnabled:   admin.IsTwoFAEnabled,
		FailedLoginCount: admin.FailedLoginCount,
		CreatedAt:        admin.CreatedAt,
		UpdatedAt:        admin.UpdatedAt,
	}, nil
}

// VerifyEmail confirms the admin's email address and activates the account
func (uc *AdminAuthUseCase) VerifyEmail(ctx context.Context, req *model.AdminVerifyEmailRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin auth verify email start")

	token := req.Token

	// 1. Validate token
	if token == "" {
		log.Warn("admin auth verify email missing token")
		return helper.NewBadRequest("verification token is required")
	}

	// 2. Retrieve admin ID from Redis
	verificationKey := fmt.Sprintf("admin:verify:%s", token)
	adminIDStr, err := uc.redis.Get(ctx, verificationKey).Result()
	if err != nil {
		if err == redis.Nil {
			log.Warn("admin auth verify email token expired")
			return helper.NewBadRequest("verification token is invalid or expired")
		}
		log.Error("admin auth verify email redis lookup failed", zap.Error(err))
		return fmt.Errorf("failed to retrieve verification token: %w", err)
	}

	// 3. Parse admin ID
	adminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		log.Error("admin auth verify email parse admin id failed", zap.Error(err))
		return helper.NewBadRequest("invalid admin ID in token")
	}

	// 4. Fetch admin user
	admin := &entity.AdminUser{}
	if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, admin, adminID); err != nil {
		log.Error("admin auth verify email load admin failed", zap.Error(err))
		return helper.NewNotFound("admin user")
	}

	// 5. Update admin to active
	admin.IsActive = true
	admin.UpdatedAt = time.Now()
	if err := uc.adminRepo.Update(ctx, uc.db.Gorm, admin); err != nil {
		log.Error("admin auth verify email update failed", zap.Error(err))
		return fmt.Errorf("failed to activate admin user: %w", err)
	}

	// 6. Delete verification token from Redis
	if err := uc.redis.Del(ctx, verificationKey).Err(); err != nil {
		// Log error but don't fail the operation
		log.Warn("admin auth verify email delete token failed", zap.Error(err))
	}
	log.Info("admin auth verify email success", zap.String("admin_id", adminID.String()))

	return nil
}
