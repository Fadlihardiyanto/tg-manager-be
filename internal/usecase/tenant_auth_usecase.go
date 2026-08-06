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
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ITenantAuthUseCase handles authentication and registration for business tenants.
type ITenantAuthUseCase interface {
	Register(ctx context.Context, req *model.TenantRegisterRequest) (*model.TenantRegisterResponse, error)
	Login(ctx context.Context, req *model.TenantLoginRequest) (*model.TenantLoginResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*model.TenantLoginResponse, error)
	VerifyEmail(ctx context.Context, req *model.TenantVerifyEmailRequest) (*model.TenantLoginResponse, error)
	ResendVerification(ctx context.Context, req *model.TenantResendVerificationRequest) error
	Onboarding(ctx context.Context, userID uuid.UUID, req *model.TenantOnboardingRequest) (*model.TenantOnboardingResponse, error)
	GetProfile(ctx context.Context, userID uuid.UUID, clientID uuid.UUID) (*model.TenantMeResponse, error)
}

type TenantAuthUseCase struct {
	db             *entity.Database
	userRepo       repository.IUserRepository
	clientRepo     repository.IClientRepository
	clientUserRepo repository.IClientUserRepository
	permissionRepo repository.ITenantPermissionRepository
	outboxRepo     repository.IOutboxRepository
	log            *zap.Logger
	redis          *redis.Client
	jwtConfig      *pkg_jwt.JWTConfig
	frontendURL    string
	bcryptCost     int
}

func NewTenantAuthUseCase(
	db *entity.Database,
	userRepo repository.IUserRepository,
	clientRepo repository.IClientRepository,
	clientUserRepo repository.IClientUserRepository,
	permissionRepo repository.ITenantPermissionRepository,
	outboxRepo repository.IOutboxRepository,
	log *zap.Logger,
	redis *redis.Client,
	jwtConfig *pkg_jwt.JWTConfig,
	frontendURL string,
	bcryptCost int,
) ITenantAuthUseCase {
	return &TenantAuthUseCase{
		db:             db,
		userRepo:       userRepo,
		clientRepo:     clientRepo,
		clientUserRepo: clientUserRepo,
		permissionRepo: permissionRepo,
		outboxRepo:     outboxRepo,
		log:            log,
		redis:          redis,
		jwtConfig:      jwtConfig,
		frontendURL:    frontendURL,
		bcryptCost:     bcryptCost,
	}
}

