package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// IAdminTenantUserUseCase handles CRUD operations for tenant users.
type IAdminTenantUserUseCase interface {
	ListTenantUsers(ctx context.Context, req *model.AdminTenantUserListRequest) ([]model.TenantUserResponse, int64, error)
	GetTenantUser(ctx context.Context, req *model.AdminTenantUserGetRequest) (*model.TenantUserResponse, error)
	CreateTenantUser(ctx context.Context, req *model.AdminTenantUserCreateRequest) (*model.TenantUserResponse, error)
	UpdateTenantUser(ctx context.Context, req *model.AdminTenantUserUpdateRequest) (*model.TenantUserResponse, error)
	DeleteTenantUser(ctx context.Context, req *model.AdminTenantUserDeleteRequest) error
}

type adminTenantUserUseCase struct {
	db             *entity.Database
	clientRepo     repository.IClientRepository
	userRepo       repository.IUserRepository
	clientUserRepo repository.IClientUserRepository
	log            *zap.Logger
	bcryptCost     int
}

func NewAdminTenantUserUseCase(
	db *entity.Database,
	clientRepo repository.IClientRepository,
	userRepo repository.IUserRepository,
	clientUserRepo repository.IClientUserRepository,
	log *zap.Logger,
	bcryptCost int,
) IAdminTenantUserUseCase {
	// Fail-fast: db/log dipakai di semua method — zero value = panic di
	// runtime tanpa jejak wiring yang salah.
	if db == nil || log == nil {
		panic("admin tenant user usecase: db and log are required")
	}
	return &adminTenantUserUseCase{
		db:             db,
		clientRepo:     clientRepo,
		userRepo:       userRepo,
		clientUserRepo: clientUserRepo,
		log:            log,
		bcryptCost:     bcryptCost,
	}
}

func (uc *adminTenantUserUseCase) ListTenantUsers(ctx context.Context, req *model.AdminTenantUserListRequest) ([]model.TenantUserResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant user list start", zap.String("client_id", req.ClientID.String()), zap.String("user_id", req.UserID), zap.String("email", helper.HashIdentifier(req.Email)), zap.String("role", req.Role), zap.String("verified", req.Verified), zap.Int("page", req.Page), zap.Int("limit", req.Limit))

	if err := requirePermission(req.CallerPermissions, "clients.read", req.CallerRoles); err != nil {
		log.Warn("admin tenant user list forbidden", zap.Error(err))
		return nil, 0, err
	}

	if err := uc.ensureClientExists(ctx, req.ClientID); err != nil {
		return nil, 0, err
	}

	clientUsers, total, err := uc.clientUserRepo.FindAllByClientIDPaginated(ctx, uc.db.Gorm, req)
	if err != nil {
		log.Error("admin tenant user list failed", zap.Error(err))
		return nil, 0, err
	}

	responses := make([]model.TenantUserResponse, len(clientUsers))
	for i := range clientUsers {
		responses[i] = converter.TenantUserToResponse(&clientUsers[i])
	}

	log.Info("admin tenant user list success", zap.Int("count", len(responses)), zap.Int64("total", total))
	return responses, total, nil
}

func (uc *adminTenantUserUseCase) GetTenantUser(ctx context.Context, req *model.AdminTenantUserGetRequest) (*model.TenantUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant user get start", zap.String("client_id", req.ClientID.String()), zap.String("user_id", req.UserID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.read", req.CallerRoles); err != nil {
		log.Warn("admin tenant user get forbidden", zap.Error(err))
		return nil, err
	}

	clientUser, err := uc.clientUserRepo.FindByClientAndUserID(ctx, uc.db.Gorm, req.ClientID, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("tenant user")
		}
		log.Error("admin tenant user get failed", zap.Error(err))
		return nil, err
	}

	resp := converter.TenantUserToResponse(clientUser)
	log.Info("admin tenant user get success")
	return &resp, nil
}

