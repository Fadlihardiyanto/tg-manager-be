package repository

import (
	"context"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IClientUserRepository handles persistence for client_users table.
type IClientUserRepository interface {
	Create(ctx context.Context, tx *gorm.DB, clientUser *entity.ClientUser) error
	Update(ctx context.Context, tx *gorm.DB, clientUser *entity.ClientUser) error
	FindByClientAndUserID(ctx context.Context, tx *gorm.DB, clientID, userID uuid.UUID) (*entity.ClientUser, error)
	FindAllByClientIDPaginated(ctx context.Context, tx *gorm.DB, req *model.AdminTenantUserListRequest) ([]entity.ClientUser, int64, error)
	SoftDelete(ctx context.Context, tx *gorm.DB, id uuid.UUID) error
	SoftDeleteByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) error
	CountActiveByUserID(ctx context.Context, tx *gorm.DB, userID uuid.UUID) (int64, error)
}

// =============================================================================
// Implementation
// =============================================================================

type ClientUserRepository struct {
	log *zap.Logger
}

func NewClientUserRepository(log *zap.Logger) IClientUserRepository {
	return &ClientUserRepository{log: log}
}

func (r *ClientUserRepository) Create(ctx context.Context, tx *gorm.DB, clientUser *entity.ClientUser) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo create start", zap.String("client_id", clientUser.ClientID.String()), zap.String("user_id", clientUser.UserID.String()))

	if err := tx.WithContext(ctx).Create(clientUser).Error; err != nil {
		log.Error("client user repo create failed", zap.Error(err))
		return err
	}
	log.Info("client user repo create success", zap.String("client_user_id", clientUser.ID.String()))
	return nil
}

func (r *ClientUserRepository) Update(ctx context.Context, tx *gorm.DB, clientUser *entity.ClientUser) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo update start", zap.String("client_user_id", clientUser.ID.String()))

	if err := tx.WithContext(ctx).Save(clientUser).Error; err != nil {
		log.Error("client user repo update failed", zap.Error(err))
		return err
	}
	log.Info("client user repo update success", zap.String("client_user_id", clientUser.ID.String()))
	return nil
}

func (r *ClientUserRepository) FindByClientAndUserID(ctx context.Context, tx *gorm.DB, clientID, userID uuid.UUID) (*entity.ClientUser, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo find by client and user start", zap.String("client_id", clientID.String()), zap.String("user_id", userID.String()))

	var clientUser entity.ClientUser
	err := tx.WithContext(ctx).
		Preload("User").
		Where("client_id = ? AND user_id = ? AND deleted_at IS NULL", clientID, userID).
		Take(&clientUser).Error
	if err != nil {
		log.Error("client user repo find by client and user failed", zap.Error(err))
		return nil, err
	}
	log.Info("client user repo find by client and user success", zap.String("client_user_id", clientUser.ID.String()))
	return &clientUser, nil
}

func (r *ClientUserRepository) FindAllByClientIDPaginated(ctx context.Context, tx *gorm.DB, req *model.AdminTenantUserListRequest) ([]entity.ClientUser, int64, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo find all paginated start", zap.String("client_id", req.ClientID.String()), zap.String("user_id", req.UserID), zap.String("email", helper.HashIdentifier(req.Email)), zap.String("role", req.Role), zap.String("verified", req.Verified), zap.Int("page", req.Page), zap.Int("size", req.Size))

	var clientUsers []entity.ClientUser
	var total int64

	db := tx.WithContext(ctx).Model(&entity.ClientUser{}).
		Joins("JOIN users ON users.id = client_users.user_id").
		Where("client_users.client_id = ? AND client_users.deleted_at IS NULL", req.ClientID)
	db = applyTenantUserListFilters(db, req)

	if err := db.Count(&total).Error; err != nil {
		log.Error("client user repo find all paginated count failed", zap.Error(err))
		return nil, 0, err
	}

	offset := (req.Page - 1) * req.Size

	err := db.
		Preload("User", "deleted_at IS NULL").
		Offset(offset).
		Limit(req.Size).
		Order("created_at DESC").
		Find(&clientUsers).Error
	if err != nil {
		log.Error("client user repo find all paginated query failed", zap.Error(err))
		return nil, 0, err
	}
	log.Info("client user repo find all paginated success", zap.Int("count", len(clientUsers)), zap.Int64("total", total))

	return clientUsers, total, nil
}

func applyTenantUserListFilters(db *gorm.DB, req *model.AdminTenantUserListRequest) *gorm.DB {
	if strings.TrimSpace(req.UserID) != "" {
		if userID, err := uuid.Parse(req.UserID); err == nil {
			db = db.Where("client_users.user_id = ?", userID)
		} else {
			return db.Where("1 = 0")
		}
	}
	if email := strings.TrimSpace(req.Email); email != "" {
		db = db.Where("users.email ILIKE ?", "%"+escapeLike(email)+"%")
	}
	if role := strings.TrimSpace(req.Role); role != "" {
		db = db.Where("client_users.role = ?", role)
	}
	switch strings.ToLower(strings.TrimSpace(req.Verified)) {
	case "true":
		db = db.Where("users.is_email_verified = ?", true)
	case "false":
		db = db.Where("users.is_email_verified = ?", false)
	}
	return db
}

func (r *ClientUserRepository) SoftDelete(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo soft delete start", zap.String("client_user_id", id.String()))

	now := time.Now()
	if err := tx.WithContext(ctx).
		Model(&entity.ClientUser{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":  false,
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		log.Error("client user repo soft delete failed", zap.Error(err))
		return err
	}

	log.Info("client user repo soft delete success", zap.String("client_user_id", id.String()))
	return nil
}

func (r *ClientUserRepository) SoftDeleteByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo soft delete by client start", zap.String("client_id", clientID.String()))

	now := time.Now()
	if err := tx.WithContext(ctx).
		Model(&entity.ClientUser{}).
		Where("client_id = ? AND deleted_at IS NULL", clientID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		log.Error("client user repo soft delete by client failed", zap.Error(err))
		return err
	}

	log.Info("client user repo soft delete by client success", zap.String("client_id", clientID.String()))
	return nil
}

func (r *ClientUserRepository) CountActiveByUserID(ctx context.Context, tx *gorm.DB, userID uuid.UUID) (int64, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client user repo count active by user start", zap.String("user_id", userID.String()))

	var count int64
	err := tx.WithContext(ctx).
		Model(&entity.ClientUser{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&count).Error
	if err != nil {
		log.Error("client user repo count active by user failed", zap.Error(err))
		return 0, err
	}
	log.Info("client user repo count active by user success", zap.Int64("count", count))
	return count, nil
}
