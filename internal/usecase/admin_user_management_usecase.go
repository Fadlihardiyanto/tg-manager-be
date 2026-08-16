package usecase

import (
	"context"
	"errors"
	"fmt"
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

// IAdminUserManagementUseCase handles admin user management operations.
type IAdminUserManagementUseCase interface {
	ListAdmins(ctx context.Context, req *model.AdminUserListRequest) ([]model.AdminUserResponse, int64, error)
	GetAdmin(ctx context.Context, req *model.AdminUserGetRequest) (*model.AdminUserResponse, error)
	CreateAdmin(ctx context.Context, req *model.AdminUserCreateRequest) (*model.AdminUserResponse, error)
	UpdateAdmin(ctx context.Context, req *model.AdminUserUpdateRequest) (*model.AdminUserResponse, error)
	DeleteAdmin(ctx context.Context, req *model.AdminUserDeleteRequest) error
	BulkDeleteAdmins(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult
	ActivateAdmin(ctx context.Context, req *model.AdminUserActivateRequest) error
	DeactivateAdmin(ctx context.Context, req *model.AdminUserDeactivateRequest) error
}

type adminUserManagementUseCase struct {
	db         *entity.Database
	adminRepo  repository.IAdminUserRepository
	roleRepo   repository.IAdminRoleRepository
	log        *zap.Logger
	bcryptCost int
}

func NewAdminUserManagementUseCase(
	db *entity.Database,
	adminRepo repository.IAdminUserRepository,
	roleRepo repository.IAdminRoleRepository,
	log *zap.Logger,
	bcryptCost int,
) IAdminUserManagementUseCase {
	// Fail-fast: db/log dipakai di semua method — zero value = panic di
	// runtime tanpa jejak wiring yang salah.
	if db == nil || log == nil {
		panic("admin user management usecase: db and log are required")
	}
	return &adminUserManagementUseCase{
		db:         db,
		adminRepo:  adminRepo,
		roleRepo:   roleRepo,
		log:        log,
		bcryptCost: bcryptCost,
	}
}

func (uc *adminUserManagementUseCase) ListAdmins(ctx context.Context, req *model.AdminUserListRequest) ([]model.AdminUserResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management list start", zap.Int("offset", req.Offset), zap.Int("limit", req.Limit))

	if err := requirePermission(req.CallerPermissions, "admins.read"); err != nil {
		log.Warn("admin user management list forbidden", zap.Error(err))
		return nil, 0, err
	}

	admins, total, err := uc.adminRepo.FindAllPaginatedWithRoles(ctx, uc.db.Gorm, req.Offset, req.Limit)
	if err != nil {
		log.Error("admin user management list failed", zap.Error(err))
		return nil, 0, err
	}

	responses := make([]model.AdminUserResponse, len(admins))
	for i := range admins {
		responses[i] = converter.ToAdminUserResponse(&admins[i])
	}

	log.Info("admin user management list success", zap.Int("count", len(responses)), zap.Int64("total", total))
	return responses, total, nil
}

func (uc *adminUserManagementUseCase) GetAdmin(ctx context.Context, req *model.AdminUserGetRequest) (*model.AdminUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management get start", zap.String("admin_id", req.AdminID.String()))

	if err := requirePermission(req.CallerPermissions, "admins.read"); err != nil {
		log.Warn("admin user management get forbidden", zap.Error(err))
		return nil, err
	}

	admin := &entity.AdminUser{}
	if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, admin, req.AdminID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin user management get not found", zap.String("admin_id", req.AdminID.String()))
			return nil, helper.NewNotFound("admin user")
		}
		log.Error("admin user management get load failed", zap.Error(err))
		return nil, err
	}

	roles, err := uc.roleRepo.FindByAdminUserID(ctx, uc.db.Gorm, req.AdminID)
	if err != nil {
		log.Error("admin user management get roles failed", zap.Error(err))
		return nil, err
	}
	admin.Roles = roles

	resp := converter.ToAdminUserResponse(admin)
	log.Info("admin user management get success", zap.String("admin_id", req.AdminID.String()))
	return &resp, nil
}

