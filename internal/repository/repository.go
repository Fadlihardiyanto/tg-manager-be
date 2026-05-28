package repository

import (
	"context"

	"gorm.io/gorm"
)

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
