package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IBroadcastRepository interface {
	IRepository[entity.Broadcast]
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID, page, limit int) ([]entity.Broadcast, error)
	CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID) (int64, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Broadcast, error)
	IncrementCounters(ctx context.Context, tx *gorm.DB, id uuid.UUID, success bool) (*entity.Broadcast, error)
}

type BroadcastRepository struct {
	Repository[entity.Broadcast]
}

func NewBroadcastRepository() IBroadcastRepository {
	return &BroadcastRepository{}
}

func (r *BroadcastRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID, page, limit int) ([]entity.Broadcast, error) {
	var broadcasts []entity.Broadcast
	offset := (page - 1) * limit

	query := tx.WithContext(ctx).Where("client_id = ? AND deleted_at IS NULL", clientID)

	if botID != nil {
		query = query.Where("bot_id = ?", *botID)
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&broadcasts).Error
	return broadcasts, err
}

func (r *BroadcastRepository) CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID) (int64, error) {
	var count int64

	query := tx.WithContext(ctx).Model(&entity.Broadcast{}).Where("client_id = ? AND deleted_at IS NULL", clientID)

	if botID != nil {
		query = query.Where("bot_id = ?", *botID)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *BroadcastRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Broadcast, error) {
	var b entity.Broadcast
	err := tx.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&b).Error
	return &b, err
}

func (r *BroadcastRepository) IncrementCounters(ctx context.Context, tx *gorm.DB, id uuid.UUID, success bool) (*entity.Broadcast, error) {
	var b entity.Broadcast
	query := tx.WithContext(ctx).Model(&b).Where("id = ?", id)

	var err error
	if success {
		err = query.UpdateColumns(map[string]interface{}{
			"sent_count": gorm.Expr("sent_count + ?", 1),
			"updated_at": gorm.Expr("NOW()"),
		}).Error
	} else {
		err = query.UpdateColumns(map[string]interface{}{
			"failed_count": gorm.Expr("failed_count + ?", 1),
			"updated_at":   gorm.Expr("NOW()"),
		}).Error
	}

	if err != nil {
		return nil, err
	}

	// Fetch updated values
	err = tx.WithContext(ctx).Where("id = ?", id).First(&b).Error
	if err != nil {
		return nil, err
	}

	// If finished all targets, update status
	if b.SentCount+b.FailedCount >= b.TotalTargets && b.Status == "processing" {
		b.Status = "completed"
		err = tx.WithContext(ctx).Model(&b).Update("status", "completed").Error
		if err != nil {
			return nil, err
		}
	}

	return &b, nil
}
