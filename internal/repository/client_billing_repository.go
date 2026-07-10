package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IClientBillingRepository handles persistence for client_billings table.
// Tracks the billing relationship between clients and platform plans (B2B).
type IClientBillingRepository interface {
	IRepository[entity.ClientBilling]
	FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.ClientBilling, error)
	// Find the current active billing for a client
	FindActiveByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (*entity.ClientBilling, error)
	// Find active OR pending billing — digunakan saat checkout untuk cegah double checkout
	FindActiveOrPendingByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (*entity.ClientBilling, error)
	// Find all billing history for a client (for billing management page)
	FindAllByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]entity.ClientBilling, error)
	// Find billings that are about to expire (for worker/cron to send warnings)
	FindExpiring(ctx context.Context, tx *gorm.DB, before time.Time) ([]entity.ClientBilling, error)
	// Find billings that are past due (expired but not cancelled)
	FindPastDue(ctx context.Context, tx *gorm.DB) ([]entity.ClientBilling, error)
	// Cancel a billing (set status = 'cancelled', cancelled_at = now)
	Cancel(ctx context.Context, tx *gorm.DB, id uuid.UUID) error
	CountActiveByPlanID(ctx context.Context, tx *gorm.DB, planID uuid.UUID) (int64, error)
	FindByExternalID(ctx context.Context, db *gorm.DB, externalID string) (*entity.ClientBilling, error)
	FindAllPaginated(ctx context.Context, db *gorm.DB, clientID *uuid.UUID, status string, offset, limit int) ([]entity.ClientBilling, int64, error)
	FindExpiredActives(ctx context.Context, db *gorm.DB, before time.Time) ([]entity.ClientBilling, error)
	Create(ctx context.Context, db *gorm.DB, billing *entity.ClientBilling) error
	Update(ctx context.Context, db *gorm.DB, billing *entity.ClientBilling) error
	AtomicUpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, fromStatus string, updates map[string]any) (int64, error)
	UpdateReceiptURL(ctx context.Context, tx *gorm.DB, id uuid.UUID, receiptURL string) error
	DeactivateOtherActiveBillings(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, excludeID uuid.UUID) error
}

// =============================================================================
// Implementation
// =============================================================================

type clientBillingRepository struct {
	Repository[entity.ClientBilling]
}

func NewClientBillingRepository() IClientBillingRepository {
	return &clientBillingRepository{}
}

func (r *clientBillingRepository) FindByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (*entity.ClientBilling, error) {
	var billing entity.ClientBilling
	err := db.WithContext(ctx).
		Preload("Client").
		Preload("Plan").
		Preload("CancelledByAdmin").
		Preload("UpdatedByAdmin").
		Where("id = ?", id).
		First(&billing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &billing, nil
}

func (r *clientBillingRepository) FindAllByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) ([]entity.ClientBilling, error) {
	var billings []entity.ClientBilling
	err := tx.WithContext(ctx).
		Where("client_id = ?", clientID).
		Order("created_at DESC").
		Find(&billings).Error
	return billings, err
}

func (r *clientBillingRepository) FindExpiring(ctx context.Context, tx *gorm.DB, before time.Time) ([]entity.ClientBilling, error) {
	var billings []entity.ClientBilling
	err := tx.WithContext(ctx).
		Where("status = 'active' AND expired_at <= ?", before).
		Order("expired_at ASC").
		Find(&billings).Error
	return billings, err
}

func (r *clientBillingRepository) FindPastDue(ctx context.Context, tx *gorm.DB) ([]entity.ClientBilling, error) {
	var billings []entity.ClientBilling
	err := tx.WithContext(ctx).
		Where("status = 'active' AND expired_at < ?", gorm.Expr("CURRENT_TIMESTAMP")).
		Find(&billings).Error
	return billings, err
}

func (r *clientBillingRepository) Cancel(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	now := time.Now()
	return tx.WithContext(ctx).
		Model(&entity.ClientBilling{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       "cancelled",
			"cancelled_at": now,
		}).Error
}

