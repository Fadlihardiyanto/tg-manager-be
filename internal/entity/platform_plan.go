package entity

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// PlatformPlan defines subscription tiers available on the platform
type PlatformPlan struct {
	ID           uuid.UUID       `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name         string          `gorm:"type:varchar(50);not null"`
	DisplayName  string          `gorm:"type:varchar(100);not null"`
	PriceMonthly decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0"`
	PriceYearly  decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0"`

	MaxBots           int `gorm:"not null;default:1"`
	MaxGroups         int `gorm:"not null;default:1"`
	MaxPackages       int `gorm:"not null;default:3"`
	MaxMembers        int `gorm:"not null;default:100"`
	MaxCustomCommands int `gorm:"not null;default:5"`
	MaxBroadcasts     int `gorm:"not null;default:3"`

	AllowMediaBroadcast bool `gorm:"not null;default:false"`
	AllowDiscountSystem bool `gorm:"not null;default:false"`
	AllowReportsExport  bool `gorm:"not null;default:false"`
	AllowHighPriority   bool `gorm:"not null;default:false"`
	TransactionLimit    int  `gorm:"not null;default:-1"`

	Features      JSONFeatureList `gorm:"type:jsonb;not null;default:'[]';column:features"`
	IsActive      bool            `gorm:"default:true"`
	IsLandingPage bool            `gorm:"default:true"`
	CreatedAt     time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt     *time.Time
}

func (PlatformPlan) TableName() string {
	return "platform_plans"
}
