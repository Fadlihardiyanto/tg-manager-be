package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IAdminPermissionRepository interface {
	// Dipakai saat finalizeLogin — ambil semua permission name milik admin
	FindPermissionNamesByAdminUserID(ctx context.Context, db *gorm.DB, adminID uuid.UUID) ([]string, error)

	// Dipakai saat middleware perlu re-validate (opsional, jika tidak cache di JWT)
	FindAllByAdminUserID(ctx context.Context, db *gorm.DB, adminID uuid.UUID) ([]entity.AdminPermission, error)

	// Untuk admin role management
	FindAll(ctx context.Context, db *gorm.DB) ([]entity.AdminPermission, error)
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.AdminPermission, error)
	FindByName(ctx context.Context, db *gorm.DB, name string) (*entity.AdminPermission, error)
}

type adminPermissionRepository struct {
	log *zap.Logger
}

func NewAdminPermissionRepository(log *zap.Logger) IAdminPermissionRepository {
	return &adminPermissionRepository{log: log}
}

// FindPermissionNamesByAdminUserID mengambil flat list permission name
// untuk di-embed ke JWT claims saat login.
// Query: admin_users -> admin_user_roles -> admin_roles -> admin_role_permissions -> admin_permissions
func (r *adminPermissionRepository) FindPermissionNamesByAdminUserID(ctx context.Context, db *gorm.DB, adminID uuid.UUID) ([]string, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin permission repo find permission names start", zap.String("admin_id", adminID.String()))

	var names []string

	err := db.WithContext(ctx).
		Model(&entity.AdminPermission{}).
		Distinct("admin_permissions.name").
		Joins("JOIN admin_role_permissions arp ON arp.admin_permission_id = admin_permissions.id").
		Joins("JOIN admin_user_roles aur ON aur.admin_role_id = arp.admin_role_id").
		Where("aur.admin_user_id = ? AND admin_permissions.deleted_at IS NULL", adminID).
		Pluck("admin_permissions.name", &names).Error

	if err != nil {
		log.Error("admin permission repo find permission names failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin permission repo find permission names success", zap.String("admin_id", adminID.String()), zap.Int("count", len(names)))

	return names, nil
}

func (r *adminPermissionRepository) FindAllByAdminUserID(ctx context.Context, db *gorm.DB, adminID uuid.UUID) ([]entity.AdminPermission, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin permission repo find all by admin start", zap.String("admin_id", adminID.String()))

	var permissions []entity.AdminPermission

	err := db.WithContext(ctx).
		Distinct("admin_permissions.*").
		Model(&entity.AdminPermission{}).
		Joins("JOIN admin_role_permissions arp ON arp.admin_permission_id = admin_permissions.id").
		Joins("JOIN admin_user_roles aur ON aur.admin_role_id = arp.admin_role_id").
		Where("aur.admin_user_id = ? AND admin_permissions.deleted_at IS NULL", adminID).
		Find(&permissions).Error
	if err != nil {
		log.Error("admin permission repo find all by admin failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin permission repo find all by admin success", zap.String("admin_id", adminID.String()), zap.Int("count", len(permissions)))

	return permissions, nil
}

func (r *adminPermissionRepository) FindAll(ctx context.Context, db *gorm.DB) ([]entity.AdminPermission, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin permission repo find all start")

	var permissions []entity.AdminPermission
	err := db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("module, action").
		Find(&permissions).Error
	if err != nil {
		log.Error("admin permission repo find all failed", zap.Error(err))
	}
	return permissions, err
}

func (r *adminPermissionRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.AdminPermission, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin permission repo find by id start", zap.String("permission_id", id.String()))

	var p entity.AdminPermission
	err := db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&p).Error
	if err != nil {
		log.Error("admin permission repo find by id failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin permission repo find by id success", zap.String("permission_id", id.String()))
	return &p, nil
}

func (r *adminPermissionRepository) FindByName(ctx context.Context, db *gorm.DB, name string) (*entity.AdminPermission, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin permission repo find by name start", zap.String("name", name))

	var p entity.AdminPermission
	err := db.WithContext(ctx).
		Where("name = ? AND deleted_at IS NULL", name).
		First(&p).Error
	if err != nil {
		log.Error("admin permission repo find by name failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin permission repo find by name success", zap.String("name", name))
	return &p, nil
}
