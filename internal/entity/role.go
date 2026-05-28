package entity

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a user role in the system
type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name        string    `gorm:"type:varchar(50);unique;not null"` // 'owner', 'admin', 'manager', 'viewer'
	DisplayName string    `gorm:"type:varchar(100);not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt   *time.Time

	// Relationships
	Permissions []Permission `gorm:"many2many:role_permissions"`
}
