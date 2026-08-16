package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IAuditLogRepository interface {
	IRepository[entity.AuditLog]
	FindByEntity(ctx context.Context, tx *gorm.DB, entityType string, entityID uuid.UUID) ([]entity.AuditLog, error)
	FindAllByClient(ctx context.Context, db *gorm.DB, clientID uuid.UUID, limit, offset int) ([]entity.AuditLog, int64, error)
	FindAllPlatform(ctx context.Context, db *gorm.DB, limit, offset int, action, resource string) ([]entity.AuditLog, int64, error)
}

type AuditLogRepository struct {
	Repository[entity.AuditLog]
}

func NewAuditLogRepository() IAuditLogRepository {
	return &AuditLogRepository{}
}

func (r *AuditLogRepository) FindByEntity(ctx context.Context, tx *gorm.DB, entityType string, entityID uuid.UUID) ([]entity.AuditLog, error) {
	var logs []entity.AuditLog
	err := tx.WithContext(ctx).Where("entity_type = ? AND entity_id = ?", entityType, entityID).Order("created_at DESC").Find(&logs).Error
	return logs, err
}

func (r *AuditLogRepository) FindAllByClient(ctx context.Context, db *gorm.DB, clientID uuid.UUID, limit, offset int) ([]entity.AuditLog, int64, error) {
	var logs []entity.AuditLog
	var total int64
	offset, limit = clampOffsetLimit(offset, limit)

	query := db.WithContext(ctx).Model(&entity.AuditLog{}).Where("client_id = ?", clientID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func (r *AuditLogRepository) FindAllPlatform(ctx context.Context, db *gorm.DB, limit, offset int, action, resource string) ([]entity.AuditLog, int64, error) {
	var logs []entity.AuditLog
	var total int64
	offset, limit = clampOffsetLimit(offset, limit)

	query := db.WithContext(ctx).Model(&entity.AuditLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("entity_type = ?", resource)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}
