package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AuditLog struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID   *uuid.UUID     `gorm:"type:uuid;index"`
	EntityType string         `gorm:"type:varchar(50);index"`
	EntityID   uuid.UUID      `gorm:"type:uuid;index"`
	Action     string         `gorm:"type:varchar(50)"`
	ActorType  string         `gorm:"type:varchar(50)"`
	ActorID    string         `gorm:"type:varchar(255)"`
	Metadata   datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt  time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
}
