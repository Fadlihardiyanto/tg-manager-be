package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IPlatformPlanRepository interface {
	FindAll(ctx context.Context, db *gorm.DB, onlyActive bool, isLandingPage *bool, page int, limit int) ([]entity.PlatformPlan, error)
	CountAll(ctx context.Context, db *gorm.DB, onlyActive bool, isLandingPage *bool) (int64, error)
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.PlatformPlan, error)
	FindByName(ctx context.Context, db *gorm.DB, name string) (*entity.PlatformPlan, error)
	Create(ctx context.Context, db *gorm.DB, plan *entity.PlatformPlan) error
	Update(ctx context.Context, db *gorm.DB, plan *entity.PlatformPlan) error
	Delete(ctx context.Context, db *gorm.DB, id uuid.UUID) error
	IsNameTaken(ctx context.Context, db *gorm.DB, name string, excludeID *uuid.UUID) (bool, error)

	// Find active plan by name (e.g., "free", "pro")
	FindActiveByName(ctx context.Context, tx *gorm.DB, name string) (*entity.PlatformPlan, error)

	// List only active plans (for client-facing plan selection)
	FindAllActive(ctx context.Context, tx *gorm.DB) ([]entity.PlatformPlan, error)

	// List all plans including inactive (for admin management)
	FindAllWithInactive(ctx context.Context, tx *gorm.DB) ([]entity.PlatformPlan, error)
}

type platformPlanRepository struct {
	Repository[entity.PlatformPlan]
}

func NewPlatformPlanRepository() IPlatformPlanRepository {
	return &platformPlanRepository{}
}

func (r *platformPlanRepository) FindAll(ctx context.Context, db *gorm.DB, onlyActive bool, isLandingPage *bool, page int, limit int) ([]entity.PlatformPlan, error) {
	var plans []entity.PlatformPlan
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Order("price_monthly ASC")
	if onlyActive {
		q = q.Where("is_active = true")
	}
	if isLandingPage != nil {
		q = q.Where("is_landing_page = ?", *isLandingPage)
	}
	if page > 0 && limit > 0 {
		offset := (page - 1) * limit
		q = q.Offset(offset).Limit(limit)
	}
	err := q.Find(&plans).Error
	return plans, err
}

func (r *platformPlanRepository) CountAll(ctx context.Context, db *gorm.DB, onlyActive bool, isLandingPage *bool) (int64, error) {
	var count int64
	q := db.WithContext(ctx).Model(&entity.PlatformPlan{}).Where("deleted_at IS NULL")
	if onlyActive {
		q = q.Where("is_active = true")
	}
	if isLandingPage != nil {
		q = q.Where("is_landing_page = ?", *isLandingPage)
	}
	err := q.Count(&count).Error
	return count, err
}

func (r *platformPlanRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.PlatformPlan, error) {
	var plan entity.PlatformPlan
	err := db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *platformPlanRepository) FindByName(ctx context.Context, db *gorm.DB, name string) (*entity.PlatformPlan, error) {
	var plan entity.PlatformPlan
	err := db.WithContext(ctx).
		Where("name = ? AND deleted_at IS NULL", name).
		First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &plan, nil
}

func (r *platformPlanRepository) Create(ctx context.Context, db *gorm.DB, plan *entity.PlatformPlan) error {
	return db.WithContext(ctx).Create(plan).Error
}

func (r *platformPlanRepository) Update(ctx context.Context, db *gorm.DB, plan *entity.PlatformPlan) error {
	return db.WithContext(ctx).Save(plan).Error
}

func (r *platformPlanRepository) Delete(ctx context.Context, db *gorm.DB, id uuid.UUID) error {
	now := time.Now()
	return db.WithContext(ctx).
		Model(&entity.PlatformPlan{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"deleted_at": now,
			"is_active":  false,
			"updated_at": now,
		}).Error
}

func (r *platformPlanRepository) IsNameTaken(ctx context.Context, db *gorm.DB, name string, excludeID *uuid.UUID) (bool, error) {
	q := db.WithContext(ctx).
		Model(&entity.PlatformPlan{}).
		Where("name = ? AND deleted_at IS NULL", name)

	if excludeID != nil {
		q = q.Where("id != ?", *excludeID)
	}

	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *platformPlanRepository) FindActiveByName(ctx context.Context, tx *gorm.DB, name string) (*entity.PlatformPlan, error) {
	var plan entity.PlatformPlan
	err := tx.WithContext(ctx).
		Where("name = ? AND is_active = true AND deleted_at IS NULL", name).
		Take(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *platformPlanRepository) FindAllActive(ctx context.Context, tx *gorm.DB) ([]entity.PlatformPlan, error) {
	var plans []entity.PlatformPlan
	err := tx.WithContext(ctx).
		Where("is_active = true AND deleted_at IS NULL").
		Order("price_monthly ASC").
		Find(&plans).Error
	return plans, err
}

func (r *platformPlanRepository) FindAllWithInactive(ctx context.Context, tx *gorm.DB) ([]entity.PlatformPlan, error) {
	var plans []entity.PlatformPlan
	err := tx.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("price_monthly ASC").
		Find(&plans).Error
	return plans, err
}
