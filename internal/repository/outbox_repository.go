package repository

import (
	"context"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IOutboxRepository interface {
	IRepository[entity.Outbox]
	Create(ctx context.Context, db *gorm.DB, outbox *entity.Outbox) error
	Update(ctx context.Context, db *gorm.DB, outbox *entity.Outbox) error
	FindPending(ctx context.Context, db *gorm.DB, limit int) ([]entity.Outbox, error)
}

type outboxRepository struct {
	Repository[entity.Outbox]
}

func NewOutboxRepository() IOutboxRepository {
	return &outboxRepository{}
}

func (r *outboxRepository) Create(ctx context.Context, db *gorm.DB, outbox *entity.Outbox) error {
	return db.WithContext(ctx).Create(outbox).Error
}

func (r *outboxRepository) Update(ctx context.Context, db *gorm.DB, outbox *entity.Outbox) error {
	return db.WithContext(ctx).Save(outbox).Error
}

func (r *outboxRepository) FindPending(ctx context.Context, db *gorm.DB, limit int) ([]entity.Outbox, error) {
	var list []entity.Outbox
	err := db.WithContext(ctx).
		Where("status = ? AND process_after <= CURRENT_TIMESTAMP AND retry_count < max_retries", "pending").
		Order("created_at ASC").
		Limit(limit).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Find(&list).Error
	return list, err
}
