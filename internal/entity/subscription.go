package entity

import (
	"time"

	"github.com/google/uuid"
)

type Subscription struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	TelegramUserID   uuid.UUID  `gorm:"type:uuid;index;not null"`
	PackageID        uuid.UUID  `gorm:"type:uuid;index;not null"`
	ClientID         uuid.UUID  `gorm:"type:uuid;index;not null"`
	OrderID          *uuid.UUID `gorm:"type:uuid"`
	Status           string     `gorm:"type:varchar(20);default:'active'"`
	ExpiredAt        time.Time  `gorm:"not null;index"`
	ActivatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	AutoRenew        bool       `gorm:"default:false"`
	GracePeriodHours int        `gorm:"default:0"`
	KickedAt         *time.Time
	LastCheckedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	CreatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt        *time.Time

	// Relationships
	User    TelegramUser `gorm:"foreignKey:telegram_user_id;references:id"`
	Package Package      `gorm:"foreignKey:package_id;references:id"`
}
