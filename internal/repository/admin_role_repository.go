package repository

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IAdminRoleRepository interface {
	FindAll(ctx context.Context, db *gorm.DB) ([]entity.AdminRole, error)
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.AdminRole, error)
	FindByName(ctx context.Context, db *gorm.DB, name string) (*entity.AdminRole, error)
	FindByAdminUserID(ctx context.Context, db *gorm.DB, adminID uuid.UUID) ([]entity.AdminRole, error)

	Create(ctx context.Context, db *gorm.DB, role *entity.AdminRole) error
	Update(ctx context.Context, db *gorm.DB, role *entity.AdminRole) error
	Delete(ctx context.Context, db *gorm.DB, id uuid.UUID) error

	// Permission assignment
	AssignPermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	RevokePermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, permissionIDs []uuid.UUID) error
	SyncPermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, permissionIDs []uuid.UUID) error

	// Role assignment ke admin user
	AssignRoleToAdmin(ctx context.Context, db *gorm.DB, adminID, roleID uuid.UUID) error
	RevokeRoleFromAdmin(ctx context.Context, db *gorm.DB, adminID, roleID uuid.UUID) error
	SyncAdminRoles(ctx context.Context, db *gorm.DB, adminID uuid.UUID, roleIDs []uuid.UUID) error
}

type adminRoleRepository struct {
	log *zap.Logger
}

func NewAdminRoleRepository(log *zap.Logger) IAdminRoleRepository {
	return &adminRoleRepository{log: log}
}

func (r *adminRoleRepository) FindAll(ctx context.Context, db *gorm.DB) ([]entity.AdminRole, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo find all start")

	var roles []entity.AdminRole
	err := db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Preload("Permissions", "deleted_at IS NULL").
		Order("name").
		Find(&roles).Error
	if err != nil {
		log.Error("admin role repo find all failed", zap.Error(err))
	}
	return roles, err
}

