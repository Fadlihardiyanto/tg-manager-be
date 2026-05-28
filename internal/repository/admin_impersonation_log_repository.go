package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IAdminImpersonationLogRepository handles persistence for admin_impersonation_logs table.
// Every impersonation session is recorded for security audit and compliance.
type IAdminImpersonationLogRepository interface {
	// Create logs a new impersonation session (started_at is set automatically)
	Create(ctx context.Context, tx *gorm.DB, log *entity.AdminImpersonationLog) error

	// EndSession marks an impersonation session as ended (sets ended_at)
	EndSession(ctx context.Context, tx *gorm.DB, id uuid.UUID) error

	// FindByAdminID returns impersonation history for a specific admin (newest first)
	FindByAdminID(ctx context.Context, tx *gorm.DB, adminUserID uuid.UUID, offset, limit int) ([]entity.AdminImpersonationLog, int64, error)

	// FindByClientID returns impersonation history for a specific client (newest first)
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, offset, limit int) ([]entity.AdminImpersonationLog, int64, error)

	// FindActiveSessions returns sessions that haven't ended yet
	FindActiveSessions(ctx context.Context, tx *gorm.DB, adminUserID uuid.UUID) ([]entity.AdminImpersonationLog, error)
}

// =============================================================================
// Implementation
// =============================================================================

type AdminImpersonationLogRepository struct{}

func NewAdminImpersonationLogRepository() IAdminImpersonationLogRepository {
	return &AdminImpersonationLogRepository{}
}

func (r *AdminImpersonationLogRepository) Create(ctx context.Context, tx *gorm.DB, log *entity.AdminImpersonationLog) error {
	return tx.WithContext(ctx).Create(log).Error
}

func (r *AdminImpersonationLogRepository) EndSession(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	return tx.WithContext(ctx).
		Model(&entity.AdminImpersonationLog{}).
		Where("id = ? AND ended_at IS NULL", id).
		Update("ended_at", gorm.Expr("CURRENT_TIMESTAMP")).
		Error
}

func (r *AdminImpersonationLogRepository) FindByAdminID(ctx context.Context, tx *gorm.DB, adminUserID uuid.UUID, offset, limit int) ([]entity.AdminImpersonationLog, int64, error) {
	var logs []entity.AdminImpersonationLog
	var total int64

	db := tx.WithContext(ctx).Model(&entity.AdminImpersonationLog{}).
		Where("admin_user_id = ?", adminUserID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.
		Offset(offset).
		Limit(limit).
		Order("started_at DESC").
		Find(&logs).Error

	return logs, total, err
}

func (r *AdminImpersonationLogRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, offset, limit int) ([]entity.AdminImpersonationLog, int64, error) {
	var logs []entity.AdminImpersonationLog
	var total int64

	db := tx.WithContext(ctx).Model(&entity.AdminImpersonationLog{}).
		Where("client_id = ?", clientID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := db.
		Offset(offset).
		Limit(limit).
		Order("started_at DESC").
		Find(&logs).Error

	return logs, total, err
}

func (r *AdminImpersonationLogRepository) FindActiveSessions(ctx context.Context, tx *gorm.DB, adminUserID uuid.UUID) ([]entity.AdminImpersonationLog, error) {
	var logs []entity.AdminImpersonationLog
	err := tx.WithContext(ctx).
		Where("admin_user_id = ? AND ended_at IS NULL", adminUserID).
		Order("started_at DESC").
		Find(&logs).Error
	return logs, err
}