func (uc *TenantAuthUseCase) Register(ctx context.Context, req *model.TenantRegisterRequest) (*model.TenantRegisterResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth register start", zap.String("email", req.Email))

	// 1. Check if email exists
	emailExists, err := uc.userRepo.EmailExists(ctx, uc.db.Gorm, req.Email)
	if err != nil {
		log.Error("tenant auth register email check failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if emailExists {
		log.Warn("tenant auth register duplicate email", zap.String("email", req.Email))
		return nil, helper.NewConflict("Email sudah terdaftar")
	}

	// 2. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), uc.bcryptCost)
	if err != nil {
		log.Error("tenant auth register hash password failed", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var registeredUser *entity.User
	var verificationToken string

	// 3. Begin transaction — create user + email verification outbox
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		// Create User
		user := &entity.User{
			ID:              uuid.New(),
			Email:           req.Email,
			Name:            req.Name,
			PasswordHash:    string(hashedPassword),
			Phone:           req.Phone,
			IsEmailVerified: false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := uc.userRepo.Create(ctx, tx, user); err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		// Prepare Email Verification Outbox
		verificationToken = uuid.New().String()
		verificationLink := fmt.Sprintf("%s/verify-email?token=%s", uc.frontendURL, verificationToken)

		payload := model.EmailNotificationPayload{
			Type:             model.EmailNotificationTenantVerification,
			To:               user.Email,
			Name:             user.Name,
			VerificationLink: verificationLink,
		}
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			log.Warn("tenant auth: failed to marshal register outbox payload", zap.Error(marshalErr))
		}

		outbox := &entity.Outbox{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   user.ID,
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
			return fmt.Errorf("create outbox event: %w", err)
		}

		registeredUser = user
		return nil
	})

	if err != nil {
		log.Error("tenant auth register transaction failed", zap.Error(err))
		return nil, err
	}

	// 4. Save verification token to Redis (Expiry 24 hours)
	tokenKey := fmt.Sprintf("auth:verify_email:%s", verificationToken)
	if err := uc.redis.Set(ctx, tokenKey, registeredUser.ID.String(), 24*time.Hour).Err(); err != nil {
		log.Error("tenant auth register failed to save verification token to redis", zap.Error(err))
		// We don't fail the registration if redis fails here, but user might need to request a new email
	}

	log.Info("tenant auth register success", zap.String("user_id", registeredUser.ID.String()))

	return &model.TenantRegisterResponse{
		User: *converter.UserToResponse(registeredUser),
	}, nil
}

func (uc *TenantAuthUseCase) Login(ctx context.Context, req *model.TenantLoginRequest) (*model.TenantLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth login start", zap.String("email", req.Email))

	// Rate limiting: max 5 attempts per email per minute
	rateLimitKey := fmt.Sprintf("auth:rate_limit:tenant_login:%s", req.Email)
	count, err := uc.redis.Incr(ctx, rateLimitKey).Result()
	if err == nil && count == 1 {
		uc.redis.Expire(ctx, rateLimitKey, 1*time.Minute)
	}
	if count > 5 {
		log.Warn("tenant auth login rate limited", zap.String("email", req.Email))
		return nil, helper.NewTooManyRequestsError("Terlalu banyak percobaan login. Silakan coba lagi nanti.")
	}

	// 1. Find user by email and preload client data
	user, err := uc.userRepo.FindByEmailWithClient(ctx, uc.db.Gorm, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("tenant auth login invalid credentials", zap.String("email", req.Email))
			return nil, helper.NewUnauthorized("Email atau password salah")
		}
		log.Error("tenant auth login repo lookup failed", zap.Error(err))
		return nil, err
	}

	// 2. Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Warn("tenant auth login password mismatch", zap.String("email", req.Email))
		return nil, helper.NewUnauthorized("Email atau password salah")
	}

	// 3. Check if email is verified
	if !user.IsEmailVerified {
		log.Warn("tenant auth login unverified email", zap.String("email", req.Email))
		return nil, helper.NewForbidden("Email belum diverifikasi. Silakan cek email Anda atau minta kirim ulang verifikasi.")
	}

	// 4. Update Last Login
	now := time.Now()
	user.LastLoginAt = &now
	if err := uc.userRepo.Update(ctx, uc.db.Gorm, user); err != nil {
		log.Error("tenant auth login failed to update last login", zap.String("user_id", user.ID.String()), zap.Error(err))
	}

	// 4. Check if user is associated with an active client
	if len(user.ClientUsers) == 0 {
		log.Info("tenant auth login user has no client, needs onboarding")

		// Generate token without client context — user can only access onboarding endpoint
		accessToken, refreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, user.ID, uuid.Nil, "", nil, uc.jwtConfig)
		if err != nil {
			log.Error("tenant auth login token generation failed (no client)", zap.Error(err))
			return nil, err
		}

		return &model.TenantLoginResponse{
			AccessToken:     accessToken,
			RefreshToken:    refreshToken,
			ExpiresIn:       int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
			User:            *converter.UserToResponse(user),
			Client:          nil,
			Role:            "",
			NeedsOnboarding: true,
		}, nil
	}

	clientUser := user.ClientUsers[0]
	client := clientUser.Client

	if !client.IsActive {
		log.Warn("tenant auth login client inactive", zap.String("client_id", client.ID.String()))
		return nil, helper.NewForbidden("Tenant bisnis Anda sedang tidak aktif, silakan hubungi admin")
	}

	// 5. Fetch permissions for the role
	permissions, err := uc.permissionRepo.FindPermissionNamesByRole(ctx, uc.db.Gorm, clientUser.Role)
	if err != nil {
		log.Error("tenant auth login fetch permissions failed", zap.Error(err))
		return nil, err
	}

	// 6. Generate Tokens (with permissions embedded)
	accessToken, refreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, user.ID, client.ID, clientUser.Role, permissions, uc.jwtConfig)
	if err != nil {
		log.Error("tenant auth login token generation failed", zap.Error(err))
		return nil, err
	}

	clientResp := converter.ClientToResponse(&client)
	log.Info("tenant auth login success", zap.String("user_id", user.ID.String()), zap.String("client_id", client.ID.String()))

	return &model.TenantLoginResponse{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
		User:            *converter.UserToResponse(user),
		Client:          clientResp,
		Role:            clientUser.Role,
		NeedsOnboarding: false,
	}, nil
}

