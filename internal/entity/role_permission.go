package entity

import "github.com/google/uuid"

// RolePermission is the junction table between Role and Permission.
// Hard delete only — no deleted_at (soft delete on junction table is ambiguous).
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`

	// Relationships
	Role       Role       `gorm:"foreignKey:RoleID"`
	Permission Permission `gorm:"foreignKey:PermissionID"`
}
