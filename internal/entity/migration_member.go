package entity

import (
	"time"

	"github.com/google/uuid"
)

type MigrationMember struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID  uuid.UUID `gorm:"type:uuid;index;not null"`
	PackageID uuid.UUID `gorm:"type:uuid;not null"`
	Username  string    `gorm:"type:varchar(255);not null"`
	ExpiredAt time.Time `gorm:"not null"`
	Status    string    `gorm:"type:varchar(20);not null;default:pending"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	ClaimedAt *time.Time

	Client  Client  `gorm:"foreignKey:ClientID"`
	Package Package `gorm:"foreignKey:PackageID"`
}