func (uc *TenantAuthUseCase) Refresh(ctx context.Context, refreshToken string) (*model.TenantLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth refresh request")

	// 1. Parse token (uses separate parser that handles jwt.RegisteredClaims)
	claims, err := pkg_jwt.ParseTenantRefreshToken(refreshToken, uc.jwtConfig.TenantSecretKey)
	if err != nil {
		log.Warn("tenant auth refresh invalid token", zap.Error(err))
		return nil, helper.NewUnauthorized("Sesi tidak valid atau telah kedaluwarsa")
	}

	// Ensure it's a refresh token by checking audience
	isRefresh := false
	for _, aud := range claims.Audience {
		if aud == "refresh" {
			isRefresh = true
			break
		}
	}
	if !isRefresh {
		log.Warn("tenant auth refresh token used invalid audience")
		return nil, helper.NewUnauthorized("Token tidak valid untuk operasi ini")
	}

	// Check if blacklisted (JTI revocation)
	isBlacklisted, err := uc.redis.Exists(ctx, "auth:blacklist:"+claims.ID).Result()
	if err == nil && isBlacklisted > 0 {
		log.Warn("tenant auth refresh token blacklisted")
		return nil, helper.NewUnauthorized("Token telah dibatalkan")
	}

	// 2. Find user
	user, err := uc.userRepo.FindByID(ctx, uc.db.Gorm, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewUnauthorized("User tidak ditemukan")
		}
		return nil, err
	}

	if !user.IsEmailVerified {
		return nil, helper.NewForbidden("Email belum diverifikasi")
	}

	// 3. Handle client logic
	if claims.ClientID == uuid.Nil {
		// Needs onboarding
		newAccessToken, newRefreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, user.ID, uuid.Nil, "", nil, uc.jwtConfig)
		if err != nil {
			return nil, err
		}

		return &model.TenantLoginResponse{
			AccessToken:     newAccessToken,
			RefreshToken:    newRefreshToken,
			ExpiresIn:       int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
			User:            *converter.UserToResponse(user),
			Client:          nil,
			Role:            "",
			NeedsOnboarding: true,
		}, nil
	}

	// Fetch client and role
	clientUser, err := uc.clientUserRepo.FindByClientAndUserID(ctx, uc.db.Gorm, claims.ClientID, user.ID)
	if err != nil || clientUser == nil {
		return nil, helper.NewUnauthorized("Akses tenant ditolak")
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, claims.ClientID)
	if err != nil || client == nil || !client.IsActive {
		return nil, helper.NewForbidden("Tenant bisnis Anda sedang tidak aktif")
	}

	// Fetch permissions
	permissions, err := uc.permissionRepo.FindPermissionNamesByRole(ctx, uc.db.Gorm, clientUser.Role)
	if err != nil {
		return nil, err
	}

	// Generate new tokens
	newAccessToken, newRefreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, user.ID, client.ID, clientUser.Role, permissions, uc.jwtConfig)
	if err != nil {
		return nil, err
	}

	log.Info("tenant auth refresh success", zap.String("user_id", user.ID.String()))

	return &model.TenantLoginResponse{
		AccessToken:     newAccessToken,
		RefreshToken:    newRefreshToken,
		ExpiresIn:       int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
		User:            *converter.UserToResponse(user),
		Client:          converter.ClientToResponse(client),
		Role:            clientUser.Role,
		NeedsOnboarding: false,
	}, nil
}

