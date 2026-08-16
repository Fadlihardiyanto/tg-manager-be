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

// IAdminUserRepository handles persistence for admin_users table.
// Extends the generic IRepository with admin-specific query methods
// needed by the auth and admin management use cases.
type IAdminUserRepository interface {
	IRepository[entity.AdminUser]

	// Auth — login flow
	FindByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.AdminUser, error)

	// Auth — load admin with roles + permissions (for JWT claims)
	FindByIDWithRoles(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.AdminUser, error)

	// Auth — brute force protection
	IncrementFailedLogin(ctx context.Context, tx *gorm.DB, id uuid.UUID) error
	ResetFailedLogin(ctx context.Context, tx *gorm.DB, id uuid.UUID) error
	LockAccount(ctx context.Context, tx *gorm.DB, id uuid.UUID, until time.Time) error

	// Auth — update login metadata
	UpdateLoginInfo(ctx context.Context, tx *gorm.DB, id uuid.UUID, ip string, loginAt time.Time) error

	// Admin — listing with pagination
	FindAllPaginated(ctx context.Context, tx *gorm.DB, offset, limit int, search string) ([]entity.AdminUser, int64, error)
	FindAllPaginatedWithRoles(ctx context.Context, tx *gorm.DB, offset, limit int, search string) ([]entity.AdminUser, int64, error)

	// 2FA
	Update2FAStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, enabled bool) error

	// Registration — check and find by email
	EmailExists(ctx context.Context, tx *gorm.DB, email string) (bool, error)
}

// =============================================================================
// Implementation
// =============================================================================

type AdminUserRepository struct {
	Repository[entity.AdminUser]
	log *zap.Logger
}

func NewAdminUserRepository(log *zap.Logger) IAdminUserRepository {
	return &AdminUserRepository{log: log}
}

// FindByEmail looks up an admin by email (for login).
// Returns gorm.ErrRecordNotFound if not found.
func (r *AdminUserRepository) FindByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.AdminUser, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo find by email start")

	var admin entity.AdminUser
	err := tx.WithContext(ctx).
		Where("email = ? AND deleted_at IS NULL", email).
		Take(&admin).Error
	if err != nil {
		log.Error("admin user repo find by email failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin user repo find by email success", zap.String("admin_id", admin.ID.String()))
	return &admin, nil
}

// FindByIDWithRoles loads an admin with their roles and each role's permissions.
// Used after login to build JWT claims with embedded permissions.
func (r *AdminUserRepository) FindByIDWithRoles(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.AdminUser, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo find by id with roles start", zap.String("admin_id", id.String()))

	var admin entity.AdminUser
	err := tx.WithContext(ctx).
		Preload("Roles", "deleted_at IS NULL").
		Preload("Roles.Permissions", "deleted_at IS NULL").
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&admin).Error
	if err != nil {
		log.Error("admin user repo find by id with roles failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin user repo find by id with roles success", zap.String("admin_id", id.String()), zap.Int("roles_count", len(admin.Roles)))
	return &admin, nil
}

// IncrementFailedLogin atomically increments failed_login_count by 1.
func (r *AdminUserRepository) IncrementFailedLogin(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo increment failed login start", zap.String("admin_id", id.String()))

	if err := tx.WithContext(ctx).
		Model(&entity.AdminUser{}).
		Where("id = ?", id).
		UpdateColumn("failed_login_count", gorm.Expr("failed_login_count + 1")).
		Error; err != nil {
		log.Error("admin user repo increment failed login failed", zap.Error(err))
		return err
	}
	log.Info("admin user repo increment failed login success", zap.String("admin_id", id.String()))
	return nil
}

// ResetFailedLogin sets failed_login_count back to 0 and clears locked_until.
// Called after a successful login.
func (r *AdminUserRepository) ResetFailedLogin(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo reset failed login start", zap.String("admin_id", id.String()))

	if err := tx.WithContext(ctx).
		Model(&entity.AdminUser{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"failed_login_count": 0,
			"locked_until":       nil,
		}).Error; err != nil {
		log.Error("admin user repo reset failed login failed", zap.Error(err))
		return err
	}
	log.Info("admin user repo reset failed login success", zap.String("admin_id", id.String()))
	return nil
}

