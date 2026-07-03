package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ICustomCommandRepository interface {
	IRepository[entity.CustomCommand]
	FindByBotIDAndTrigger(ctx context.Context, tx *gorm.DB, botID uuid.UUID, trigger string) (*entity.CustomCommand, error)
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID, isActive *bool, page, limit int) ([]entity.CustomCommand, error)
	CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID, isActive *bool) (int64, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.CustomCommand, error)
}

type CustomCommandRepository struct {
	Repository[entity.CustomCommand]
}

func NewCustomCommandRepository() ICustomCommandRepository {
	return &CustomCommandRepository{}
}

func (r *CustomCommandRepository) FindByBotIDAndTrigger(ctx context.Context, tx *gorm.DB, botID uuid.UUID, trigger string) (*entity.CustomCommand, error) {
	var cmd entity.CustomCommand
	err := tx.WithContext(ctx).
		Where("bot_id = ? AND command_trigger = ? AND is_active = true AND deleted_at IS NULL", botID, trigger).
		First(&cmd).Error
	return &cmd, err
}

func (r *CustomCommandRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID, isActive *bool, page, limit int) ([]entity.CustomCommand, error) {
	var commands []entity.CustomCommand
	offset := (page - 1) * limit

	query := tx.WithContext(ctx).Where("client_id = ? AND deleted_at IS NULL", clientID)

	if botID != nil {
		query = query.Where("bot_id = ?", *botID)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	err := query.Preload("Bot").Offset(offset).Limit(limit).Find(&commands).Error
	return commands, err
}

func (r *CustomCommandRepository) CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID *uuid.UUID, isActive *bool) (int64, error) {
	var count int64

	query := tx.WithContext(ctx).Model(&entity.CustomCommand{}).Where("client_id = ? AND deleted_at IS NULL", clientID)

	if botID != nil {
		query = query.Where("bot_id = ?", *botID)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *CustomCommandRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.CustomCommand, error) {
	var cmd entity.CustomCommand
	err := tx.WithContext(ctx).Preload("Bot").Where("id = ? AND deleted_at IS NULL", id).First(&cmd).Error
	return &cmd, err
}
