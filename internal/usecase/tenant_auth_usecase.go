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
	VerifyEmail(ctx context.Context, req *model.TenantVerifyEmailRequest) error
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
	}
}

func (uc *TenantAuthUseCase) Register(ctx context.Context, req *model.TenantRegisterRequest) (*model.TenantRegisterResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth register start", zap.String("email", req.Email), zap.String("business_slug", req.BusinessSlug))

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

	// 2. Check if business slug exists
	clientBySlug, err := uc.clientRepo.FindBySlug(ctx, uc.db.Gorm, req.BusinessSlug)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("tenant auth register slug check failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check business slug: %w", err)
	}
	if clientBySlug != nil {
		log.Warn("tenant auth register duplicate slug", zap.String("slug", req.BusinessSlug))
		return nil, helper.NewConflict("Business Slug sudah digunakan")
	}

	// 3. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("tenant auth register hash password failed", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var registeredUser *entity.User
	var registeredClient *entity.Client
	var verificationToken string

	// 4. Begin transaction
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		// Create User
		user := &entity.User{
			ID:              uuid.New(),
			Email:           req.Email,
			Name:            req.UserName,
			PasswordHash:    string(hashedPassword),
			Phone:           req.Phone,
			IsEmailVerified: false,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		if err := uc.userRepo.Create(ctx, tx, user); err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		// Create Client (Business)
		client := &entity.Client{
			ID:               uuid.New(),
			Name:             req.BusinessName,
			Slug:             req.BusinessSlug,
			OwnerUserID:      user.ID,
			SubscriptionTier: "free", // Default to free plan
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
			UserID:     user.ID,
			Role:       "owner",
			IsActive:   true,
			AcceptedAt: &now,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := uc.clientUserRepo.Create(ctx, tx, clientUser); err != nil {
			return fmt.Errorf("create client user: %w", err)
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
		payloadBytes, _ := json.Marshal(payload)

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
		registeredClient = client
		return nil
	})

	if err != nil {
		log.Error("tenant auth register transaction failed", zap.Error(err))
		return nil, err
	}

	// 5. Save verification token to Redis (Expiry 24 hours)
	tokenKey := fmt.Sprintf("auth:verify_email:%s", verificationToken)
	if err := uc.redis.Set(ctx, tokenKey, registeredUser.ID.String(), 24*time.Hour).Err(); err != nil {
		log.Error("tenant auth register failed to save verification token to redis", zap.Error(err))
		// We don't fail the registration if redis fails here, but user might need to request a new email
	}

	log.Info("tenant auth register success", zap.String("user_id", registeredUser.ID.String()), zap.String("client_id", registeredClient.ID.String()))

	return &model.TenantRegisterResponse{
		User:   *converter.UserToResponse(registeredUser),
		Client: *converter.ClientToResponse(registeredClient),
	}, nil
}

func (uc *TenantAuthUseCase) Login(ctx context.Context, req *model.TenantLoginRequest) (*model.TenantLoginResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth login start", zap.String("email", req.Email))

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

	// 3. Update Last Login
	now := time.Now()
	user.LastLoginAt = &now
	_ = uc.userRepo.Update(ctx, uc.db.Gorm, user)

	// 4. Check if user is associated with an active client
	if len(user.ClientUsers) == 0 {
		log.Error("tenant auth login no active client user found")
		return nil, helper.NewForbidden("Akun Anda belum tergabung dengan tenant bisnis manapun")
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

	log.Info("tenant auth login success", zap.String("user_id", user.ID.String()), zap.String("client_id", client.ID.String()))

	return &model.TenantLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(uc.jwtConfig.AccessExpiry.Seconds()),
		User:         *converter.UserToResponse(user),
		Client:       *converter.ClientToResponse(&client),
		Role:         clientUser.Role,
	}, nil
}

func (uc *TenantAuthUseCase) VerifyEmail(ctx context.Context, req *model.TenantVerifyEmailRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("tenant auth verify email start")

	tokenKey := fmt.Sprintf("auth:verify_email:%s", req.Token)

	// 1. Get User ID from Redis
	userIDStr, err := uc.redis.Get(ctx, tokenKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			log.Warn("tenant auth verify email token not found or expired")
			return helper.NewBadRequest("Token verifikasi tidak valid atau sudah kadaluarsa")
		}
		log.Error("tenant auth verify email redis get failed", zap.Error(err))
		return err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Error("tenant auth verify email parse uuid failed", zap.Error(err))
		return err
	}

	// 2. Begin transaction to update user
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
		return err
	}

	// 3. Delete token from Redis
	_ = uc.redis.Del(ctx, tokenKey)

	log.Info("tenant auth verify email success", zap.String("user_id", userID.String()))
	return nil
}
