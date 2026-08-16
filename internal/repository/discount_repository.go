package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type IPlatformDiscountRepository interface {
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.PlatformDiscount, error)
	FindByCode(ctx context.Context, db *gorm.DB, code string) (*entity.PlatformDiscount, error)
	FindAll(ctx context.Context, db *gorm.DB, onlyActive bool) ([]entity.PlatformDiscount, error)
	FindAutoApplicable(ctx context.Context, db *gorm.DB, clientID, planID uuid.UUID, amount decimal.Decimal) (*entity.PlatformDiscount, error)
	Create(ctx context.Context, db *gorm.DB, d *entity.PlatformDiscount) error
	Update(ctx context.Context, db *gorm.DB, d *entity.PlatformDiscount) error
	IncrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) (bool, error)
	DecrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) error
	SoftDelete(ctx context.Context, db *gorm.DB, id uuid.UUID) error
}

type platformDiscountRepository struct{}

func NewPlatformDiscountRepository() IPlatformDiscountRepository {
	return &platformDiscountRepository{}
}

func (r *platformDiscountRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.PlatformDiscount, error) {
	var d entity.PlatformDiscount
	err := db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *platformDiscountRepository) FindByCode(ctx context.Context, db *gorm.DB, code string) (*entity.PlatformDiscount, error) {
	var d entity.PlatformDiscount
	err := db.WithContext(ctx).
		Where("code = ? AND is_active = true AND deleted_at IS NULL", code).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *platformDiscountRepository) FindAll(ctx context.Context, db *gorm.DB, onlyActive bool) ([]entity.PlatformDiscount, error) {
	var discounts []entity.PlatformDiscount
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Order("created_at DESC")
	if onlyActive {
		now := time.Now()
		q = q.Where("is_active = true AND valid_from <= ? AND (valid_until IS NULL OR valid_until >= ?)", now, now)
	}
	err := q.Find(&discounts).Error
	return discounts, err
}

// FindAutoApplicable mencari diskon otomatis (tanpa kode) yang berlaku untuk client & plan tertentu
func (r *platformDiscountRepository) FindAutoApplicable(ctx context.Context, db *gorm.DB, clientID, planID uuid.UUID, amount decimal.Decimal) (*entity.PlatformDiscount, error) {
	now := time.Now()
	var d entity.PlatformDiscount
	err := db.WithContext(ctx).
		Where(`
			code IS NULL
			AND is_active = true
			AND deleted_at IS NULL
			AND valid_from <= ?
			AND (valid_until IS NULL OR valid_until >= ?)
			AND (max_usage = -1 OR used_count < max_usage)
			AND min_purchase <= ?
			AND (applicable_client_ids IS NULL OR ? = ANY(applicable_client_ids))
			AND (applicable_plan_ids IS NULL OR ? = ANY(applicable_plan_ids))
		`, now, now, amount, clientID.String(), planID.String()).
		Order("value DESC"). // ambil diskon terbesar
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *platformDiscountRepository) Create(ctx context.Context, db *gorm.DB, d *entity.PlatformDiscount) error {
	return db.WithContext(ctx).Create(d).Error
}

func (r *platformDiscountRepository) Update(ctx context.Context, db *gorm.DB, d *entity.PlatformDiscount) error {
	return db.WithContext(ctx).Save(d).Error
}

func (r *platformDiscountRepository) IncrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) (bool, error) {
	// Ceiling atomik: check kuota di snapshot (ApplyByCode/ApplyAuto) bukan
	// atomic — dua checkout konkuren bisa oversold. Guard di WHERE supaya
	// increment tidak pernah melewati max_usage (max_usage = -1 = unlimited).
	// Return (rows>0) agar caller bisa menolak checkout saat kuota habis.
	result := db.WithContext(ctx).Model(&entity.PlatformDiscount{}).
		Where("id = ? AND (max_usage = -1 OR used_count < max_usage)", id).
		Updates(map[string]any{
			"used_count": gorm.Expr("used_count + 1"),
			"updated_at": time.Now(),
		})
	return result.RowsAffected > 0, result.Error
}

func (r *platformDiscountRepository) DecrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	return db.WithContext(ctx).Model(&entity.PlatformDiscount{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_count": gorm.Expr("GREATEST(used_count - 1, 0)"),
			"updated_at": time.Now(),
		}).Error
}

func (r *platformDiscountRepository) SoftDelete(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	now := time.Now()
	return db.WithContext(ctx).Model(&entity.PlatformDiscount{}).
		Where("id = ?", id).
		Updates(map[string]any{"deleted_at": now, "is_active": false, "updated_at": now}).Error
}

// ── Member Discount ───────────────────────────────────────────

type IMemberDiscountRepository interface {
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.MemberDiscount, error)
	FindByCode(ctx context.Context, db *gorm.DB, clientID uuid.UUID, code string) (*entity.MemberDiscount, error)
	FindAllByClient(ctx context.Context, db *gorm.DB, clientID uuid.UUID, onlyActive bool, page, limit int) ([]entity.MemberDiscount, error)
	CountAllByClient(ctx context.Context, db *gorm.DB, clientID uuid.UUID, onlyActive bool) (int64, error)
	FindAutoApplicable(ctx context.Context, db *gorm.DB, clientID, packageID uuid.UUID, amount decimal.Decimal) (*entity.MemberDiscount, error)
	Create(ctx context.Context, db *gorm.DB, d *entity.MemberDiscount) error
	Update(ctx context.Context, db *gorm.DB, d *entity.MemberDiscount) error
	IncrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) (bool, error)
	DecrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) error
	SoftDelete(ctx context.Context, db *gorm.DB, id uuid.UUID) error

	// Usage tracking
	CountUsageByUser(ctx context.Context, db *gorm.DB, discountID, telegramUserID uuid.UUID) (int64, error)
	CreateUsage(ctx context.Context, db *gorm.DB, usage *entity.MemberDiscountUsage) error
	DeleteUsageByOrderID(ctx context.Context, db *gorm.DB, orderID uuid.UUID) (int64, error)
}

