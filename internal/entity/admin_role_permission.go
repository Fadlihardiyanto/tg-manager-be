package entity

import (
	"github.com/google/uuid"
)

// Junction table for admin_role <-> admin_permission
type AdminRolePermission struct {
	AdminRoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	AdminPermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
}
