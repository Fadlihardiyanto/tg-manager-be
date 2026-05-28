package entity

import (
	"time"

	"github.com/google/uuid"
)

// ClientUser represents the relationship between Client and User with role assignment
type ClientUser struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID   uuid.UUID  `gorm:"type:uuid;index;not null"`
	UserID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	Role       string     `gorm:"type:varchar(50);not null"` // 'owner', 'admin', 'manager', 'viewer'
	IsActive   bool       `gorm:"default:true"`
	InvitedBy  *uuid.UUID `gorm:"type:uuid"`
	InvitedAt  *time.Time
	AcceptedAt *time.Time
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt  *time.Time

	// Relationships
	Client        Client `gorm:"foreignKey:ClientID"`
	User          User   `gorm:"foreignKey:UserID"`
	InvitedByUser *User  `gorm:"foreignKey:InvitedBy"`

	// Unique constraint
	// UNIQUE(client_id, user_id) - handled via migration or database constraints
}
