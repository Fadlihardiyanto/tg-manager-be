package entity

import (
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID       uuid.UUID `gorm:"type:uuid;index;not null"`
	BotID          uuid.UUID `gorm:"type:uuid;index;not null"`
	TelegramChatID int64     `gorm:"not null;unique"` // Telegram's chat ID is always a number
	Name           string    `gorm:"type:varchar(255)"`
	Description    string    `gorm:"type:text"`
	IsActive       bool      `gorm:"default:true"`
	InactiveReason string    `gorm:"type:varchar(255)"`
	MemberCount    int       `gorm:"default:0"`
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt      *time.Time

	// Relationships
	Client   Client      `gorm:"foreignKey:ClientID"`
	Bot      TelegramBot `gorm:"foreignKey:BotID"`
	Packages []Package   `gorm:"many2many:package_groups;"`
}
