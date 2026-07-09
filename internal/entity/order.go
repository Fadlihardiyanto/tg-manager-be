package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type Order struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	TelegramUserID uuid.UUID       `gorm:"type:uuid;index;not null"`
	PackageID      uuid.UUID       `gorm:"type:uuid;index;not null"`
	ExternalID     string          `gorm:"type:varchar(255);not null;index"`
	Amount         decimal.Decimal `gorm:"type:decimal(12,2);not null"`
	OriginalAmount decimal.Decimal `gorm:"type:decimal(12,2)"`
	DiscountID     *uuid.UUID      `gorm:"type:uuid"`
	DiscountAmount decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0"`
	Status         string          `gorm:"type:varchar(20);default:'pending'"`
	PaymentMethod  string          `gorm:"type:varchar(50)"`
	PaidAt         *time.Time
	ClientID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	SubscriptionID *uuid.UUID `gorm:"type:uuid"`
	PaymentURL     string     `gorm:"type:varchar(500)"`
	ReceiptURL       string         `gorm:"type:varchar(500);column:receipt_url"`
	RawNotification  datatypes.JSON `gorm:"type:jsonb;column:raw_notification"`
	ExpiredAt        *time.Time
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt      *time.Time

	// Relationships
	User     TelegramUser    `gorm:"foreignKey:TelegramUserID"`
	Package  Package         `gorm:"foreignKey:PackageID"`
	Discount *MemberDiscount `gorm:"foreignKey:DiscountID"`
}
