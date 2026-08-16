package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IOrderRepository interface {
	IRepository[entity.Order]
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Order, error)
	FindByExternalID(ctx context.Context, tx *gorm.DB, externalID string) (*entity.Order, error)
	FindByExternalIDWithPackage(ctx context.Context, tx *gorm.DB, externalID string) (*entity.Order, error)
	FindBySubscriptionID(ctx context.Context, tx *gorm.DB, subscriptionID uuid.UUID) (*entity.Order, error)
	FindPendingOrderByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (*entity.Order, error)
	CountPaidByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (int64, error)
	CountPaidByUserAndPackages(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageIDs []uuid.UUID) (map[uuid.UUID]int64, error)
	FindExpiredPendingOrders(ctx context.Context, tx *gorm.DB, limit int) ([]entity.Order, error)
	FindRecentByTelegramUserID(ctx context.Context, tx *gorm.DB, telegramUserID int64, clientID uuid.UUID, limit int) ([]entity.Order, error)
	FindTransactionsByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.TransactionFilterRequest) ([]entity.Order, error)
	CountTransactionsByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.TransactionFilterRequest) (int64, error)
	AtomicUpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, fromStatus string, updates map[string]any) (int64, error)
	Create(ctx context.Context, tx *gorm.DB, order *entity.Order) error
	Update(ctx context.Context, tx *gorm.DB, order *entity.Order) error
}

type OrderRepository struct {
	Repository[entity.Order]
}

func NewOrderRepository() IOrderRepository {
	return &OrderRepository{}
}

