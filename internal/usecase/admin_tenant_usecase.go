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

// IAdminTenantUseCase handles superadmin management of tenant/client accounts.
type IAdminTenantUseCase interface {
	ListAllClients(ctx context.Context, req *model.AdminListAllClientsRequest) ([]model.ClientResponse, int64, error)
	GetClientDetail(ctx context.Context, req *model.AdminGetClientDetailRequest) (*model.ClientResponse, error)
	CreateClient(ctx context.Context, req *model.AdminCreateClientRequest) (*model.ClientResponse, error)
	UpdateClient(ctx context.Context, req *model.AdminUpdateClientRequest) (*model.ClientResponse, error)
	DeleteClient(ctx context.Context, req *model.AdminDeleteClientRequest) error
	BulkDeleteClients(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult
	ActivateClient(ctx context.Context, req *model.AdminActivateClientRequest) error
	DeactivateClient(ctx context.Context, req *model.AdminDeactivateClientRequest) error
}

type adminTenantUseCase struct {
	db             *entity.Database
	clientRepo     repository.IClientRepository
	userRepo       repository.IUserRepository
	clientUserRepo repository.IClientUserRepository
	log            *zap.Logger
	bcryptCost     int
}

func NewAdminTenantUseCase(
	db *entity.Database,
	clientRepo repository.IClientRepository,
	userRepo repository.IUserRepository,
	clientUserRepo repository.IClientUserRepository,
	log *zap.Logger,
	bcryptCost int,
) IAdminTenantUseCase {
	return &adminTenantUseCase{
		db:             db,
		clientRepo:     clientRepo,
		userRepo:       userRepo,
		clientUserRepo: clientUserRepo,
		log:            log,
		bcryptCost:     bcryptCost,
	}
}

func (uc *adminTenantUseCase) ListAllClients(ctx context.Context, req *model.AdminListAllClientsRequest) ([]model.ClientResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant list clients start", zap.String("client_id", req.ClientID), zap.String("name", req.Name), zap.String("slug", req.Slug), zap.String("active", req.Active), zap.String("subscription_tier", req.SubscriptionTier), zap.Int("page", req.Page), zap.Int("size", req.Size))

	if err := requirePermission(req.CallerPermissions, "clients.read"); err != nil {
		log.Warn("admin tenant list clients forbidden", zap.Error(err))
		return nil, 0, err
	}

	clients, total, err := uc.clientRepo.FindAllPaginated(ctx, uc.db.Gorm, req)
	if err != nil {
		log.Error("admin tenant list clients failed", zap.Error(err))
		return nil, 0, err
	}

	log.Info("admin tenant list clients success", zap.Int("count", len(clients)), zap.Int64("total", total))
	return converter.ClientsToResponse(clients), total, nil
}

func (uc *adminTenantUseCase) GetClientDetail(ctx context.Context, req *model.AdminGetClientDetailRequest) (*model.ClientResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant get client start", zap.String("client_id", req.ClientID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.read"); err != nil {
		log.Warn("admin tenant get client forbidden", zap.Error(err))
		return nil, err
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin tenant get client not found", zap.String("client_id", req.ClientID.String()))
			return nil, helper.NewNotFound("client")
		}
		log.Error("admin tenant get client failed", zap.Error(err))
		return nil, err
	}

	resp := converter.ClientToResponse(client)
	log.Info("admin tenant get client success", zap.String("client_id", req.ClientID.String()))
	return resp, nil
}

