package entity

import (
	"time"

	"github.com/google/uuid"
)

// User represents a system user (not Telegram user)
type User struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Email           string    `gorm:"type:varchar(255);not null"`
	Name            string    `gorm:"type:varchar(255);not null"`
	PasswordHash    string    `gorm:"type:varchar(255);not null"`
	Phone           string    `gorm:"type:varchar(50)"`
	AvatarURL       string    `gorm:"type:varchar(500)"`
	IsEmailVerified bool      `gorm:"default:false"`
	LastLoginAt     *time.Time
	CreatedAt       time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt       time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt       *time.Time

	// Relationships
	OwnedClients []Client     `gorm:"foreignKey:OwnerUserID"`
	Clients      []Client     `gorm:"many2many:client_users"`
	ClientUsers  []ClientUser `gorm:"foreignKey:UserID"`
}
