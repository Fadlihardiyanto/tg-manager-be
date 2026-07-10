package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type CustomCommand struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID       uuid.UUID      `gorm:"type:uuid;index;not null"`
	BotUUID        uuid.UUID      `gorm:"column:bot_id;type:uuid;index;not null"`
	CommandTrigger string         `gorm:"type:varchar(50);not null"`
	ResponseType   string         `gorm:"type:varchar(20);not null;default:'text'"`
	ResponseText   string         `gorm:"type:text;not null"`
	FileUrl        *string        `gorm:"type:varchar(500)"`
	TelegramFileID *string        `gorm:"type:text"`
	IsActive       bool           `gorm:"default:true"`
	AccessScope    string         `gorm:"type:varchar(20);not null;default:'public'"`
	ChatTypeScope  string         `gorm:"type:varchar(20);not null;default:'all'"`
	PackageIDs     pq.StringArray `gorm:"type:uuid[]"`
	GroupIDs       pq.StringArray `gorm:"type:uuid[]"`
	CreatedAt      time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt      *time.Time

	Client Client      `gorm:"foreignKey:ClientID"`
	Bot    TelegramBot `gorm:"foreignKey:BotUUID;references:ID"`
}

func (CustomCommand) TableName() string {
	return "custom_commands"
}
