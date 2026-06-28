package repository

import (
	"context"
	"errors"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ITelegramUserRepository interface {
	IRepository[entity.TelegramUser]
	FindByTelegramID(ctx context.Context, tx *gorm.DB, telegramID int64) (*entity.TelegramUser, error)
	FindMembersByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MemberFilterRequest) ([]model.AggregatedMemberRow, error)
	CountMembersByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MemberFilterRequest) (int64, error)
	FindMemberDetailByID(ctx context.Context, tx *gorm.DB, userID uuid.UUID, clientID uuid.UUID) (*entity.TelegramUser, error)
	CountOrdersByUserIDs(ctx context.Context, tx *gorm.DB, userIDs []uuid.UUID, clientID uuid.UUID) (map[uuid.UUID]int64, error)
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

// ── Member queries ───────────────────────────────────────────────────

// applyMemberScope is a shared helper that builds the base GORM query for
// member listing/counting. It applies status, search, and package filters
// as EXISTS sub-queries so that pagination operates on distinct users.
func (r *TelegramUserRepository) applyMemberScope(query *gorm.DB, clientID uuid.UUID, filter model.MemberFilterRequest) *gorm.DB {
	q := query.Model(&entity.TelegramUser{}).
		Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.client_id = ? AND s.deleted_at IS NULL)", clientID).
		Where("telegram_users.deleted_at IS NULL")

	// Status filter — restrict to users that have at least one subscription
	// matching the requested status. "all" (or empty) means no restriction.
	switch filter.Status {
	case "active":
		q = q.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.client_id = ? AND s.status = 'active' AND s.deleted_at IS NULL)", clientID)
	case "expired":
		q = q.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.client_id = ? AND s.status = 'expired' AND s.deleted_at IS NULL)", clientID)
	}

	// Free-text search across username, first_name, last_name
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		q = q.Where("telegram_users.username ILIKE ? OR telegram_users.first_name ILIKE ? OR telegram_users.last_name ILIKE ?", search, search, search)
	}

	// Package filter — only users subscribed to a specific package
	if filter.PackageID != uuid.Nil {
		q = q.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.package_id = ? AND s.deleted_at IS NULL)", filter.PackageID)
	}

	if filter.JoinedStart != nil && filter.JoinedEnd != nil {
		q = q.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.client_id = ? AND s.created_at BETWEEN ? AND ? AND s.deleted_at IS NULL)", clientID, filter.JoinedStart, filter.JoinedEnd)
	}

	if filter.ExpiredStart != nil && filter.ExpiredEnd != nil {
		q = q.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.client_id = ? AND s.expired_at BETWEEN ? AND ? AND s.deleted_at IS NULL)", clientID, filter.ExpiredStart, filter.ExpiredEnd)
	}

	if filter.NearestExpiryStart != nil && filter.NearestExpiryEnd != nil {
		q = q.Where("EXISTS (SELECT 1 FROM (SELECT MIN(s.expired_at) as min_exp FROM subscriptions s WHERE s.telegram_user_id = telegram_users.id AND s.client_id = ? AND s.status = 'active' AND s.deleted_at IS NULL) sub WHERE sub.min_exp BETWEEN ? AND ?)", clientID, filter.NearestExpiryStart, filter.NearestExpiryEnd)
	}

	return q
}

// preloadMemberSubscriptions adds the subscription preload (with Package name)
// scoped to the given client.
func preloadMemberSubscriptions(query *gorm.DB, clientID uuid.UUID) *gorm.DB {
	return query.
		Preload("Subscriptions", func(db *gorm.DB) *gorm.DB {
			return db.Where("client_id = ? AND deleted_at IS NULL", clientID).
				Order("CASE WHEN status = 'active' THEN 0 ELSE 1 END, expired_at DESC")
		}).
		Preload("Subscriptions.Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		})
}

// FindMembersByClientID returns paginated telegram users with aggregated subscriptions.
func (r *TelegramUserRepository) FindMembersByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MemberFilterRequest) ([]model.AggregatedMemberRow, error) {
	var users []model.AggregatedMemberRow

	page := filter.Page
	limit := filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := r.applyMemberScope(tx.WithContext(ctx), clientID, filter)

	err := query.
		Select(`
			telegram_users.id,
			telegram_users.telegram_user_id,
			telegram_users.username,
			telegram_users.first_name,
			telegram_users.last_name,
			telegram_users.phone,
			telegram_users.created_at,
			bool_or(s.status = 'active' AND s.deleted_at IS NULL) as global_status,
			COALESCE(json_agg(p.name) FILTER (WHERE s.status = 'active' AND s.deleted_at IS NULL), '[]') as active_packages,
			MIN(s.expired_at) FILTER (WHERE s.status = 'active' AND s.deleted_at IS NULL) as nearest_expiry
		`).
		Joins("LEFT JOIN subscriptions s ON s.telegram_user_id = telegram_users.id AND s.client_id = ?", clientID).
		Joins("LEFT JOIN packages p ON p.id = s.package_id").
		Group("telegram_users.id").
		Order("telegram_users.created_at DESC").
		Offset(offset).Limit(limit).
		Find(&users).Error

	return users, err
}

// CountMembersByClientID counts distinct telegram users matching the filter.
func (r *TelegramUserRepository) CountMembersByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MemberFilterRequest) (int64, error) {
	var count int64
	query := r.applyMemberScope(tx.WithContext(ctx), clientID, filter)
	err := query.Count(&count).Error
	return count, err
}

// FindMemberDetailByID fetches a single telegram user with ALL subscription
// history for the tenant's client. Returns (nil, nil) when not found.
func (r *TelegramUserRepository) FindMemberDetailByID(ctx context.Context, tx *gorm.DB, userID uuid.UUID, clientID uuid.UUID) (*entity.TelegramUser, error) {
	var user entity.TelegramUser

	err := tx.WithContext(ctx).
		Preload("Subscriptions", func(db *gorm.DB) *gorm.DB {
			return db.Where("client_id = ? AND deleted_at IS NULL", clientID).
				Order("CASE WHEN status = 'active' THEN 0 ELSE 1 END, expired_at DESC")
		}).
		Preload("Subscriptions.Package", func(db *gorm.DB) *gorm.DB {
			return db.Unscoped()
		}).
		Where("id = ? AND deleted_at IS NULL", userID).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

// CountOrdersByUserIDs returns a map[telegramUserUUID]→paidOrderCount.
func (r *TelegramUserRepository) CountOrdersByUserIDs(ctx context.Context, tx *gorm.DB, userIDs []uuid.UUID, clientID uuid.UUID) (map[uuid.UUID]int64, error) {
	result := make(map[uuid.UUID]int64, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	type row struct {
		TelegramUserID uuid.UUID
		Count          int64
	}

	var rows []row
	err := tx.WithContext(ctx).
		Model(&entity.Order{}).
		Select("telegram_user_id, COUNT(*) as count").
		Where("telegram_user_id IN ? AND client_id = ? AND status = 'paid' AND deleted_at IS NULL", userIDs, clientID).
		Group("telegram_user_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.TelegramUserID] = r.Count
	}

	return result, nil
}
