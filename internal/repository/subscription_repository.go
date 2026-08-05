package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	CountActiveMembersByBotID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID uuid.UUID) (int64, error)
	FindActiveMemberIDsByBotID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID uuid.UUID, groupIDs []uuid.UUID) ([]int64, error)
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
		Preload("Package", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
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
		Where("status = ? AND expired_at < ? AND deleted_at IS NULL", "active", gorm.Expr("CURRENT_TIMESTAMP")).
		Order("expired_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&subscriptions).Error; err != nil {
		return nil, err
	}

	if len(subscriptions) > 0 {
		userIDs := make([]uuid.UUID, len(subscriptions))
		for i, s := range subscriptions {
			userIDs[i] = s.TelegramUserID
		}
		var users []entity.TelegramUser
		if err := tx.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
		userMap := make(map[uuid.UUID]entity.TelegramUser, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}
		for i := range subscriptions {
			subscriptions[i].User = userMap[subscriptions[i].TelegramUserID]
		}
	}

	return subscriptions, nil
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

	if err := query.Find(&subscriptions).Error; err != nil {
		return nil, err
	}

	if len(subscriptions) == 0 {
		return subscriptions, nil
	}

	// Preload User manually — the Belongs-To GORM tag on Subscription has a naming
	// collision (TelegramUserID exists in both tables with different types), so
	// Preload("User") generates a broken query.
	userIDs := make([]uuid.UUID, len(subscriptions))
	for i, s := range subscriptions {
		userIDs[i] = s.TelegramUserID
	}

	var users []entity.TelegramUser
	if err := tx.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}

	userMap := make(map[uuid.UUID]entity.TelegramUser, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	for i := range subscriptions {
		subscriptions[i].User = userMap[subscriptions[i].TelegramUserID]
	}

	return subscriptions, nil
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

func (r *SubscriptionRepository) CountActiveMembersByBotID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Table("subscriptions s").
		Select("COUNT(DISTINCT s.telegram_user_id)").
		Joins("JOIN packages p ON s.package_id = p.id").
		Where("s.status = ? AND s.expired_at > CURRENT_TIMESTAMP AND s.deleted_at IS NULL", "active").
		Where(`
			(p.is_all_access = true AND p.client_id = ?)
			OR EXISTS (
				SELECT 1 FROM package_groups pg
				JOIN groups g ON pg.group_id = g.id
				WHERE pg.package_id = p.id AND g.bot_id = ? AND g.is_active = true AND g.deleted_at IS NULL
			)
		`, clientID, botID).
		Count(&count).Error
	return count, err
}

func (r *SubscriptionRepository) FindActiveMemberIDsByBotID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, botID uuid.UUID, groupIDs []uuid.UUID) ([]int64, error) {
	var ids []int64

	query := tx.WithContext(ctx).
		Table("subscriptions s").
		Select("DISTINCT tu.telegram_user_id").
		Joins("JOIN telegram_users tu ON s.telegram_user_id = tu.id").
		Joins("JOIN packages p ON s.package_id = p.id").
		Where("s.status = 'active' AND s.expired_at > ? AND s.deleted_at IS NULL", time.Now())

	if len(groupIDs) > 0 {
		idStrs := make([]string, 0, len(groupIDs))
		for _, id := range groupIDs {
			idStrs = append(idStrs, "'"+id.String()+"'")
		}
		query = query.Where(`
			(p.is_all_access = true AND p.client_id = ?)
			OR EXISTS (
				SELECT 1 FROM package_groups pg
				JOIN groups g ON pg.group_id = g.id
				WHERE pg.package_id = p.id AND g.id IN (`+strings.Join(idStrs, ",")+`) AND g.is_active = true AND g.deleted_at IS NULL
			)
		`, clientID)
	} else {
		query = query.Where(`
			(p.is_all_access = true AND p.client_id = ?)
			OR EXISTS (
				SELECT 1 FROM package_groups pg
				JOIN groups g ON pg.group_id = g.id
				WHERE pg.package_id = p.id AND g.bot_id = ? AND g.is_active = true AND g.deleted_at IS NULL
			)
		`, clientID, botID)
	}

	err := query.Pluck("telegram_user_id", &ids).Error
	return ids, err
}
