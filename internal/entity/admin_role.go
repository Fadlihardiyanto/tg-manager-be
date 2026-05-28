package entity

import (
	"time"

	"github.com/google/uuid"
)

type AdminRole struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name        string    `gorm:"type:varchar(50);unique;not null"`
	DisplayName string    `gorm:"type:varchar(100);not null"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt   *time.Time

	Permissions []AdminPermission `gorm:"many2many:admin_role_permissions;"`
}
