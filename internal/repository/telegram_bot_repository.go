package repository

import (
	"context"
	"errors"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ITelegramBotRepository interface {
	IRepository[entity.TelegramBot]
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, page, limit int) ([]entity.TelegramBot, error)
	FindFirstByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (*entity.TelegramBot, error)
	FindByBotID(ctx context.Context, tx *gorm.DB, botID int64) (*entity.TelegramBot, error)
	FindByBotIDAndClientID(ctx context.Context, tx *gorm.DB, botID int64, clientID uuid.UUID) (*entity.TelegramBot, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.TelegramBot, error)
	Delete(ctx context.Context, tx *gorm.DB, bot *entity.TelegramBot) error
	CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
}

type TelegramBotRepository struct {
	Repository[entity.TelegramBot]
}

func NewTelegramBotRepository() ITelegramBotRepository {
	return &TelegramBotRepository{}
}

func (r *TelegramBotRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, page, limit int) ([]entity.TelegramBot, error) {
	var bots []entity.TelegramBot
	page, limit = clampPagination(page, limit)
	offset := (page - 1) * limit
	err := tx.WithContext(ctx).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Offset(offset).
		Limit(limit).
		Find(&bots).Error
	return bots, err
}

func (r *TelegramBotRepository) FindFirstByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (*entity.TelegramBot, error) {
	var bot entity.TelegramBot
	err := tx.WithContext(ctx).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Order("created_at ASC").
		First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &bot, nil
}

func (r *TelegramBotRepository) FindByBotID(ctx context.Context, tx *gorm.DB, botID int64) (*entity.TelegramBot, error) {
	var bot entity.TelegramBot
	err := tx.WithContext(ctx).Where("bot_id = ? AND deleted_at IS NULL", botID).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &bot, nil
}

func (r *TelegramBotRepository) FindByBotIDAndClientID(ctx context.Context, tx *gorm.DB, botID int64, clientID uuid.UUID) (*entity.TelegramBot, error) {
	var bot entity.TelegramBot
	err := tx.WithContext(ctx).Where("bot_id = ? AND client_id = ? AND deleted_at IS NULL", botID, clientID).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &bot, nil
}

func (r *TelegramBotRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.TelegramBot, error) {
	var bot entity.TelegramBot
	err := tx.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&bot).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &bot, nil
}

func (r *TelegramBotRepository) Delete(ctx context.Context, tx *gorm.DB, bot *entity.TelegramBot) error {
	return tx.WithContext(ctx).Delete(bot).Error
}

func (r *TelegramBotRepository) CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.TelegramBot{}).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Count(&count).Error
	return count, err
}
