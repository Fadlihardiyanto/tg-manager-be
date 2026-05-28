package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IOrderRepository interface {
	IRepository[entity.Order]
	FindByExternalID(ctx context.Context, tx *gorm.DB, externalID string) (*entity.Order, error)
	FindPendingOrderByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (*entity.Order, error)
	FindExpiredPendingOrders(ctx context.Context, tx *gorm.DB, limit int) ([]entity.Order, error)
	Create(ctx context.Context, tx *gorm.DB, order *entity.Order) error
	Update(ctx context.Context, tx *gorm.DB, order *entity.Order) error
}

type OrderRepository struct {
	Repository[entity.Order]
}

func NewOrderRepository() IOrderRepository {
	return &OrderRepository{}
}

func (r *OrderRepository) FindByExternalID(ctx context.Context, tx *gorm.DB, externalID string) (*entity.Order, error) {
	var order entity.Order
	err := tx.WithContext(ctx).Where("external_id = ? AND deleted_at IS NULL", externalID).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindPendingOrderByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (*entity.Order, error) {
	var order entity.Order
	err := tx.WithContext(ctx).
		Where("telegram_user_id = ? AND package_id = ? AND status = ? AND deleted_at IS NULL AND (expired_at IS NULL OR expired_at > ?)", userID, packageID, "pending", time.Now()).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindExpiredPendingOrders(ctx context.Context, tx *gorm.DB, limit int) ([]entity.Order, error) {
	var orders []entity.Order
	err := tx.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL AND expired_at IS NOT NULL AND expired_at <= ?", "pending", time.Now()).
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) Create(ctx context.Context, tx *gorm.DB, order *entity.Order) error {
	return tx.WithContext(ctx).Create(order).Error
}

func (r *OrderRepository) Update(ctx context.Context, tx *gorm.DB, order *entity.Order) error {
	return tx.WithContext(ctx).Save(order).Error
}