type memberDiscountRepository struct{}

func NewMemberDiscountRepository() IMemberDiscountRepository {
	return &memberDiscountRepository{}
}

func (r *memberDiscountRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.MemberDiscount, error) {
	var d entity.MemberDiscount
	err := db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *memberDiscountRepository) FindByCode(ctx context.Context, db *gorm.DB, clientID uuid.UUID, code string) (*entity.MemberDiscount, error) {
	var d entity.MemberDiscount
	err := db.WithContext(ctx).
		Where("client_id = ? AND code = ? AND is_active = true AND deleted_at IS NULL", clientID, code).
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *memberDiscountRepository) FindAllByClient(ctx context.Context, db *gorm.DB, clientID uuid.UUID, onlyActive bool, page, limit int) ([]entity.MemberDiscount, error) {
	var discounts []entity.MemberDiscount
	q := db.WithContext(ctx).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Order("created_at DESC")
	if onlyActive {
		now := time.Now()
		q = q.Where("is_active = true AND valid_from <= ? AND (valid_until IS NULL OR valid_until >= ?)", now, now)
	}
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		q = q.Offset(offset).Limit(limit)
	}
	err := q.Find(&discounts).Error
	return discounts, err
}

func (r *memberDiscountRepository) CountAllByClient(ctx context.Context, db *gorm.DB, clientID uuid.UUID, onlyActive bool) (int64, error) {
	var count int64
	q := db.WithContext(ctx).Model(&entity.MemberDiscount{}).
		Where("client_id = ? AND deleted_at IS NULL", clientID)
	if onlyActive {
		now := time.Now()
		q = q.Where("is_active = true AND valid_from <= ? AND (valid_until IS NULL OR valid_until >= ?)", now, now)
	}
	err := q.Count(&count).Error
	return count, err
}

func (r *memberDiscountRepository) FindAutoApplicable(ctx context.Context, db *gorm.DB, clientID, packageID uuid.UUID, amount decimal.Decimal) (*entity.MemberDiscount, error) {
	now := time.Now()
	var d entity.MemberDiscount
	err := db.WithContext(ctx).
		Where(`
			client_id = ?
			AND code IS NULL
			AND is_active = true
			AND deleted_at IS NULL
			AND valid_from <= ?
			AND (valid_until IS NULL OR valid_until >= ?)
			AND (max_usage = -1 OR used_count < max_usage)
			AND min_purchase <= ?
			AND (applicable_package_ids IS NULL OR ? = ANY(applicable_package_ids))
		`, clientID, now, now, amount, packageID.String()).
		Order("value DESC").
		First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *memberDiscountRepository) Create(ctx context.Context, db *gorm.DB, d *entity.MemberDiscount) error {
	return db.WithContext(ctx).Create(d).Error
}

func (r *memberDiscountRepository) Update(ctx context.Context, db *gorm.DB, d *entity.MemberDiscount) error {
	return db.WithContext(ctx).Save(d).Error
}

func (r *memberDiscountRepository) IncrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) (bool, error) {
	// Guard ceiling atomik (sama dengan platform) — member discount quota
	// tidak boleh oversold oleh checkout konkuren.
	result := db.WithContext(ctx).Model(&entity.MemberDiscount{}).
		Where("id = ? AND (max_usage = -1 OR used_count < max_usage)", id).
		Updates(map[string]any{
			"used_count": gorm.Expr("used_count + 1"),
			"updated_at": time.Now(),
		})
	return result.RowsAffected > 0, result.Error
}

func (r *memberDiscountRepository) SoftDelete(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	now := time.Now()
	return db.WithContext(ctx).Model(&entity.MemberDiscount{}).
		Where("id = ?", id).
		Updates(map[string]any{"deleted_at": now, "is_active": false, "updated_at": now}).Error
}

func (r *memberDiscountRepository) CountUsageByUser(ctx context.Context, db *gorm.DB, discountID, telegramUserID uuid.UUID) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Model(&entity.MemberDiscountUsage{}).
		Where("discount_id = ? AND telegram_user_id = ?", discountID, telegramUserID).
		Count(&count).Error
	return count, err
}

func (r *memberDiscountRepository) CreateUsage(ctx context.Context, db *gorm.DB, usage *entity.MemberDiscountUsage) error {
	return db.WithContext(ctx).Create(usage).Error
}

func (r *memberDiscountRepository) DecrementUsage(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	return db.WithContext(ctx).Model(&entity.MemberDiscount{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"used_count": gorm.Expr("GREATEST(used_count - 1, 0)"),
			"updated_at": time.Now(),
		}).Error
}

func (r *memberDiscountRepository) DeleteUsageByOrderID(ctx context.Context, db *gorm.DB, orderID uuid.UUID) (int64, error) {
	result := db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Delete(&entity.MemberDiscountUsage{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
