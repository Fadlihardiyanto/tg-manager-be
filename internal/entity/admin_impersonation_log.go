package entity

import (
	"time"

	"github.com/google/uuid"
)

// AdminImpersonationLog records when an admin impersonates a client/user
type AdminImpersonationLog struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	AdminUserID  uuid.UUID  `gorm:"type:uuid;index;not null"`
	ClientID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	TargetUserID *uuid.UUID `gorm:"type:uuid"`
	Reason       string     `gorm:"type:text"`
	IPAddress    string     `gorm:"type:varchar(45)"`
	StartedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP"`
	EndedAt      *time.Time
}