func (r *adminRoleRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.AdminRole, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo find by id start", zap.String("role_id", id.String()))

	var role entity.AdminRole
	err := db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Preload("Permissions", "deleted_at IS NULL").
		First(&role).Error
	if err != nil {
		log.Error("admin role repo find by id failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role repo find by id success", zap.String("role_id", id.String()))
	return &role, nil
}

func (r *adminRoleRepository) FindByName(ctx context.Context, db *gorm.DB, name string) (*entity.AdminRole, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo find by name start", zap.String("name", name))

	var role entity.AdminRole
	err := db.WithContext(ctx).
		Where("name = ? AND deleted_at IS NULL", name).
		Preload("Permissions", "deleted_at IS NULL").
		First(&role).Error
	if err != nil {
		log.Error("admin role repo find by name failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role repo find by name success", zap.String("name", name))
	return &role, nil
}

func (r *adminRoleRepository) FindByAdminUserID(ctx context.Context, db *gorm.DB, adminID uuid.UUID) ([]entity.AdminRole, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo find by admin user id start", zap.String("admin_id", adminID.String()))

	var roles []entity.AdminRole
	err := db.WithContext(ctx).
		Model(&entity.AdminRole{}).
		Joins("JOIN admin_user_roles aur ON aur.admin_role_id = admin_roles.id").
		Where("aur.admin_user_id = ? AND admin_roles.deleted_at IS NULL", adminID).
		Preload("Permissions", "deleted_at IS NULL").
		Find(&roles).Error
	if err != nil {
		log.Error("admin role repo find by admin user id failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role repo find by admin user id success", zap.String("admin_id", adminID.String()), zap.Int("count", len(roles)))
	return roles, err
}

func (r *adminRoleRepository) Create(ctx context.Context, db *gorm.DB, role *entity.AdminRole) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo create start", zap.String("name", role.Name))

	if err := db.WithContext(ctx).Create(role).Error; err != nil {
		log.Error("admin role repo create failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo create success", zap.String("role_id", role.ID.String()))
	return nil
}

func (r *adminRoleRepository) Update(ctx context.Context, db *gorm.DB, role *entity.AdminRole) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo update start", zap.String("role_id", role.ID.String()))

	if err := db.WithContext(ctx).Save(role).Error; err != nil {
		log.Error("admin role repo update failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo update success", zap.String("role_id", role.ID.String()))
	return nil
}

func (r *adminRoleRepository) Delete(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo delete start", zap.String("role_id", id.String()))

	now := time.Now()
	if err := db.WithContext(ctx).Model(&entity.AdminRole{}).
		Where("id = ?", id).
		Update("deleted_at", now).Error; err != nil {
		log.Error("admin role repo delete failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo delete success", zap.String("role_id", id.String()))
	return nil
}

// AssignPermissions menambah permissions ke role (tidak hapus yang lama)
func (r *adminRoleRepository) AssignPermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo assign permissions start", zap.String("role_id", roleID.String()), zap.Int("count", len(permissionIDs)))

	role, err := r.FindByID(ctx, db, roleID)
	if err != nil {
		return err
	}

	permissions := make([]entity.AdminPermission, len(permissionIDs))
	for i, pid := range permissionIDs {
		permissions[i] = entity.AdminPermission{ID: pid}
	}

	if err := db.WithContext(ctx).Model(role).Association("Permissions").Append(permissions); err != nil {
		log.Error("admin role repo assign permissions failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo assign permissions success", zap.String("role_id", roleID.String()))
	return nil
}

// RevokePermissions mencabut permissions tertentu dari role
func (r *adminRoleRepository) RevokePermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo revoke permissions start", zap.String("role_id", roleID.String()), zap.Int("count", len(permissionIDs)))

	role, err := r.FindByID(ctx, db, roleID)
	if err != nil {
		return err
	}

	permissions := make([]entity.AdminPermission, len(permissionIDs))
	for i, pid := range permissionIDs {
		permissions[i] = entity.AdminPermission{ID: pid}
	}

	if err := db.WithContext(ctx).Model(role).Association("Permissions").Delete(permissions); err != nil {
		log.Error("admin role repo revoke permissions failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo revoke permissions success", zap.String("role_id", roleID.String()))
	return nil
}

// SyncPermissions replace semua permissions role sekaligus — idiomatic untuk update
func (r *adminRoleRepository) SyncPermissions(ctx context.Context, db *gorm.DB, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo sync permissions start", zap.String("role_id", roleID.String()), zap.Int("count", len(permissionIDs)))

	role, err := r.FindByID(ctx, db, roleID)
	if err != nil {
		return err
	}

	permissions := make([]entity.AdminPermission, len(permissionIDs))
	for i, pid := range permissionIDs {
		permissions[i] = entity.AdminPermission{ID: pid}
	}

	// Replace = hapus semua lama, assign yang baru — bungkus dalam transaction:
	// gagal di tengah (koneksi putus, FK) tidak boleh meninggalkan role dengan
	// permission kosong (atau admin tanpa roles sama sekali).
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(role).Association("Permissions").Replace(permissions)
	})
	if err != nil {
		log.Error("admin role repo sync permissions failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo sync permissions success", zap.String("role_id", roleID.String()))
	return nil
}

func (r *adminRoleRepository) AssignRoleToAdmin(ctx context.Context, db *gorm.DB, adminID, roleID uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo assign role to admin start", zap.String("admin_id", adminID.String()), zap.String("role_id", roleID.String()))

	junction := entity.AdminUserRole{
		AdminUserID: adminID,
		AdminRoleID: roleID,
	}
	if err := db.WithContext(ctx).
		Where(junction).
		FirstOrCreate(&junction).Error; err != nil {
		log.Error("admin role repo assign role to admin failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo assign role to admin success", zap.String("admin_id", adminID.String()), zap.String("role_id", roleID.String()))
	return nil
}

func (r *adminRoleRepository) RevokeRoleFromAdmin(ctx context.Context, db *gorm.DB, adminID, roleID uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo revoke role from admin start", zap.String("admin_id", adminID.String()), zap.String("role_id", roleID.String()))

	if err := db.WithContext(ctx).
		Where("admin_user_id = ? AND admin_role_id = ?", adminID, roleID).
		Delete(&entity.AdminUserRole{}).Error; err != nil {
		log.Error("admin role repo revoke role from admin failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo revoke role from admin success", zap.String("admin_id", adminID.String()), zap.String("role_id", roleID.String()))
	return nil
}

func (r *adminRoleRepository) SyncAdminRoles(ctx context.Context, db *gorm.DB, adminID uuid.UUID, roleIDs []uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin role repo sync admin roles start", zap.String("admin_id", adminID.String()), zap.Int("count", len(roleIDs)))

	admin := &entity.AdminUser{ID: adminID}

	roles := make([]entity.AdminRole, len(roleIDs))
	for i, rid := range roleIDs {
		roles[i] = entity.AdminRole{ID: rid}
	}

	// Replace dalam transaction — gagal di tengah tidak boleh membuat admin
	// kehilangan SEMUA roles (lihat SyncPermissions).
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(admin).Association("Roles").Replace(roles)
	})
	if err != nil {
		log.Error("admin role repo sync admin roles failed", zap.Error(err))
		return err
	}
	log.Info("admin role repo sync admin roles success", zap.String("admin_id", adminID.String()))
	return nil
}