func (uc *TenantAuthUseCase) VerifyEmail(ctx context.Context, req *model.TenantVerifyEmailRequest) (*model.TenantLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth verify email start")

	tokenKey := fmt.Sprintf("auth:verify_email:%s", req.Token)

	// 1. Get User ID from Redis
	userIDStr, err := uc.redis.Get(ctx, tokenKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			log.Warn("tenant auth verify email token not found or expired")
			return nil, helper.NewBadRequest("Token verifikasi tidak valid atau sudah kadaluarsa")
		}
		log.Error("tenant auth verify email redis get failed", zap.Error(err))
		return nil, err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Error("tenant auth verify email parse uuid failed", zap.Error(err))
		return nil, err
	}

	// 2. Begin transaction to update user and mark as verified
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		user, err := uc.userRepo.FindByID(ctx, tx, userID)
		if err != nil {
			return err
		}

		if user.IsEmailVerified {
			return nil // Already verified
		}

		user.IsEmailVerified = true
		user.UpdatedAt = time.Now()

		if err := uc.userRepo.Update(ctx, tx, user); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Error("tenant auth verify email transaction failed", zap.Error(err))
		return nil, err
	}

	// 3. Delete token from Redis
	if err := uc.redis.Del(ctx, tokenKey).Err(); err != nil {
		log.Warn("tenant auth verify email failed to delete token from redis", zap.Error(err))
	}

	// 4. Auto-login: fetch user with client data and generate JWT
	userBasic, err := uc.userRepo.FindByID(ctx, uc.db.Gorm, userID)
	if err != nil {
		log.Error("tenant auth verify email fetch user failed", zap.Error(err))
		return nil, err
	}

	user, err := uc.userRepo.FindByEmailWithClient(ctx, uc.db.Gorm, userBasic.Email)
	if err != nil {
		log.Error("tenant auth verify email fetch user with client failed", zap.Error(err))
		return nil, err
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	if err := uc.userRepo.Update(ctx, uc.db.Gorm, user); err != nil {
		log.Error("tenant auth verify email failed to update last login", zap.String("user_id", user.ID.String()), zap.Error(err))
	}

	// 5. Generate JWT — same logic as Login
	if len(user.ClientUsers) == 0 {
		// User has no client yet — needs onboarding
		accessToken, refreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, user.ID, uuid.Nil, "", nil, uc.jwtConfig)
		if err != nil {
			log.Error("tenant auth verify email token generation failed (no client)", zap.Error(err))
			return nil, err
		}

		log.Info("tenant auth verify email success + auto-login (needs onboarding)", zap.String("user_id", userID.String()))

		return &model.TenantLoginResponse{
			AccessToken:     accessToken,
			RefreshToken:    refreshToken,
			ExpiresIn:       int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
			User:            *converter.UserToResponse(user),
			Client:          nil,
			Role:            "",
			NeedsOnboarding: true,
		}, nil
	}

	clientUser := user.ClientUsers[0]
	client := clientUser.Client

	permissions, err := uc.permissionRepo.FindPermissionNamesByRole(ctx, uc.db.Gorm, clientUser.Role)
	if err != nil {
		log.Error("tenant auth verify email fetch permissions failed", zap.Error(err))
		return nil, err
	}

	accessToken, refreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, user.ID, client.ID, clientUser.Role, permissions, uc.jwtConfig)
	if err != nil {
		log.Error("tenant auth verify email token generation failed", zap.Error(err))
		return nil, err
	}

	clientResp := converter.ClientToResponse(&client)
	log.Info("tenant auth verify email success + auto-login", zap.String("user_id", userID.String()))

	return &model.TenantLoginResponse{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
		User:            *converter.UserToResponse(user),
		Client:          clientResp,
		Role:            clientUser.Role,
		NeedsOnboarding: false,
	}, nil
}

