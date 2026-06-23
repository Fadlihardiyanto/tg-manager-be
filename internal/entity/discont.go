package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type PlatformDiscount struct {
	ID   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Name string    `gorm:"type:varchar(255);not null;column:name"`
	Code *string   `gorm:"type:varchar(100);column:code"`
	// NULL = otomatis tanpa kode

	Type string `gorm:"type:varchar(20);not null;column:type"`
	// 'percentage' | 'fixed'
	Value       decimal.Decimal  `gorm:"type:decimal(12,2);not null;column:value"`
	MaxDiscount *decimal.Decimal `gorm:"type:decimal(12,2);column:max_discount"`
	// Cap untuk type=percentage
	MinPurchase decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0;column:min_purchase"`

	MaxUsage          int            `gorm:"not null;default:-1;column:max_usage"`
	UsedCount         int            `gorm:"not null;default:0;column:used_count"`
	ApplicablePlanIDs pq.StringArray `gorm:"type:uuid[];column:applicable_plan_ids"`
	// NULL = semua plan
	ApplicableClientIDs pq.StringArray `gorm:"type:uuid[];column:applicable_client_ids"`
	// NULL = semua client

	ValidFrom  time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:valid_from"`
	ValidUntil *time.Time `gorm:"column:valid_until"`
	IsActive   bool       `gorm:"not null;default:true;column:is_active"`

	CreatedBy *uuid.UUID `gorm:"type:uuid;column:created_by"`
	CreatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:updated_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`
}

func (PlatformDiscount) TableName() string {
	return "platform_discounts"
}

type MemberDiscount struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	ClientID uuid.UUID `gorm:"type:uuid;not null;column:client_id"`
	Name     string    `gorm:"type:varchar(255);not null;column:name"`
	Code     *string   `gorm:"type:varchar(100);column:code"`
	// NULL = otomatis tanpa kode

	Type        string           `gorm:"type:varchar(20);not null;column:type"`
	Value       decimal.Decimal  `gorm:"type:decimal(12,2);not null;column:value"`
	MaxDiscount *decimal.Decimal `gorm:"type:decimal(12,2);column:max_discount"`
	MinPurchase decimal.Decimal  `gorm:"type:decimal(12,2);not null;default:0;column:min_purchase"`

	MaxUsage             int            `gorm:"not null;default:-1;column:max_usage"`
	UsedCount            int            `gorm:"not null;default:0;column:used_count"`
	MaxUsagePerUser      int            `gorm:"not null;default:1;column:max_usage_per_user"`
	ApplicablePackageIDs pq.StringArray `gorm:"type:uuid[];column:applicable_package_ids"`

	ValidFrom  time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:valid_from"`
	ValidUntil *time.Time `gorm:"column:valid_until"`
	IsActive   bool       `gorm:"not null;default:true;column:is_active"`

	CreatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:created_at"`
	UpdatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:updated_at"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`

	// Relations
	Client Client `gorm:"foreignKey:ClientID"`
}

func (MemberDiscount) TableName() string {
	return "member_discounts"
}

// ── Member Discount Usage ─────────────────────────────────────

type MemberDiscountUsage struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	DiscountID     uuid.UUID       `gorm:"type:uuid;not null;column:discount_id"`
	TelegramUserID uuid.UUID       `gorm:"type:uuid;not null;column:telegram_user_id"`
	OrderID        uuid.UUID       `gorm:"type:uuid;not null;column:order_id"`
	DiscountAmount decimal.Decimal `gorm:"type:decimal(12,2);not null;column:discount_amount"`
	UsedAt         time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP;column:used_at"`
}

func (MemberDiscountUsage) TableName() string {
	return "member_discount_usages"
}
