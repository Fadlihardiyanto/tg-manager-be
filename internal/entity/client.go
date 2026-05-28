package entity

import (
	"time"

	"github.com/google/uuid"
)

type Client struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name             string    `gorm:"type:varchar(255);not null"`
	Slug             string    `gorm:"type:varchar(255);unique;not null"`
	Description      string    `gorm:"type:text"`
	LogoURL          string    `gorm:"type:varchar(500)"`
	IsActive         bool      `gorm:"default:true"`
	OwnerUserID      uuid.UUID `gorm:"type:uuid;not null"`
	SubscriptionTier string    `gorm:"type:varchar(50);default:'free'"`
	MidtransServerKey *string   `gorm:"column:midtrans_server_key"`
	MidtransClientKey *string   `gorm:"column:midtrans_client_key"`
	MidtransIsSandbox bool      `gorm:"column:midtrans_is_sandbox;default:true"`
	CreatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt        *time.Time

	// Relationships
	Owner    *User         `gorm:"foreignKey:OwnerUserID"`
	Users    []User        `gorm:"many2many:client_users"`
	Bots     []TelegramBot `gorm:"foreignKey:ClientID"`
	Groups   []Group       `gorm:"foreignKey:ClientID"`
	Packages []Package     `gorm:"foreignKey:ClientID"`
}
