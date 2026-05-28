package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ITenantPermissionRepository handles permission lookup for tenant users.
// Permissions are resolved via: client_users.role → roles.name → role_permissions → permissions
type ITenantPermissionRepository interface {
	// FindPermissionNamesByRole returns a flat list of permission names for a given role name.
	// Used at login to embed permissions into JWT claims (no DB hit on subsequent requests).
	// Query: roles → role_permissions → permissions
	FindPermissionNamesByRole(ctx context.Context, db *gorm.DB, roleName string) ([]string, error)

	// FindAll returns all tenant permissions (for seeder / admin visibility).
	FindAll(ctx context.Context, db *gorm.DB) ([]entity.Permission, error)
}

// =============================================================================
// Implementation
// =============================================================================

type tenantPermissionRepository struct {
	log *zap.Logger
}

func NewTenantPermissionRepository(log *zap.Logger) ITenantPermissionRepository {
	return &tenantPermissionRepository{log: log}
}

// FindPermissionNamesByRole fetches flat permission names for a given role name.
// e.g. role = "owner" → ["packages.create", "packages.read", "bots.create", ...]
func (r *tenantPermissionRepository) FindPermissionNamesByRole(ctx context.Context, db *gorm.DB, roleName string) ([]string, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("tenant permission repo find permission names by role start", zap.String("role", roleName))

	var names []string

	err := db.WithContext(ctx).
		Model(&entity.Permission{}).
		Distinct("permissions.name").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("JOIN roles r ON r.id = rp.role_id").
		Where("r.name = ? AND r.deleted_at IS NULL AND permissions.deleted_at IS NULL", roleName).
		Pluck("permissions.name", &names).Error

	if err != nil {
		log.Error("tenant permission repo find permission names by role failed", zap.Error(err))
		return nil, err
	}

	log.Info("tenant permission repo find permission names by role success",
		zap.String("role", roleName),
		zap.Int("count", len(names)),
	)
	return names, nil
}

func (r *tenantPermissionRepository) FindAll(ctx context.Context, db *gorm.DB) ([]entity.Permission, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("tenant permission repo find all start")

	var permissions []entity.Permission
	err := db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("module, action").
		Find(&permissions).Error

	if err != nil {
		log.Error("tenant permission repo find all failed", zap.Error(err))
	}
	return permissions, err
}