func (uc *adminUserManagementUseCase) CreateAdmin(ctx context.Context, req *model.AdminUserCreateRequest) (*model.AdminUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management create start", zap.String("email", helper.HashIdentifier(req.Email)))

	if err := requirePermission(req.CallerPermissions, "admins.create"); err != nil {
		log.Warn("admin user management create forbidden", zap.Error(err))
		return nil, err
	}

	exists, err := uc.adminRepo.EmailExists(ctx, uc.db.Gorm, req.Email)
	if err != nil {
		log.Error("admin user management create email check failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check email existence: %w", err)
	}
	if exists {
		log.Warn("admin user management create duplicate email", zap.String("email", helper.HashIdentifier(req.Email)))
		return nil, helper.NewConflict("email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), uc.bcryptCost)
	if err != nil {
		log.Error("admin user management create hash failed", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	admin := &entity.AdminUser{
		ID:               uuid.New(),
		Email:            req.Email,
		Name:             req.Name,
		PasswordHash:     string(hashedPassword),
		IsActive:         true,
		IsTwoFAEnabled:   false,
		FailedLoginCount: 0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := uc.adminRepo.Create(ctx, uc.db.Gorm, admin); err != nil {
		log.Error("admin user management create failed", zap.Error(err))
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	resp := converter.ToAdminUserResponse(admin)
	log.Info("admin user management create success", zap.String("admin_id", admin.ID.String()))
	return &resp, nil
}

func (uc *adminUserManagementUseCase) UpdateAdmin(ctx context.Context, req *model.AdminUserUpdateRequest) (*model.AdminUserResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management update start", zap.String("admin_id", req.AdminID.String()))

	if err := requirePermission(req.CallerPermissions, "admins.update"); err != nil {
		log.Warn("admin user management update forbidden", zap.Error(err))
		return nil, err
	}

	admin := &entity.AdminUser{}
	if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, admin, req.AdminID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin user management update not found", zap.String("admin_id", req.AdminID.String()))
			return nil, helper.NewNotFound("admin user")
		}
		log.Error("admin user management update load failed", zap.Error(err))
		return nil, err
	}

	if req.Email != "" && req.Email != admin.Email {
		exists, err := uc.adminRepo.EmailExists(ctx, uc.db.Gorm, req.Email)
		if err != nil {
			log.Error("admin user management update email check failed", zap.Error(err))
			return nil, fmt.Errorf("failed to check email existence: %w", err)
		}
		if exists {
			return nil, helper.NewConflict("email already exists")
		}
		admin.Email = req.Email
	}
	if req.Name != "" {
		admin.Name = req.Name
	}

	admin.UpdatedAt = time.Now()
	if err := uc.adminRepo.Update(ctx, uc.db.Gorm, admin); err != nil {
		log.Error("admin user management update failed", zap.Error(err))
		return nil, fmt.Errorf("failed to update admin user: %w", err)
	}

	roles, err := uc.roleRepo.FindByAdminUserID(ctx, uc.db.Gorm, admin.ID)
	if err != nil {
		log.Error("admin user management update roles failed", zap.Error(err))
		return nil, err
	}
	admin.Roles = roles

	resp := converter.ToAdminUserResponse(admin)
	log.Info("admin user management update success", zap.String("admin_id", admin.ID.String()))
	return &resp, nil
}

func (uc *adminUserManagementUseCase) DeleteAdmin(ctx context.Context, req *model.AdminUserDeleteRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management delete start", zap.String("admin_id", req.AdminID.String()))

	if err := requirePermission(req.CallerPermissions, "admins.delete"); err != nil {
		log.Warn("admin user management delete forbidden", zap.Error(err))
		return err
	}

	admin := &entity.AdminUser{}
	if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, admin, req.AdminID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin user management delete not found", zap.String("admin_id", req.AdminID.String()))
			return helper.NewNotFound("admin user")
		}
		log.Error("admin user management delete load failed", zap.Error(err))
		return err
	}

	now := time.Now()
	admin.IsActive = false
	admin.DeletedAt = &now
	admin.UpdatedAt = now

	if err := uc.adminRepo.Update(ctx, uc.db.Gorm, admin); err != nil {
		log.Error("admin user management delete failed", zap.Error(err))
		return err
	}

	log.Info("admin user management delete success", zap.String("admin_id", req.AdminID.String()))
	return nil
}

func (uc *adminUserManagementUseCase) BulkDeleteAdmins(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.DeleteAdmin(ctx, &model.AdminUserDeleteRequest{AdminID: id, CallerPermissions: callerPermissions})
	})
}

func (uc *adminUserManagementUseCase) ActivateAdmin(ctx context.Context, req *model.AdminUserActivateRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management activate start", zap.String("admin_id", req.AdminID.String()))

	if err := requirePermission(req.CallerPermissions, "admins.update"); err != nil {
		log.Warn("admin user management activate forbidden", zap.Error(err))
		return err
	}

	admin := &entity.AdminUser{}
	if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, admin, req.AdminID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin user management activate not found", zap.String("admin_id", req.AdminID.String()))
			return helper.NewNotFound("admin user")
		}
		log.Error("admin user management activate load failed", zap.Error(err))
		return err
	}

	admin.IsActive = true
	admin.UpdatedAt = time.Now()
	if err := uc.adminRepo.Update(ctx, uc.db.Gorm, admin); err != nil {
		log.Error("admin user management activate failed", zap.Error(err))
		return err
	}

	log.Info("admin user management activate success", zap.String("admin_id", req.AdminID.String()))
	return nil
}

func (uc *adminUserManagementUseCase) DeactivateAdmin(ctx context.Context, req *model.AdminUserDeactivateRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin user management deactivate start", zap.String("admin_id", req.AdminID.String()))

	if err := requirePermission(req.CallerPermissions, "admins.update"); err != nil {
		log.Warn("admin user management deactivate forbidden", zap.Error(err))
		return err
	}

	admin := &entity.AdminUser{}
	if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, admin, req.AdminID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin user management deactivate not found", zap.String("admin_id", req.AdminID.String()))
			return helper.NewNotFound("admin user")
		}
		log.Error("admin user management deactivate load failed", zap.Error(err))
		return err
	}

	admin.IsActive = false
	admin.UpdatedAt = time.Now()
	if err := uc.adminRepo.Update(ctx, uc.db.Gorm, admin); err != nil {
		log.Error("admin user management deactivate failed", zap.Error(err))
		return err
	}

	log.Info("admin user management deactivate success", zap.String("admin_id", req.AdminID.String()))
	return nil
}
