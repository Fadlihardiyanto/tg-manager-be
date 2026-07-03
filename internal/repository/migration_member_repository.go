package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IMigrationMemberRepository interface {
	IRepository[entity.MigrationMember]
	BulkInsert(ctx context.Context, tx *gorm.DB, members []entity.MigrationMember) error
	FindExistingUsernames(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, packageID uuid.UUID, usernames []string) (map[string]bool, error)
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MigrationMemberFilterRequest) ([]entity.MigrationMember, error)
	CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MigrationMemberFilterRequest) (int64, error)
	FindAllByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]entity.MigrationMember, error)
	FindPendingClaimsByUsername(ctx context.Context, tx *gorm.DB, username string) ([]entity.MigrationMember, error)
	UpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, status string) error
}

type MigrationMemberRepository struct {
	Repository[entity.MigrationMember]
}

func NewMigrationMemberRepository() IMigrationMemberRepository {
	return &MigrationMemberRepository{}
}

func (r *MigrationMemberRepository) BulkInsert(ctx context.Context, tx *gorm.DB, members []entity.MigrationMember) error {
	if len(members) == 0 {
		return nil
	}
	return tx.WithContext(ctx).CreateInBatches(members, 100).Error
}

func (r *MigrationMemberRepository) FindExistingUsernames(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, packageID uuid.UUID, usernames []string) (map[string]bool, error) {
	if len(usernames) == 0 {
		return map[string]bool{}, nil
	}

	var existing []entity.MigrationMember
	err := tx.WithContext(ctx).
		Where("client_id = ? AND package_id = ? AND username IN ? AND status = ?", clientID, packageID, usernames, "pending").
		Find(&existing).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(existing))
	for _, m := range existing {
		result[m.Username] = true
	}
	return result, nil
}

func (r *MigrationMemberRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MigrationMemberFilterRequest) ([]entity.MigrationMember, error) {
	var members []entity.MigrationMember

	page := filter.Page
	limit := filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := tx.WithContext(ctx).Where("client_id = ?", clientID)

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if filter.Search != "" {
		searchPattern := "%" + escapeLike(filter.Search) + "%"
		query = query.Where("username ILIKE ?", searchPattern)
	}

	if filter.PackageID != uuid.Nil {
		query = query.Where("package_id = ?", filter.PackageID)
	}

	err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&members).Error
	return members, err
}

func (r *MigrationMemberRepository) CountByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, filter model.MigrationMemberFilterRequest) (int64, error) {
	var count int64
	query := tx.WithContext(ctx).Model(&entity.MigrationMember{}).Where("client_id = ?", clientID)

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if filter.Search != "" {
		searchPattern := "%" + escapeLike(filter.Search) + "%"
		query = query.Where("username ILIKE ?", searchPattern)
	}

	if filter.PackageID != uuid.Nil {
		query = query.Where("package_id = ?", filter.PackageID)
	}

	err := query.Count(&count).Error
	return count, err
}

// ponytail: limit prevents unbounded memory for large exports.
// 5000 rows is enough for migration member reports; bump if needed.
const exportMaxRows = 5000

func (r *MigrationMemberRepository) FindAllByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]entity.MigrationMember, error) {
	var members []entity.MigrationMember
	err := tx.WithContext(ctx).Where("client_id = ?", clientID).Order("created_at DESC").Limit(exportMaxRows).Find(&members).Error
	return members, err
}

func (r *MigrationMemberRepository) FindPendingClaimsByUsername(ctx context.Context, tx *gorm.DB, username string) ([]entity.MigrationMember, error) {
	var claims []entity.MigrationMember
	err := tx.WithContext(ctx).Where("username = ? AND status = ?", username, "pending").Find(&claims).Error
	return claims, err
}

func (r *MigrationMemberRepository) UpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, status string) error {
	updates := map[string]any{
		"status": status,
	}
	if status == "claimed" {
		updates["claimed_at"] = gorm.Expr("CURRENT_TIMESTAMP")
	}
	return tx.WithContext(ctx).Model(&entity.MigrationMember{}).Where("id = ?", id).Updates(updates).Error
}
