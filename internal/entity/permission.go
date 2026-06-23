package entity

import (
	"time"

	"github.com/google/uuid"
)

// Permission represents an action that can be performed in the system
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name        string    `gorm:"type:varchar(100);not null"` // 'packages.create', 'groups.delete', etc
	Module      string    `gorm:"type:varchar(50);not null"`         // 'packages', 'groups', 'bots', 'orders', 'analytics'
	Action      string    `gorm:"type:varchar(50);not null"`         // 'create', 'read', 'update', 'delete'
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt   *time.Time

	// Relationships
	Roles []Role `gorm:"many2many:role_permissions"`
}
