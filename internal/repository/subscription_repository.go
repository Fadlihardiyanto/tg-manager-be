package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ISubscriptionRepository interface {
	IRepository[entity.Subscription]
	HasActiveSubscriptionForGroup(ctx context.Context, tx *gorm.DB, telegramUserID int64, telegramChatID int64) (bool, error)
	FindByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (*entity.Subscription, error)
	FindActiveAllAccessByUserAndClient(ctx context.Context, tx *gorm.DB, userID uuid.UUID, clientID uuid.UUID) (*entity.Subscription, error)
	FindExpiredSubscriptions(ctx context.Context, tx *gorm.DB, limit int) ([]entity.Subscription, error)
	FindExpiringSoon(ctx context.Context, tx *gorm.DB, withinHours int, limit int) ([]entity.Subscription, error)
	FindActiveByTelegramUserID(ctx context.Context, tx *gorm.DB, telegramUserID int64, clientID uuid.UUID) ([]entity.Subscription, error)
	FindActiveByIdWithPackage(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Subscription, error)
	Create(ctx context.Context, tx *gorm.DB, subscription *entity.Subscription) error
	Update(ctx context.Context, tx *gorm.DB, subscription *entity.Subscription) error
	CountActiveUniqueUsersByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
}

type SubscriptionRepository struct {
	Repository[entity.Subscription]
}

func NewSubscriptionRepository() ISubscriptionRepository {
	return &SubscriptionRepository{}
}

func (r *SubscriptionRepository) FindByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (*entity.Subscription, error) {
	var subscription entity.Subscription
	err := tx.WithContext(ctx).
		Where("telegram_user_id = ? AND package_id = ? AND status = ? AND deleted_at IS NULL", userID, packageID, "active").
		First(&subscription).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &subscription, nil
}

func (r *SubscriptionRepository) FindActiveAllAccessByUserAndClient(ctx context.Context, tx *gorm.DB, userID uuid.UUID, clientID uuid.UUID) (*entity.Subscription, error) {
	var subscription entity.Subscription
	err := tx.WithContext(ctx).
		Joins("JOIN packages ON subscriptions.package_id = packages.id").
		Where("subscriptions.telegram_user_id = ? AND packages.client_id = ? AND packages.is_all_access = true AND subscriptions.status = ? AND subscriptions.deleted_at IS NULL", userID, clientID, "active").
		Preload("Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		First(&subscription).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &subscription, nil
}

func (r *SubscriptionRepository) FindExpiredSubscriptions(ctx context.Context, tx *gorm.DB, limit int) ([]entity.Subscription, error) {
	var subscriptions []entity.Subscription
	query := tx.WithContext(ctx).
		Preload("Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("Package.Groups").
		Preload("User").
		Where("status = ? AND expired_at < ? AND deleted_at IS NULL", "active", gorm.Expr("CURRENT_TIMESTAMP")).
		Order("expired_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&subscriptions).Error
	return subscriptions, err
}

// FindExpiringSoon returns active subscriptions expiring within the next `withinHours` hours
// that have NOT yet had a reminder outbox event published for that exact window.
// Deduplication is handled by checking the outbox table for an existing
// (aggregate_id, event_type) pair, so the same reminder is never sent twice.
func (r *SubscriptionRepository) FindExpiringSoon(ctx context.Context, tx *gorm.DB, withinHours int, limit int) ([]entity.Subscription, error) {
	var subscriptions []entity.Subscription

	eventType := fmt.Sprintf("expiry.reminder_%dh", withinHours)

	query := tx.WithContext(ctx).
		Preload("Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("Package.Groups").
		Preload("User").
		Where(
			`status = 'active'
			AND deleted_at IS NULL
			AND expired_at > CURRENT_TIMESTAMP
			AND expired_at <= CURRENT_TIMESTAMP + (? * INTERVAL '1 hour')
			AND id NOT IN (
				SELECT aggregate_id FROM outbox
				WHERE event_type = ? AND status != 'failed'
			)`,
			withinHours, eventType,
		).
		Order("expired_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&subscriptions).Error
	return subscriptions, err
}

// FindActiveByTelegramUserID fetches all active subscriptions for a given Telegram user
// scoped to a specific client (tenant), with Package + Package.Groups preloaded.
func (r *SubscriptionRepository) FindActiveByTelegramUserID(ctx context.Context, tx *gorm.DB, telegramUserID int64, clientID uuid.UUID) ([]entity.Subscription, error) {
	var subscriptions []entity.Subscription
	err := tx.WithContext(ctx).
		Joins("JOIN telegram_users tu ON subscriptions.telegram_user_id = tu.id").
		Preload("Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("Package.Groups").
		Where("tu.telegram_user_id = ?", telegramUserID).
		Where("subscriptions.client_id = ?", clientID).
		Where("subscriptions.status = ?", "active").
		Where("subscriptions.deleted_at IS NULL").
		Order("subscriptions.expired_at ASC").
		Find(&subscriptions).Error
	return subscriptions, err
}

// FindActiveByIdWithPackage fetches a single active subscription by its ID
// with Package + Package.Groups preloaded.
func (r *SubscriptionRepository) FindActiveByIdWithPackage(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Subscription, error) {
	var sub entity.Subscription
	err := tx.WithContext(ctx).
		Preload("Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Preload("Package.Groups").
		Where("id = ? AND status = 'active' AND deleted_at IS NULL", id).
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) Create(ctx context.Context, tx *gorm.DB, subscription *entity.Subscription) error {
	return tx.WithContext(ctx).Create(subscription).Error
}

func (r *SubscriptionRepository) Update(ctx context.Context, tx *gorm.DB, subscription *entity.Subscription) error {
	return tx.WithContext(ctx).Save(subscription).Error
}

func (r *SubscriptionRepository) HasActiveSubscriptionForGroup(ctx context.Context, tx *gorm.DB, telegramUserID int64, telegramChatID int64) (bool, error) {
	var count int64
	err := tx.WithContext(ctx).
		Table("subscriptions s").
		Joins("JOIN telegram_users tu ON s.telegram_user_id = tu.id").
		Joins("JOIN packages p ON s.package_id = p.id").
		Where("tu.telegram_user_id = ?", telegramUserID).
		Where("s.status = ?", "active").
		Where("s.deleted_at IS NULL").
		Where(`
			(p.is_all_access = true AND p.client_id = (
				SELECT client_id FROM groups
				WHERE telegram_chat_id = ? AND deleted_at IS NULL
				LIMIT 1
			))
			OR EXISTS (
				SELECT 1 FROM package_groups pg
				JOIN groups g ON pg.group_id = g.id
				WHERE pg.package_id = p.id AND g.telegram_chat_id = ? AND g.deleted_at IS NULL
			)
		`, telegramChatID, telegramChatID).
		Count(&count).Error
	return count > 0, err
}

func (r *SubscriptionRepository) CountActiveUniqueUsersByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.Subscription{}).
		Where("client_id = ? AND status = 'active' AND deleted_at IS NULL", clientID).
		Distinct("telegram_user_id").
		Count(&count).Error
	return count, err
}
