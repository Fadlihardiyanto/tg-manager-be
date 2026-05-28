package entity

import (
	"time"

	"github.com/google/uuid"
)

type AdminPermission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name        string    `gorm:"type:varchar(100);unique;not null"`
	Module      string    `gorm:"type:varchar(50);not null"`
	Action      string    `gorm:"type:varchar(50);not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt   *time.Time

	Roles []AdminRole `gorm:"many2many:admin_role_permissions;"`
}
