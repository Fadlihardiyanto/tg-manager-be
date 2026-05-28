package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Outbox represents the outbox pattern for reliable event publishing
type Outbox struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	AggregateType string         `gorm:"type:varchar(100);not null"`
	AggregateID   uuid.UUID      `gorm:"type:uuid;not null"`
	EventType     string         `gorm:"type:varchar(100);not null"`
	Payload       datatypes.JSON `gorm:"type:jsonb;not null"`
	Status        string         `gorm:"type:varchar(20);not null;default:'pending'"`
	RetryCount    int            `gorm:"not null;default:0"`
	MaxRetries    int            `gorm:"not null;default:3"`
	LastError     string         `gorm:"type:text"`
	ProcessAfter  time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP"`
	ProcessedAt   *time.Time
	CreatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (Outbox) TableName() string {
	return "outbox"
}