func (r *OrderRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Order, error) {
	var order entity.Order
	err := tx.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindBySubscriptionID(ctx context.Context, tx *gorm.DB, subscriptionID uuid.UUID) (*entity.Order, error) {
	var order entity.Order
	err := tx.WithContext(ctx).Where("subscription_id = ? AND deleted_at IS NULL", subscriptionID).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
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

// CountPaidByUserAndPackage counts paid orders for a (member, package) pair —
// basis untuk batas pembelian paket per member (lifetime).
func (r *OrderRepository) CountPaidByUserAndPackage(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.Order{}).
		Where("telegram_user_id = ? AND package_id = ? AND status = ? AND deleted_at IS NULL", userID, packageID, "paid").
		Count(&count).Error
	return count, err
}

// CountPaidByUserAndPackages returns grouped paid-order counts per package for
// one member — satu query untuk daftar paket di bot.
func (r *OrderRepository) CountPaidByUserAndPackages(ctx context.Context, tx *gorm.DB, userID uuid.UUID, packageIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	counts := make(map[uuid.UUID]int64)
	if len(packageIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		PackageID uuid.UUID
		Count     int64
	}
	err := tx.WithContext(ctx).
		Model(&entity.Order{}).
		Select("package_id, COUNT(*) as count").
		Where("telegram_user_id = ? AND package_id IN ? AND status = ? AND deleted_at IS NULL", userID, packageIDs, "paid").
		Group("package_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.PackageID] = row.Count
	}
	return counts, nil
}

func (r *OrderRepository) FindExpiredPendingOrders(ctx context.Context, tx *gorm.DB, limit int) ([]entity.Order, error) {
	var orders []entity.Order
	err := tx.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL AND expired_at IS NOT NULL AND expired_at <= ?", "pending", time.Now()).
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

func (r *OrderRepository) FindRecentByTelegramUserID(ctx context.Context, tx *gorm.DB, telegramUserID int64, clientID uuid.UUID, limit int) ([]entity.Order, error) {
	var orders []entity.Order
	err := tx.WithContext(ctx).
		Joins("JOIN telegram_users tu ON orders.telegram_user_id = tu.id").
		Preload("Package").
		Where("tu.telegram_user_id = ?", telegramUserID).
		Where("orders.client_id = ?", clientID).
		Where("orders.deleted_at IS NULL").
		Order("orders.created_at DESC").
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

// ── Tenant Transaction queries ──────────────────────────────────────

// applyTransactionScope builds the base GORM query for tenant transaction
// listing/counting. It JOINs to packages and telegram_users, and applies
// status, search, package, payment_method, and date-range filters.
func (r *OrderRepository) applyTransactionScope(query *gorm.DB, clientID uuid.UUID, filter model.TransactionFilterRequest) *gorm.DB {
	q := query.Model(&entity.Order{}).
		Joins("JOIN packages ON packages.id = orders.package_id").
		Joins("JOIN telegram_users ON telegram_users.id = orders.telegram_user_id").
		Where("orders.client_id = ? AND orders.deleted_at IS NULL", clientID)

	// Status filter
	if filter.Status != "" && filter.Status != "all" {
		q = q.Where("orders.status = ?", filter.Status)
	}

	// Package filter
	if filter.PackageID != uuid.Nil {
		q = q.Where("orders.package_id = ?", filter.PackageID)
	}

	// Payment method filter
	if filter.PaymentMethod != "" {
		q = q.Where("orders.payment_method = ?", filter.PaymentMethod)
	}

	// Free-text search across member username, first_name, last_name, external_id
	if filter.Search != "" {
		search := "%" + escapeLike(filter.Search) + "%"
		q = q.Where("telegram_users.username ILIKE ? OR telegram_users.first_name ILIKE ? OR telegram_users.last_name ILIKE ? OR orders.external_id ILIKE ?", search, search, search, search)
	}

	// Date range filters (based on order created_at)
	if filter.DateFrom != nil {
		q = q.Where("orders.created_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		q = q.Where("orders.created_at <= ?", *filter.DateTo)
	}

	return q
}

// FindTransactionsByClientID returns paginated orders with preloaded Package, User, and Discount.
// ponytail: Preload("User") generates a broken query here — Order.TelegramUserID (uuid FK)
// collides with TelegramUser.TelegramUserID (bigint), so GORM filters the wrong column
// (`telegram_user_id IN (uuid...)`). Users are loaded manually, same as SubscriptionRepository.
func (r *OrderRepository) FindTransactionsByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.TransactionFilterRequest) ([]entity.Order, error) {
	var orders []entity.Order

	page := filter.Page
	limit := filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	// ponytail: page has no upper bound upstream (min=1 only); (page-1)*limit
	// overflows int for absurd pages (>= ~9.2e16 with limit=100) and GORM emits
	// a negative OFFSET -> Postgres error -> 500. Clamp instead: sane pages are
	// unaffected, garbage pages get an empty result instead of a crash.
	const maxPage = 1_000_000
	if page > maxPage {
		page = maxPage
	}
	offset := (page - 1) * limit

	query := r.applyTransactionScope(tx.WithContext(ctx), clientID, filter)
	query = query.
		Preload("Package", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		Preload("Discount", func(db *gorm.DB) *gorm.DB { return db.Unscoped() })

	err := query.
		Order("orders.created_at DESC").
		Offset(offset).Limit(limit).
		Find(&orders).Error
	if err != nil {
		return nil, err
	}

	if len(orders) > 0 {
		userIDs := make([]uuid.UUID, 0, len(orders))
		seen := make(map[uuid.UUID]struct{}, len(orders))
		for _, o := range orders {
			if _, ok := seen[o.TelegramUserID]; ok {
				continue
			}
			seen[o.TelegramUserID] = struct{}{}
			userIDs = append(userIDs, o.TelegramUserID)
		}

		var users []entity.TelegramUser
		if err := tx.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
		userMap := make(map[uuid.UUID]entity.TelegramUser, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}
		for i := range orders {
			orders[i].User = userMap[orders[i].TelegramUserID]
		}
	}

	return orders, nil
}

// CountTransactionsByClientID counts orders matching the transaction filter.
func (r *OrderRepository) CountTransactionsByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.TransactionFilterRequest) (int64, error) {
	var count int64
	query := r.applyTransactionScope(tx.WithContext(ctx), clientID, filter)
	err := query.Count(&count).Error
	return count, err
}

func (r *OrderRepository) FindByExternalIDWithPackage(ctx context.Context, tx *gorm.DB, externalID string) (*entity.Order, error) {
	var order entity.Order
	err := tx.WithContext(ctx).
		Preload("Package.Groups").
		Where("external_id = ? AND deleted_at IS NULL", externalID).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) AtomicUpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, fromStatus string, updates map[string]any) (int64, error) {
	result := tx.WithContext(ctx).
		Model(&entity.Order{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(updates)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
