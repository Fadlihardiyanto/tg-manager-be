package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ITelegramGroupRepository interface {
	IRepository[entity.Group]
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]entity.Group, error)
	FindByBotID(ctx context.Context, tx *gorm.DB, botID uuid.UUID) ([]entity.Group, error)
	FindByTelegramID(ctx context.Context, tx *gorm.DB, telegramID int64) (*entity.Group, error)
	FindAllActive(ctx context.Context, tx *gorm.DB) ([]entity.Group, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Group, error)
	Delete(ctx context.Context, tx *gorm.DB, group *entity.Group) error
}

type TelegramGroupRepository struct {
	Repository[entity.Group]
}

func NewTelegramGroupRepository() ITelegramGroupRepository {
	return &TelegramGroupRepository{}
}

func (r *TelegramGroupRepository) FindByBotID(ctx context.Context, tx *gorm.DB, botID uuid.UUID) ([]entity.Group, error) {
	var groups []entity.Group
	err := tx.WithContext(ctx).Where("bot_id = ? AND deleted_at IS NULL", botID).Find(&groups).Error
	return groups, err
}

func (r *TelegramGroupRepository) FindByTelegramID(ctx context.Context, tx *gorm.DB, telegramID int64) (*entity.Group, error) {
	var group entity.Group
	err := tx.WithContext(ctx).Where("telegram_chat_id = ? AND deleted_at IS NULL", telegramID).First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *TelegramGroupRepository) FindAllActive(ctx context.Context, tx *gorm.DB) ([]entity.Group, error) {
	var groups []entity.Group
	err := tx.WithContext(ctx).Where("is_active = ? AND deleted_at IS NULL", true).Find(&groups).Error
	return groups, err
}

func (r *TelegramGroupRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]entity.Group, error) {
	var groups []entity.Group
	err := tx.WithContext(ctx).Where("client_id = ? AND deleted_at IS NULL", clientID).Find(&groups).Error
	return groups, err
}

func (r *TelegramGroupRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Group, error) {
	var group entity.Group
	err := tx.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&group).Error
	return &group, err
}

func (r *TelegramGroupRepository) Delete(ctx context.Context, tx *gorm.DB, group *entity.Group) error {
	return tx.WithContext(ctx).Delete(group).Error
}
