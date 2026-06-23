package entity

import (
	"time"

	"github.com/google/uuid"
)

type TelegramUser struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	TelegramUserID int64     `gorm:"not null;index"` // Telegram's user ID is always a number
	Username       string    `gorm:"type:varchar(255)"`
	FirstName      string    `gorm:"type:varchar(255)"`
	LastName       string    `gorm:"type:varchar(255)"`
	Phone          string    `gorm:"type:varchar(50)"`
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt      *time.Time

	// Relationships
	Subscriptions []Subscription `gorm:"foreignKey:TelegramUserID"`
	Orders        []Order        `gorm:"foreignKey:TelegramUserID"`
}