func (uc *adminTenantUseCase) CreateClient(ctx context.Context, req *model.AdminCreateClientRequest) (*model.ClientResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant create client start", zap.String("name", req.Name), zap.String("slug", req.Slug))

	if err := requirePermission(req.CallerPermissions, "clients.create"); err != nil {
		log.Warn("admin tenant create client forbidden", zap.Error(err))
		return nil, err
	}

	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if req.Slug == "" {
		return nil, helper.NewBadRequest("slug wajib diisi")
	}

	if existing, err := uc.clientRepo.FindBySlug(ctx, uc.db.Gorm, req.Slug); err == nil && existing != nil {
		log.Warn("admin tenant create client duplicate slug", zap.String("slug", req.Slug))
		return nil, helper.NewConflict(fmt.Sprintf("client with slug '%s' already exists", req.Slug))
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("admin tenant create client slug check failed", zap.Error(err))
		return nil, err
	}

	if req.OwnerUserID == nil && req.OwnerUser == nil {
		return nil, helper.NewBadRequest("owner_user_id atau owner_user wajib diisi")
	}

	var createdClient *entity.Client
	err := uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ownerUserID uuid.UUID
		if req.OwnerUserID != nil {
			if err := uc.ensureUserExistsTx(ctx, tx, *req.OwnerUserID); err != nil {
				return err
			}
			ownerUserID = *req.OwnerUserID
		} else {
			owner, err := uc.createOwnerUserTx(ctx, tx, req.OwnerUser)
			if err != nil {
				return err
			}
			ownerUserID = owner.ID
		}

		now := time.Now()
		client := &entity.Client{
			ID:               uuid.New(),
			Name:             strings.TrimSpace(req.Name),
			Slug:             req.Slug,
			Description:      req.Description,
			LogoURL:          req.LogoURL,
			OwnerUserID:      ownerUserID,
			IsActive:         true,
			SubscriptionTier: normalizeSubscriptionTier(req.SubscriptionTier),
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := uc.clientRepo.Create(ctx, tx, client); err != nil {
			log.Error("admin tenant create client failed", zap.Error(err))
			return fmt.Errorf("failed to create client: %w", err)
		}

		clientUser := &entity.ClientUser{
			ID:         uuid.New(),
			ClientID:   client.ID,
			UserID:     ownerUserID,
			Role:       "owner",
			IsActive:   true,
			AcceptedAt: &now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := uc.clientUserRepo.Create(ctx, tx, clientUser); err != nil {
			log.Error("admin tenant create client user failed", zap.Error(err))
			return fmt.Errorf("failed to create client owner relation: %w", err)
		}

		createdClient = client
		return nil
	})
	if err != nil {
		return nil, err
	}

	resp := converter.ClientToResponse(createdClient)
	log.Info("admin tenant create client success", zap.String("client_id", createdClient.ID.String()))
	return resp, nil
}

func (uc *adminTenantUseCase) UpdateClient(ctx context.Context, req *model.AdminUpdateClientRequest) (*model.ClientResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant update client start", zap.String("client_id", req.ClientID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.update"); err != nil {
		log.Warn("admin tenant update client forbidden", zap.Error(err))
		return nil, err
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin tenant update client not found", zap.String("client_id", req.ClientID.String()))
			return nil, helper.NewNotFound("client")
		}
		log.Error("admin tenant update client load failed", zap.Error(err))
		return nil, err
	}

	if req.Slug != "" {
		slug := strings.ToLower(strings.TrimSpace(req.Slug))
		if slug == "" {
			return nil, helper.NewBadRequest("slug wajib diisi")
		}
		existing, err := uc.clientRepo.FindBySlug(ctx, uc.db.Gorm, slug)
		if err == nil && existing != nil && existing.ID != client.ID {
			return nil, helper.NewConflict(fmt.Sprintf("client with slug '%s' already exists", slug))
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Error("admin tenant update client slug check failed", zap.Error(err))
			return nil, err
		}
		client.Slug = slug
	}

	if req.Name != "" {
		client.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		client.Description = req.Description
	}
	if req.LogoURL != "" {
		client.LogoURL = req.LogoURL
	}
	if req.SubscriptionTier != "" {
		client.SubscriptionTier = normalizeSubscriptionTier(req.SubscriptionTier)
	}
	if req.IsActive != nil {
		client.IsActive = *req.IsActive
	}
	if req.OwnerUserID != nil {
		if err := uc.ensureUserExists(ctx, *req.OwnerUserID); err != nil {
			return nil, err
		}
		client.OwnerUserID = *req.OwnerUserID
	}

	client.UpdatedAt = time.Now()
	if err := uc.clientRepo.Update(ctx, uc.db.Gorm, client); err != nil {
		log.Error("admin tenant update client failed", zap.Error(err))
		return nil, fmt.Errorf("failed to update client: %w", err)
	}

	resp := converter.ClientToResponse(client)
	log.Info("admin tenant update client success", zap.String("client_id", req.ClientID.String()))
	return resp, nil
}

