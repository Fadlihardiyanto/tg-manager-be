package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ClientBilling represents billing history for a client
type ClientBilling struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID uuid.UUID `gorm:"type:uuid;index;not null"`
	PlanID   uuid.UUID `gorm:"type:uuid;index;not null"`

	Status       string `gorm:"type:varchar(20);not null;default:'pending'"`
	BillingCycle string `gorm:"type:varchar(20);not null;default:'monthly'"`

	Amount      decimal.Decimal `gorm:"type:decimal(12,2);not null"`
	StartedAt   time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP"`
	ExpiredAt   time.Time       `gorm:"not null"`
	CancelledAt *time.Time

	ExternalID string     `gorm:"type:varchar(255);uniqueIndex;column:external_id"`
	PaymentURL string     `gorm:"type:varchar(500);column:payment_url"`
	PaidAt     *time.Time `gorm:"column:paid_at"`

	IsManual  bool       `gorm:"default:false"`
	Note      string     `gorm:"type:text"`
	CreatedBy   *uuid.UUID `gorm:"type:uuid;column:created_by"`
	CancelledBy *uuid.UUID `gorm:"type:uuid;column:cancelled_by"`
	UpdatedBy   *uuid.UUID `gorm:"type:uuid;column:updated_by"`

	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`

	Client Client       `gorm:"foreignKey:ClientID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Plan   PlatformPlan `gorm:"foreignKey:PlanID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	CancelledByAdmin *AdminUser `gorm:"foreignKey:CancelledBy;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	UpdatedByAdmin   *AdminUser `gorm:"foreignKey:UpdatedBy;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	OriginalAmount decimal.Decimal `gorm:"type:decimal(12,2);not null;column:original_amount"`
	DiscountAmount decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0;column:discount_amount"`
	DiscountID     *uuid.UUID      `gorm:"type:uuid;column:discount_id"`
}

func (ClientBilling) TableName() string {
	return "client_billings"
}
