package entity

import (
	"time"

	"github.com/google/uuid"
)

// Junction table mapping admin users to roles
type AdminUserRole struct {
	AdminUserID uuid.UUID  `gorm:"type:uuid;primaryKey"`
	AdminRoleID uuid.UUID  `gorm:"type:uuid;primaryKey"`
	AssignedBy  *uuid.UUID `gorm:"type:uuid"`
	AssignedAt  time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP"`
}