func (uc *TenantAuthUseCase) ResendVerification(ctx context.Context, req *model.TenantResendVerificationRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth resend verification start", zap.String("email", req.Email))

	// 1. Find user by email
	user, err := uc.userRepo.FindByEmail(ctx, uc.db.Gorm, req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("tenant auth resend verification user not found", zap.String("email", req.Email))
			// Return nil to prevent email enumeration
			return nil
		}
		log.Error("tenant auth resend verification repo lookup failed", zap.Error(err))
		return err
	}

	// 2. Check if already verified
	if user.IsEmailVerified {
		log.Warn("tenant auth resend verification email already verified", zap.String("email", req.Email))
		return helper.NewBadRequest("Email sudah diverifikasi")
	}

	// 3. Generate new verification token
	verificationToken := uuid.New().String()
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", uc.frontendURL, verificationToken)

	// 4. Begin transaction — create outbox event
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		payload := model.EmailNotificationPayload{
			Type:             model.EmailNotificationTenantVerification,
			To:               user.Email,
			Name:             user.Name,
			VerificationLink: verificationLink,
		}
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			log.Warn("tenant auth: failed to marshal resend verification outbox payload", zap.Error(marshalErr))
		}

		outbox := &entity.Outbox{
			ID:            uuid.New(),
			AggregateType: "user",
			AggregateID:   user.ID,
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
			return fmt.Errorf("create outbox event: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Error("tenant auth resend verification transaction failed", zap.Error(err))
		return err
	}

	// 5. Save verification token to Redis (Expiry 24 hours)
	tokenKey := fmt.Sprintf("auth:verify_email:%s", verificationToken)
	if err := uc.redis.Set(ctx, tokenKey, user.ID.String(), 24*time.Hour).Err(); err != nil {
		log.Error("tenant auth resend verification failed to save token to redis", zap.Error(err))
		// Don't fail if redis fails — user can request again
	}

	log.Info("tenant auth resend verification success", zap.String("user_id", user.ID.String()))
	return nil
}

