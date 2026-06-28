package entity

import (
	"time"

	"github.com/google/uuid"
)

type TelegramBot struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID  uuid.UUID `gorm:"type:uuid;index;not null"`
	Token     string    `gorm:"type:text;not null"`
	Username  string    `gorm:"type:varchar(255)"`
	BotID     int64     `gorm:"column:bot_id"`
	BotRole   string    `gorm:"column:bot_role;default:'all_in_one'"`
	IsActive  bool      `gorm:"default:true"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time

	// Relationships
	Client Client  `gorm:"foreignKey:ClientID"`
	Groups []Group `gorm:"foreignKey:BotUUID;references:ID"`
}
