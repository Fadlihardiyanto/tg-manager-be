package entity

import (
	"time"

	"github.com/google/uuid"
)

// AdminUser represents platform operator / superadmin.
// Table: admin_users (separated from tenant users for security isolation).
type AdminUser struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:id"`
	Email            string     `gorm:"type:varchar(255);uniqueIndex;not null;column:email"`
	Name             string     `gorm:"type:varchar(255);not null;column:name"`
	PasswordHash     string     `gorm:"type:varchar(255);not null;column:password_hash"`
	IsActive         bool       `gorm:"default:true;column:is_active"`
	IsTwoFAEnabled   bool       `gorm:"default:false;column:is_two_fa_enabled"`
	TwoFASecret      string     `gorm:"type:text;column:two_fa_secret"` // Not used for email OTP, reserved for future TOTP
	LastLoginAt      *time.Time `gorm:"column:last_login_at"`
	LastLoginIP      string     `gorm:"type:varchar(45);column:last_login_ip"`
	FailedLoginCount int        `gorm:"default:0;column:failed_login_count"`
	LockedUntil      *time.Time `gorm:"column:locked_until"`
	CreatedAt        time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:created_at"`
	UpdatedAt        time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;column:updated_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`

	// Relationships
	Roles []AdminRole `gorm:"many2many:admin_user_roles;foreignKey:ID;joinForeignKey:admin_user_id;References:ID;joinReferences:admin_role_id"`
}

// TableName overrides GORM's default table name.
func (AdminUser) TableName() string {
	return "admin_users"
}
