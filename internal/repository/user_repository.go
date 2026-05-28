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

// IUserRepository handles persistence for users table.
// Extends the generic IRepository with user-specific query methods.
type IUserRepository interface {
	IRepository[entity.User]

	FindByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.User, error)
	FindByEmailWithClient(ctx context.Context, tx *gorm.DB, email string) (*entity.User, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.User, error)
	EmailExists(ctx context.Context, tx *gorm.DB, email string) (bool, error)
	SoftDelete(ctx context.Context, tx *gorm.DB, id uuid.UUID) error
}

// =============================================================================
// Implementation
// =============================================================================

type UserRepository struct {
	Repository[entity.User]
	log *zap.Logger
}

func NewUserRepository(log *zap.Logger) IUserRepository {
	return &UserRepository{log: log}
}

func (r *UserRepository) FindByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("user repo find by email start")

	var user entity.User
	err := tx.WithContext(ctx).
		Where("email = ? AND deleted_at IS NULL", email).
		Take(&user).Error
	if err != nil {
		log.Error("user repo find by email failed", zap.Error(err))
		return nil, err
	}
	log.Info("user repo find by email success", zap.String("user_id", user.ID.String()))
	return &user, nil
}

func (r *UserRepository) FindByEmailWithClient(ctx context.Context, tx *gorm.DB, email string) (*entity.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("user repo find by email with client start")

	var user entity.User
	err := tx.WithContext(ctx).
		Preload("ClientUsers", "is_active = ? AND deleted_at IS NULL", true).
		Preload("ClientUsers.Client").
		Where("email = ? AND deleted_at IS NULL", email).
		Take(&user).Error
	if err != nil {
		log.Error("user repo find by email with client failed", zap.Error(err))
		return nil, err
	}
	log.Info("user repo find by email with client success", zap.String("user_id", user.ID.String()))
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.User, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("user repo find by id start", zap.String("user_id", id.String()))

	var user entity.User
	err := tx.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&user).Error
	if err != nil {
		log.Error("user repo find by id failed", zap.Error(err))
		return nil, err
	}
	log.Info("user repo find by id success", zap.String("user_id", user.ID.String()))
	return &user, nil
}

func (r *UserRepository) EmailExists(ctx context.Context, tx *gorm.DB, email string) (bool, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("user repo email exists start")

	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.User{}).
		Where("email = ? AND deleted_at IS NULL", email).
		Count(&count).Error
	if err != nil {
		log.Error("user repo email exists failed", zap.Error(err))
		return false, err
	}
	log.Info("user repo email exists success", zap.Bool("exists", count > 0))
	return count > 0, nil
}

func (r *UserRepository) SoftDelete(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("user repo soft delete start", zap.String("user_id", id.String()))

	now := time.Now()
	if err := tx.WithContext(ctx).
		Model(&entity.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		log.Error("user repo soft delete failed", zap.Error(err))
		return err
	}

	log.Info("user repo soft delete success", zap.String("user_id", id.String()))
	return nil
}
