package entity

import (
	"time"

	"github.com/google/uuid"
)

type Broadcast struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID       uuid.UUID  `gorm:"type:uuid;index;not null"`
	BotUUID        uuid.UUID  `gorm:"column:bot_id;type:uuid;index;not null"`
	TargetType     string     `gorm:"type:varchar(20);not null"` // 'group' or 'member' (DM)
	MessageType    string     `gorm:"type:varchar(20);not null"` // 'text', 'photo', 'document'
	MessageText    string     `gorm:"type:text;not null"`
	FileUrl        *string    `gorm:"type:varchar(500)"`
	TelegramFileID *string    `gorm:"type:varchar(255)"`
	Status         string     `gorm:"type:varchar(20);not null;default:'pending'"`
	TotalTargets   int        `gorm:"type:integer;not null;default:0"`
	SentCount      int        `gorm:"type:integer;not null;default:0"`
	FailedCount    int        `gorm:"type:integer;not null;default:0"`
	FailedDetails  *string    `gorm:"type:jsonb"`
	GroupFilter    *string    `gorm:"type:jsonb"`
	ScheduledAt    *time.Time `gorm:"type:timestamp with time zone"`
	CreatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt      *time.Time `gorm:"index"`

	Client Client      `gorm:"foreignKey:ClientID"`
	Bot    TelegramBot `gorm:"foreignKey:BotUUID;references:ID"`
}

func (Broadcast) TableName() string {
	return "broadcasts"
}
