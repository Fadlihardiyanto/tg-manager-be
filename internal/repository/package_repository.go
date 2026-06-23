package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IPackageRepository interface {
	IRepository[entity.Package]
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, page, limit int) ([]entity.Package, error)
	FindByGroupID(ctx context.Context, tx *gorm.DB, groupID uuid.UUID) ([]entity.Package, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Package, error)
	Delete(ctx context.Context, tx *gorm.DB, pkg *entity.Package) error
	AssociateGroups(ctx context.Context, tx *gorm.DB, pkg *entity.Package, groups []entity.Group) error
	CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error)
}

type PackageRepository struct {
	Repository[entity.Package]
}

func NewPackageRepository() IPackageRepository {
	return &PackageRepository{}
}

func (r *PackageRepository) FindByGroupID(ctx context.Context, tx *gorm.DB, groupID uuid.UUID) ([]entity.Package, error) {
	var packages []entity.Package
	err := tx.WithContext(ctx).
		Joins("JOIN package_groups ON package_groups.package_id = packages.id").
		Where("package_groups.group_id = ? AND packages.deleted_at IS NULL", groupID).
		Find(&packages).Error
	return packages, err
}

func (r *PackageRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, page, limit int) ([]entity.Package, error) {
	var packages []entity.Package
	offset := (page - 1) * limit
	err := tx.WithContext(ctx).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Offset(offset).
		Limit(limit).
		Find(&packages).Error
	return packages, err
}

func (r *PackageRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Package, error) {
	var pkg entity.Package
	err := tx.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Preload("Groups").First(&pkg).Error
	return &pkg, err
}

func (r *PackageRepository) Delete(ctx context.Context, tx *gorm.DB, pkg *entity.Package) error {
	return tx.WithContext(ctx).Delete(pkg).Error
}

func (r *PackageRepository) AssociateGroups(ctx context.Context, tx *gorm.DB, pkg *entity.Package, groups []entity.Group) error {
	return tx.WithContext(ctx).Model(pkg).Association("Groups").Replace(groups)
}

func (r *PackageRepository) CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.Package{}).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Count(&count).Error
	return count, err
}
