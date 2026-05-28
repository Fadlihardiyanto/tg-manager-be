package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"gorm.io/gorm"
)

type ITelegramUserRepository interface {
	IRepository[entity.TelegramUser]
	FindByTelegramID(ctx context.Context, tx *gorm.DB, telegramID int64) (*entity.TelegramUser, error)
}

type TelegramUserRepository struct {
	Repository[entity.TelegramUser]
}

func NewTelegramUserRepository() ITelegramUserRepository {
	return &TelegramUserRepository{}
}

func (r *TelegramUserRepository) FindByTelegramID(ctx context.Context, tx *gorm.DB, telegramID int64) (*entity.TelegramUser, error) {
	var user entity.TelegramUser
	err := tx.WithContext(ctx).Where("telegram_user_id = ?", telegramID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