func (r *clientBillingRepository) CountActiveByPlanID(ctx context.Context, tx *gorm.DB, planID uuid.UUID) (int64, error) {
	var count int64
	err := tx.WithContext(ctx).
		Where("plan_id = ? AND status = 'active'", planID).
		Count(&count).Error
	return count, err
}

func (r *clientBillingRepository) FindByExternalID(ctx context.Context, db *gorm.DB, externalID string) (*entity.ClientBilling, error) {
	var billing entity.ClientBilling
	err := db.WithContext(ctx).
		Preload("Client").
		Preload("Plan").
		Where("external_id = ?", externalID).
		First(&billing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &billing, nil
}

func (r *clientBillingRepository) FindActiveByClientID(ctx context.Context, db *gorm.DB, clientID uuid.UUID) (*entity.ClientBilling, error) {
	var billing entity.ClientBilling
	now := time.Now()
	err := db.WithContext(ctx).
		Preload("Plan").
		Preload("Client").
		Where("client_id = ? AND status = 'active' AND started_at <= ? AND expired_at > ?", clientID, now, now).
		Order("expired_at DESC").
		First(&billing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &billing, nil
}

// FindActiveOrPendingByClientID mencari billing dengan status 'active' atau 'pending'.
// Digunakan saat checkout untuk mencegah double checkout.
func (r *clientBillingRepository) FindActiveOrPendingByClientID(ctx context.Context, db *gorm.DB, clientID uuid.UUID) (*entity.ClientBilling, error) {
	var billing entity.ClientBilling
	err := db.WithContext(ctx).
		Preload("Plan").
		Where("client_id = ? AND status IN ('active', 'pending')", clientID).
		Order("created_at DESC").
		First(&billing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &billing, nil
}

func (r *clientBillingRepository) FindAllPaginated(ctx context.Context, db *gorm.DB, clientID *uuid.UUID, status string, offset, limit int) ([]entity.ClientBilling, int64, error) {
	var billings []entity.ClientBilling
	var total int64

	q := db.WithContext(ctx).
		Model(&entity.ClientBilling{}).
		Preload("Client").
		Preload("Plan").
		Preload("CancelledByAdmin").
		Preload("UpdatedByAdmin")

	if clientID != nil {
		q = q.Where("client_id = ?", *clientID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := q.Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&billings).Error

	return billings, total, err
}

func (r *clientBillingRepository) FindExpiredActives(ctx context.Context, db *gorm.DB, before time.Time) ([]entity.ClientBilling, error) {
	var billings []entity.ClientBilling
	err := db.WithContext(ctx).
		Preload("Client").
		Where("status = 'active' AND expired_at <= ?", before).
		Find(&billings).Error
	return billings, err
}

func (r *clientBillingRepository) Create(ctx context.Context, db *gorm.DB, billing *entity.ClientBilling) error {
	return db.WithContext(ctx).Create(billing).Error
}

func (r *clientBillingRepository) Update(ctx context.Context, db *gorm.DB, billing *entity.ClientBilling) error {
	return db.WithContext(ctx).Save(billing).Error
}

func (r *clientBillingRepository) AtomicUpdateStatus(ctx context.Context, tx *gorm.DB, id uuid.UUID, fromStatus string, updates map[string]any) (int64, error) {
	result := tx.WithContext(ctx).
		Model(&entity.ClientBilling{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(updates)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func (r *clientBillingRepository) UpdateReceiptURL(ctx context.Context, tx *gorm.DB, id uuid.UUID, receiptURL string) error {
	return tx.WithContext(ctx).
		Model(&entity.ClientBilling{}).
		Where("id = ?", id).
		Update("receipt_url", receiptURL).Error
}

func (r *clientBillingRepository) DeactivateOtherActiveBillings(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, excludeID uuid.UUID) error {
	return tx.WithContext(ctx).
		Model(&entity.ClientBilling{}).
		Where("client_id = ? AND status = 'active' AND id != ?", clientID, excludeID).
		Updates(map[string]any{
			"status":       "cancelled",
			"cancelled_at": time.Now(),
		}).Error
}
