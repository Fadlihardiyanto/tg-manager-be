package repository

import (
	"context"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IClientRepository interface {
	IRepository[entity.Client]
	FindByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.Client, error)
	FindByIDWithOwner(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Client, error)
	FindAllPaginated(ctx context.Context, tx *gorm.DB, req *model.AdminListAllClientsRequest) ([]entity.Client, int64, error)
	FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Client, error)
	FindBySlug(ctx context.Context, tx *gorm.DB, slug string) (*entity.Client, error)
	SoftDelete(ctx context.Context, tx *gorm.DB, client *entity.Client) error
	Activate(ctx context.Context, tx *gorm.DB, client *entity.Client) error
	Deactivate(ctx context.Context, tx *gorm.DB, client *entity.Client) error
	UpdateSubscriptionTier(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, tier string) error
}

type ClientRepository struct {
	Repository[entity.Client]
	log *zap.Logger
}

func NewClientRepository(log *zap.Logger) IClientRepository {
	return &ClientRepository{log: log}
}

func (r *ClientRepository) FindByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.Client, error) {
	var client entity.Client
	err := tx.WithContext(ctx).Where("email = ?", email).First(&client).Error
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func (r *ClientRepository) FindAllPaginated(ctx context.Context, tx *gorm.DB, req *model.AdminListAllClientsRequest) ([]entity.Client, int64, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo find all paginated start",
		zap.String("client_id", req.ClientID),
		zap.String("name", req.Name),
		zap.String("slug", req.Slug),
		zap.String("active", req.Active),
		zap.String("subscription_tier", req.SubscriptionTier),
		zap.Int("page", req.Page),
		zap.Int("size", req.Size))

	var clients []entity.Client
	var total int64

	db := tx.WithContext(ctx).Model(&entity.Client{}).Where("deleted_at IS NULL")
	db = applyClientListFilters(db, req)

	if err := db.Count(&total).Error; err != nil {
		log.Error("client repo find all paginated count failed", zap.Error(err))
		return nil, 0, err
	}

	page, limit := clampPagination(req.Page, req.Size)
	offset := (page - 1) * limit

	err := db.
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&clients).Error
	if err != nil {
		log.Error("client repo find all paginated query failed", zap.Error(err))
		return nil, 0, err
	}

	log.Info("client repo find all paginated success", zap.Int("count", len(clients)), zap.Int64("total", total))
	return clients, total, nil
}

func applyClientListFilters(db *gorm.DB, req *model.AdminListAllClientsRequest) *gorm.DB {
	if strings.TrimSpace(req.ClientID) != "" {
		if clientID, err := uuid.Parse(req.ClientID); err == nil {
			db = db.Where("id = ?", clientID)
		} else {
			return db.Where("1 = 0")
		}
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		db = db.Where("name ILIKE ?", "%"+escapeLike(name)+"%")
	}
	if slug := strings.TrimSpace(req.Slug); slug != "" {
		db = db.Where("slug ILIKE ?", "%"+escapeLike(slug)+"%")
	}
	if tier := strings.TrimSpace(req.SubscriptionTier); tier != "" {
		db = db.Where("subscription_tier = ?", tier)
	}
	switch strings.ToLower(strings.TrimSpace(req.Active)) {
	case "true":
		db = db.Where("is_active = ?", true)
	case "false":
		db = db.Where("is_active = ?", false)
	}
	return db
}

func (r *ClientRepository) FindByID(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Client, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo find by id start", zap.String("client_id", id.String()))

	var client entity.Client
	err := tx.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&client).Error
	if err != nil {
		log.Error("client repo find by id failed", zap.Error(err))
		return nil, err
	}

	log.Info("client repo find by id success", zap.String("client_id", id.String()))
	return &client, nil
}

func (r *ClientRepository) FindByIDWithOwner(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*entity.Client, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo find by id with owner start", zap.String("client_id", id.String()))

	var client entity.Client
	err := tx.WithContext(ctx).
		Preload("Owner").
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&client).Error
	if err != nil {
		log.Error("client repo find by id with owner failed", zap.Error(err))
		return nil, err
	}

	log.Info("client repo find by id with owner success", zap.String("client_id", id.String()))
	return &client, nil
}

func (r *ClientRepository) FindBySlug(ctx context.Context, tx *gorm.DB, slug string) (*entity.Client, error) {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo find by slug start", zap.String("slug", slug))

	var client entity.Client
	err := tx.WithContext(ctx).
		Where("slug = ? AND deleted_at IS NULL", slug).
		Take(&client).Error
	if err != nil {
		log.Error("client repo find by slug failed", zap.Error(err))
		return nil, err
	}

	log.Info("client repo find by slug success", zap.String("slug", slug))
	return &client, nil
}

func (r *ClientRepository) SoftDelete(ctx context.Context, tx *gorm.DB, client *entity.Client) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo soft delete start", zap.String("client_id", client.ID.String()))

	now := time.Now()
	if err := tx.WithContext(ctx).
		Model(&entity.Client{}).
		Where("id = ?", client.ID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"deleted_at": now,
			"updated_at": now,
		}).Error; err != nil {
		log.Error("client repo soft delete failed", zap.Error(err))
		return err
	}

	log.Info("client repo soft delete success", zap.String("client_id", client.ID.String()))
	return nil
}

func (r *ClientRepository) Activate(ctx context.Context, tx *gorm.DB, client *entity.Client) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo activate start", zap.String("client_id", client.ID.String()))

	if err := tx.WithContext(ctx).
		Model(&entity.Client{}).
		Where("id = ?", client.ID).
		Updates(map[string]interface{}{
			"is_active":  true,
			"updated_at": time.Now(),
		}).Error; err != nil {
		log.Error("client repo activate failed", zap.Error(err))
		return err
	}

	log.Info("client repo activate success", zap.String("client_id", client.ID.String()))
	return nil
}

func (r *ClientRepository) Deactivate(ctx context.Context, tx *gorm.DB, client *entity.Client) error {
	log := logger.FromContext(ctx, r.log)
	log.Info("client repo deactivate start", zap.String("client_id", client.ID.String()))

	if err := tx.WithContext(ctx).
		Model(&entity.Client{}).
		Where("id = ?", client.ID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		}).Error; err != nil {
		log.Error("client repo deactivate failed", zap.Error(err))
		return err
	}

	log.Info("client repo deactivate success", zap.String("client_id", client.ID.String()))
	return nil
}

func (r *ClientRepository) UpdateSubscriptionTier(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, tier string) error {
	return tx.WithContext(ctx).
		Model(&entity.Client{}).
		Where("id = ?", clientID).
		Update("subscription_tier", tier).Error
}