// LockAccount sets locked_until to prevent login until the specified time.
// Typically called after N failed login attempts.
func (r *AdminUserRepository) LockAccount(ctx context.Context, tx *gorm.DB, id uuid.UUID, until time.Time) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo lock account start", zap.String("admin_id", id.String()), zap.Time("until", until))

	if err := tx.WithContext(ctx).
		Model(&entity.AdminUser{}).
		Where("id = ?", id).
		Update("locked_until", until).
		Error; err != nil {
		log.Error("admin user repo lock account failed", zap.Error(err))
		return err
	}
	log.Info("admin user repo lock account success", zap.String("admin_id", id.String()))
	return nil
}

// UpdateLoginInfo records a successful login (timestamp + IP).
func (r *AdminUserRepository) UpdateLoginInfo(ctx context.Context, tx *gorm.DB, id uuid.UUID, ip string, loginAt time.Time) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo update login info start", zap.String("admin_id", id.String()))

	if err := tx.WithContext(ctx).
		Model(&entity.AdminUser{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_login_at": loginAt,
			"last_login_ip": ip,
		}).Error; err != nil {
		log.Error("admin user repo update login info failed", zap.Error(err))
		return err
	}
	log.Info("admin user repo update login info success", zap.String("admin_id", id.String()))
	return nil
}

// FindAllPaginated returns a page of admin users with total count.
// Excludes soft-deleted records.
func (r *AdminUserRepository) FindAllPaginated(ctx context.Context, tx *gorm.DB, offset, limit int, search string) ([]entity.AdminUser, int64, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo find all paginated start", zap.Int("offset", offset), zap.Int("limit", limit))

	var admins []entity.AdminUser
	var total int64
	offset, limit = clampOffsetLimit(offset, limit)

	db := tx.WithContext(ctx).Model(&entity.AdminUser{}).Where("deleted_at IS NULL")
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		db = db.Where("name ILIKE ? OR email ILIKE ?", pattern, pattern)
	}

	if err := db.Count(&total).Error; err != nil {
		log.Error("admin user repo find all paginated count failed", zap.Error(err))
		return nil, 0, err
	}

	err := db.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&admins).Error
	if err != nil {
		log.Error("admin user repo find all paginated query failed", zap.Error(err))
		return nil, 0, err
	}
	log.Info("admin user repo find all paginated success", zap.Int("count", len(admins)), zap.Int64("total", total))

	return admins, total, nil
}

// FindAllPaginatedWithRoles returns a page of admin users with roles preloaded.
// Excludes soft-deleted records.
func (r *AdminUserRepository) FindAllPaginatedWithRoles(ctx context.Context, tx *gorm.DB, offset, limit int, search string) ([]entity.AdminUser, int64, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo find all paginated with roles start", zap.Int("offset", offset), zap.Int("limit", limit))

	var admins []entity.AdminUser
	var total int64
	offset, limit = clampOffsetLimit(offset, limit)

	db := tx.WithContext(ctx).Model(&entity.AdminUser{}).Where("deleted_at IS NULL")
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		db = db.Where("name ILIKE ? OR email ILIKE ?", pattern, pattern)
	}

	if err := db.Count(&total).Error; err != nil {
		log.Error("admin user repo find all paginated with roles count failed", zap.Error(err))
		return nil, 0, err
	}

	err := db.
		Preload("Roles", "deleted_at IS NULL").
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&admins).Error
	if err != nil {
		log.Error("admin user repo find all paginated with roles query failed", zap.Error(err))
		return nil, 0, err
	}
	log.Info("admin user repo find all paginated with roles success", zap.Int("count", len(admins)), zap.Int64("total", total))

	return admins, total, nil
}

// Update2FAStatus enables or disables 2FA for an admin user.
func (r *AdminUserRepository) Update2FAStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, enabled bool) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo update 2fa status start", zap.String("admin_id", id.String()), zap.Bool("enabled", enabled))

	if err := tx.WithContext(ctx).
		Model(&entity.AdminUser{}).
		Where("id = ?", id).
		Update("is_two_fa_enabled", enabled).
		Error; err != nil {
		log.Error("admin user repo update 2fa status failed", zap.Error(err))
		return err
	}
	log.Info("admin user repo update 2fa status success", zap.String("admin_id", id.String()))
	return nil
}

// EmailExists checks if an email is already registered.
// Used during registration to validate email uniqueness.
func (r *AdminUserRepository) EmailExists(ctx context.Context, tx *gorm.DB, email string) (bool, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("admin user repo email exists start")

	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.AdminUser{}).
		Where("email = ? AND deleted_at IS NULL", email).
		Count(&count).Error
	if err != nil {
		log.Error("admin user repo email exists failed", zap.Error(err))
		return false, err
	}
	log.Info("admin user repo email exists success", zap.Bool("exists", count > 0))
	return count > 0, nil
}