func (uc *adminTenantUseCase) DeleteClient(ctx context.Context, req *model.AdminDeleteClientRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant delete client start", zap.String("client_id", req.ClientID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.delete"); err != nil {
		log.Warn("admin tenant delete client forbidden", zap.Error(err))
		return err
	}

	err := uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		client, err := uc.clientRepo.FindByID(ctx, tx, req.ClientID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Warn("admin tenant delete client not found", zap.String("client_id", req.ClientID.String()))
				return helper.NewNotFound("client")
			}
			log.Error("admin tenant delete client load failed", zap.Error(err))
			return err
		}

		if err := uc.clientUserRepo.SoftDeleteByClientID(ctx, tx, client.ID); err != nil {
			log.Error("admin tenant delete client relations failed", zap.Error(err))
			return err
		}

		if err := uc.clientRepo.SoftDelete(ctx, tx, client); err != nil {
			log.Error("admin tenant delete client failed", zap.Error(err))
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Info("admin tenant delete client success", zap.String("client_id", req.ClientID.String()))
	return nil
}

func (uc *adminTenantUseCase) BulkDeleteClients(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.DeleteClient(ctx, &model.AdminDeleteClientRequest{ClientID: id, CallerPermissions: callerPermissions})
	})
}

func (uc *adminTenantUseCase) ActivateClient(ctx context.Context, req *model.AdminActivateClientRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant activate client start", zap.String("client_id", req.ClientID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.update"); err != nil {
		log.Warn("admin tenant activate client forbidden", zap.Error(err))
		return err
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin tenant activate client not found", zap.String("client_id", req.ClientID.String()))
			return helper.NewNotFound("client")
		}
		log.Error("admin tenant activate client load failed", zap.Error(err))
		return err
	}

	if client.IsActive {
		log.Info("admin tenant activate client already active", zap.String("client_id", req.ClientID.String()))
		return nil
	}

	if err := uc.clientRepo.Activate(ctx, uc.db.Gorm, client); err != nil {
		log.Error("admin tenant activate client failed", zap.Error(err))
		return err
	}

	log.Info("admin tenant activate client success", zap.String("client_id", req.ClientID.String()))
	return nil
}

func (uc *adminTenantUseCase) DeactivateClient(ctx context.Context, req *model.AdminDeactivateClientRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin tenant deactivate client start", zap.String("client_id", req.ClientID.String()))

	if err := requirePermission(req.CallerPermissions, "clients.update"); err != nil {
		log.Warn("admin tenant deactivate client forbidden", zap.Error(err))
		return err
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin tenant deactivate client not found", zap.String("client_id", req.ClientID.String()))
			return helper.NewNotFound("client")
		}
		log.Error("admin tenant deactivate client load failed", zap.Error(err))
		return err
	}

	if !client.IsActive {
		log.Info("admin tenant deactivate client already inactive", zap.String("client_id", req.ClientID.String()))
		return nil
	}

	if err := uc.clientRepo.Deactivate(ctx, uc.db.Gorm, client); err != nil {
		log.Error("admin tenant deactivate client failed", zap.Error(err))
		return err
	}

	log.Info("admin tenant deactivate client success", zap.String("client_id", req.ClientID.String()))
	return nil
}

func (uc *adminTenantUseCase) ensureUserExists(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return helper.NewBadRequest("owner_user_id wajib diisi")
	}

	var user entity.User
	err := uc.db.Gorm.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", userID).
		Take(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("user")
		}
		return err
	}

	return nil
}

func (uc *adminTenantUseCase) ensureUserExistsTx(ctx context.Context, tx *gorm.DB, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return helper.NewBadRequest("owner_user_id wajib diisi")
	}

	var user entity.User
	err := tx.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", userID).
		Take(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("user")
		}
		return err
	}
	return nil
}

func (uc *adminTenantUseCase) createOwnerUserTx(ctx context.Context, tx *gorm.DB, req *model.AdminOwnerUserCreateRequest) (*entity.User, error) {
	if req == nil {
		return nil, helper.NewBadRequest("owner_user wajib diisi")
	}

	exists, err := uc.userRepo.EmailExists(ctx, tx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, helper.NewConflict("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), uc.bcryptCost)
	if err != nil {
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

	if err := uc.userRepo.Create(ctx, tx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func normalizeSubscriptionTier(tier string) string {
	tier = strings.ToLower(strings.TrimSpace(tier))
	if tier == "" {
		return "free"
	}
	return tier
}
