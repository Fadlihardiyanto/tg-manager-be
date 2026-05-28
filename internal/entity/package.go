package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Package struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	ClientID     uuid.UUID       `gorm:"type:uuid;index;not null"`
	Name         string          `gorm:"type:varchar(255);not null"`
	Description  string          `gorm:"type:text"`
	Price        decimal.Decimal `gorm:"type:decimal(12,2);not null"`
	DurationDays int             `gorm:"not null"`
	IsAllAccess  bool            `gorm:"default:false"`
	IsActive     bool            `gorm:"default:true"`
	CreatedAt    time.Time       `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time       `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt    *time.Time

	// Relationships
	Client Client  `gorm:"foreignKey:ClientID"`
	Groups []Group `gorm:"many2many:package_groups;"`
}