func (uc *adminTenantUserUseCase) CreateTenantUser(ctx context.Context, req *model.AdminTenantUserCreateRequest) (*model.TenantUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant user create start", zap.String("client_id", req.ClientID.String()), zap.String("email", helper.HashIdentifier(req.Email)))

	if err := requirePermission(req.CallerPermissions, "clients.update", req.CallerRoles); err != nil {
		log.Warn("admin tenant user create forbidden", zap.Error(err))
		return nil, err
	}

	if err := uc.ensureClientExists(ctx, req.ClientID); err != nil {
		return nil, err
	}

	if exists, err := uc.userRepo.EmailExists(ctx, uc.db.Gorm, req.Email); err != nil {
		log.Error("admin tenant user create email check failed", zap.Error(err))
		return nil, err
	} else if exists {
		return nil, helper.NewConflict("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), uc.bcryptCost)
	if err != nil {
		log.Error("admin tenant user create hash failed", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	user := &entity.User{
		ID:              uuid.New(),
		Email:           strings.TrimSpace(req.Email),
		Name:            strings.TrimSpace(req.Name),
		PasswordHash:    string(hashedPassword),
		Phone:           req.Phone,
		IsEmailVerified: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.userRepo.Create(ctx, uc.db.Gorm, user); err != nil {
		log.Error("admin tenant user create user failed", zap.Error(err))
		return nil, err
	}

	clientUser := &entity.ClientUser{
		ID:         uuid.New(),
		ClientID:   req.ClientID,
		UserID:     user.ID,
		Role:       req.Role,
		IsActive:   true,
		AcceptedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := uc.clientUserRepo.Create(ctx, uc.db.Gorm, clientUser); err != nil {
		log.Error("admin tenant user create client user failed", zap.Error(err))
		return nil, err
	}

	clientUser.User = *user
	resp := converter.TenantUserToResponse(clientUser)
	log.Info("admin tenant user create success", zap.String("user_id", user.ID.String()))
	return &resp, nil
}

func (uc *adminTenantUserUseCase) UpdateTenantUser(ctx context.Context, req *model.AdminTenantUserUpdateRequest) (*model.TenantUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant user update start", zap.String("client_id", req.ClientID.String()), zap.String("user_id", req.UserID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.update", req.CallerRoles); err != nil {
		log.Warn("admin tenant user update forbidden", zap.Error(err))
		return nil, err
	}

	clientUser, err := uc.clientUserRepo.FindByClientAndUserID(ctx, uc.db.Gorm, req.ClientID, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("tenant user")
		}
		log.Error("admin tenant user update load failed", zap.Error(err))
		return nil, err
	}

	user := clientUser.User
	if req.Name != "" {
		user.Name = strings.TrimSpace(req.Name)
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}
	user.UpdatedAt = time.Now()

	if err := uc.userRepo.Update(ctx, uc.db.Gorm, &user); err != nil {
		log.Error("admin tenant user update user failed", zap.Error(err))
		return nil, err
	}

	if req.Role != "" {
		clientUser.Role = req.Role
	}
	if req.IsActive != nil {
		clientUser.IsActive = *req.IsActive
	}
	clientUser.UpdatedAt = time.Now()

	if err := uc.clientUserRepo.Update(ctx, uc.db.Gorm, clientUser); err != nil {
		log.Error("admin tenant user update client user failed", zap.Error(err))
		return nil, err
	}

	clientUser.User = user
	resp := converter.TenantUserToResponse(clientUser)
	log.Info("admin tenant user update success", zap.String("user_id", req.UserID.String()))
	return &resp, nil
}

func (uc *adminTenantUserUseCase) DeleteTenantUser(ctx context.Context, req *model.AdminTenantUserDeleteRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant user delete start", zap.String("client_id", req.ClientID.String()), zap.String("user_id", req.UserID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.update", req.CallerRoles); err != nil {
		log.Warn("admin tenant user delete forbidden", zap.Error(err))
		return err
	}

	clientUser, err := uc.clientUserRepo.FindByClientAndUserID(ctx, uc.db.Gorm, req.ClientID, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("tenant user")
		}
		log.Error("admin tenant user delete load failed", zap.Error(err))
		return err
	}

	// Cascade dalam satu transaction — kegagalan di tengah tidak boleh
	// meninggalkan client-user terhapus tapi user aktif yatim.
	err = uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := uc.clientUserRepo.SoftDelete(ctx, tx, clientUser.ID); err != nil {
			return err
		}

		remaining, err := uc.clientUserRepo.CountActiveByUserID(ctx, tx, clientUser.UserID)
		if err != nil {
			return err
		}
		if remaining == 0 {
			if err := uc.userRepo.SoftDelete(ctx, tx, clientUser.UserID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Error("admin tenant user delete cascade failed", zap.Error(err))
		return err
	}

	log.Info("admin tenant user delete success", zap.String("user_id", req.UserID.String()))
	return nil
}

func (uc *adminTenantUserUseCase) ensureClientExists(ctx context.Context, clientID uuid.UUID) error {
	if clientID == uuid.Nil {
		return helper.NewBadRequest("client_id wajib diisi")
	}

	_, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("client")
		}
		return err
	}
	return nil
}
