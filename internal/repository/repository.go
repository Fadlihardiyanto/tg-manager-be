package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

// escapeLike sanitizes user input for use in SQL ILIKE/LIKE patterns.
// It escapes % and _ wildcards to prevent blind data enumeration via crafted search queries.
func escapeLike(s string) string {
	return strings.NewReplacer("%", "\\%", "_", "\\_").Replace(s)
}

// clampPagination membatasi page/limit supaya (page-1)*limit tidak overflow
// (offset negatif -> Postgres error -> 500) dan limit=0 tidak menghasilkan
// query tanpa batas. Behavior-preserving untuk semua nilai wajar.
func clampPagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	const maxPage = 1_000_000
	if page > maxPage {
		page = maxPage
	}
	return page, limit
}

// clampOffsetLimit membatasi offset/limit yang diterima langsung dari caller
// (sudah dihitung di usecase): offset negatif -> Postgres error 500;
// limit <= 0 -> query tanpa batas.
func clampOffsetLimit(offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return offset, limit
}

// IRepository is the base generic interface for all repositories
type IRepository[T any] interface {
	Create(ctx context.Context, tx *gorm.DB, entity *T) error
	Update(ctx context.Context, tx *gorm.DB, entity *T) error
	Delete(ctx context.Context, tx *gorm.DB, entity *T) error
	CountById(ctx context.Context, tx *gorm.DB, id any) (int64, error)
	FindById(ctx context.Context, tx *gorm.DB, entity *T, id any) error
	FindAll(ctx context.Context, tx *gorm.DB) ([]T, error)
}

// Repository provides the default generic implementation
type Repository[T any] struct{}

func (r *Repository[T]) Create(ctx context.Context, tx *gorm.DB, entity *T) error {
	return tx.WithContext(ctx).Create(entity).Error
}

func (r *Repository[T]) Update(ctx context.Context, tx *gorm.DB, entity *T) error {
	return tx.WithContext(ctx).Save(entity).Error
}

func (r *Repository[T]) Delete(ctx context.Context, tx *gorm.DB, entity *T) error {
	return tx.WithContext(ctx).Delete(entity).Error
}

func (r *Repository[T]) CountById(ctx context.Context, tx *gorm.DB, id any) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).Model(new(T)).Where("id = ?", id).Count(&count).Error
	return count, err
}

func (r *Repository[T]) FindById(ctx context.Context, tx *gorm.DB, entity *T, id any) error {
	return tx.WithContext(ctx).Where("id = ?", id).Take(entity).Error
}

func (r *Repository[T]) FindAll(ctx context.Context, tx *gorm.DB) ([]T, error) {
	var entities []T
	err := tx.WithContext(ctx).Find(&entities).Error
	return entities, err
}
