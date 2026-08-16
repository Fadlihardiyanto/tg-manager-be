package entity

import (
	"time"

	"github.com/google/uuid"
)

type ReportSetting struct {
	ClientID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Enabled      bool      `gorm:"not null;default:false"`
	TargetChatID int64     `gorm:"not null"`
	BotID        uuid.UUID `gorm:"type:uuid;not null"`
	ReportTime   string    `gorm:"type:varchar(5);not null;default:'08:00'"`
	LastSentAt   *time.Time
	UpdatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func (ReportSetting) TableName() string { return "report_settings" }