// Onboarding creates a new business (client) for an authenticated user who hasn't set up their tenant yet.
// It creates a Client + ClientUser in a single transaction, then returns a new JWT with client context.
func (uc *TenantAuthUseCase) Onboarding(ctx context.Context, userID uuid.UUID, req *model.TenantOnboardingRequest) (*model.TenantOnboardingResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant onboarding start", zap.String("user_id", userID.String()), zap.String("slug", req.BusinessSlug))

	// 1. Fetch user
	user, err := uc.userRepo.FindByID(ctx, uc.db.Gorm, userID)
	if err != nil {
		log.Error("tenant onboarding user not found", zap.Error(err))
		return nil, helper.NewNotFound("User tidak ditemukan")
	}

	// 2. Check if user already has an active client
	existingUser, err := uc.userRepo.FindByEmailWithClient(ctx, uc.db.Gorm, user.Email)
	if err != nil {
		log.Error("tenant onboarding check existing client failed", zap.Error(err))
		return nil, err
	}
	if len(existingUser.ClientUsers) > 0 {
		log.Warn("tenant onboarding user already has a client")
		return nil, helper.NewConflict("Anda sudah memiliki bisnis terdaftar")
	}

	// 3. Check if business slug is available
	clientBySlug, err := uc.clientRepo.FindBySlug(ctx, uc.db.Gorm, req.BusinessSlug)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("tenant onboarding slug check failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check business slug: %w", err)
	}
	if clientBySlug != nil {
		log.Warn("tenant onboarding duplicate slug", zap.String("slug", req.BusinessSlug))
		return nil, helper.NewConflict("Business slug sudah digunakan")
	}

	var createdClient *entity.Client

	// 4. Begin transaction — create client + client_user
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		// Create Client (Business)
		client := &entity.Client{
			ID:               uuid.New(),
			Name:             req.BusinessName,
			Slug:             req.BusinessSlug,
			Category:         req.Category,
			OwnerUserID:      userID,
			SubscriptionTier: "free",
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := uc.clientRepo.Create(ctx, tx, client); err != nil {
			return fmt.Errorf("create client: %w", err)
		}

		// Create ClientUser (Assign role 'owner')
		now := time.Now()
		clientUser := &entity.ClientUser{
			ID:         uuid.New(),
			ClientID:   client.ID,
			UserID:     userID,
			Role:       "owner",
			IsActive:   true,
			AcceptedAt: &now,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := uc.clientUserRepo.Create(ctx, tx, clientUser); err != nil {
			return fmt.Errorf("create client user: %w", err)
		}

		createdClient = client
		return nil
	})

	if err != nil {
		log.Error("tenant onboarding transaction failed", zap.Error(err))
		return nil, err
	}

	// 5. Fetch permissions for owner role
	permissions, err := uc.permissionRepo.FindPermissionNamesByRole(ctx, uc.db.Gorm, "owner")
	if err != nil {
		log.Error("tenant onboarding fetch permissions failed", zap.Error(err))
		return nil, err
	}

	// 6. Generate new JWT with client context
	accessToken, refreshToken, _, err := pkg_jwt.GenerateTenantTokens(ctx, userID, createdClient.ID, "owner", permissions, uc.jwtConfig)
	if err != nil {
		log.Error("tenant onboarding token generation failed", zap.Error(err))
		return nil, err
	}

	log.Info("tenant onboarding completed successfully", zap.String("client_id", createdClient.ID.String()))

	return &model.TenantOnboardingResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(uc.jwtConfig.TenantAccessExpiry.Seconds()),
		User:         *converter.UserToResponse(user),
		Client:       *converter.ClientToResponse(createdClient),
		Role:         "owner",
	}, nil
}

func (uc *TenantAuthUseCase) GetProfile(ctx context.Context, userID uuid.UUID, clientID uuid.UUID) (*model.TenantMeResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth get profile start", zap.String("user_id", userID.String()))

	user, err := uc.userRepo.FindByID(ctx, uc.db.Gorm, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("User tidak ditemukan")
		}
		log.Error("failed to find user", zap.Error(err))
		return nil, err
	}

	// Fetch client associated with user
	var clientResp *model.ClientResponse
	var role string
	var permissions []string
	needsOnboarding := true

	if clientID != uuid.Nil {
		// Verify client ownership/access
		clientUser, err := uc.clientUserRepo.FindByClientAndUserID(ctx, uc.db.Gorm, clientID, userID)
		if err == nil && clientUser != nil {
			client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
			if err == nil && client != nil {
				clientResp = converter.ClientToResponse(client)
				role = clientUser.Role
				needsOnboarding = false

				// Fetch permissions
				perms, err := uc.permissionRepo.FindPermissionNamesByRole(ctx, uc.db.Gorm, role)
				if err == nil {
					permissions = perms
				} else {
					log.Warn("failed to fetch permissions for role", zap.String("role", role), zap.Error(err))
				}
			}
		} else {
			log.Warn("client user not found for get profile", zap.Error(err))
		}
	} else {
		// Check if user has any client
		userWithClient, err := uc.userRepo.FindByEmailWithClient(ctx, uc.db.Gorm, user.Email)
		if err == nil && len(userWithClient.ClientUsers) > 0 {
			needsOnboarding = false
		}
	}

	return &model.TenantMeResponse{
		User:            *converter.UserToResponse(user),
		Client:          clientResp,
		Role:            role,
		Permissions:     permissions,
		NeedsOnboarding: needsOnboarding,
	}, nil
}
