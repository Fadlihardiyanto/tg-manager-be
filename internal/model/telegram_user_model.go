package model

import (
	"time"

	"github.com/google/uuid"
)

// Telegram User Models
type TelegramUserResponse struct {
	ID             uuid.UUID `json:"id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	Username       string    `json:"username"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Phone          string    `json:"phone"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ── Member (telegram_users JOIN subscriptions) ─────────────────────

// AggregatedMemberRow captures the raw SQL aggregation result.
type AggregatedMemberRow struct {
	ID             uuid.UUID  `gorm:"column:id"`
	TelegramUserID int64      `gorm:"column:telegram_user_id"`
	Username       string     `gorm:"column:username"`
	FirstName      string     `gorm:"column:first_name"`
	LastName       string     `gorm:"column:last_name"`
	Phone          string     `gorm:"column:phone"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	GlobalStatus   bool       `gorm:"column:global_status"`
	ActivePackages []byte     `gorm:"column:active_packages"`
	NearestExpiry  *time.Time `gorm:"column:nearest_expiry"`
}

// MemberSubscriptionBrief is the subscription summary embedded in member list/detail.
type MemberSubscriptionBrief struct {
	ID          uuid.UUID  `json:"id"`
	PackageID   uuid.UUID  `json:"package_id"`
	PackageName string     `json:"package_name"`
	Status      string     `json:"status"`
	ActivatedAt time.Time  `json:"activated_at"`
	ExpiredAt   time.Time  `json:"expired_at"`
	AutoRenew   bool       `json:"auto_renew"`
	KickedAt    *time.Time `json:"kicked_at,omitempty"`
}

// MemberResponse is used in the paginated member list.
// Shows the most relevant subscription (active first, then latest expired).
type MemberResponse struct {
	ID             uuid.UUID  `json:"id"`
	TelegramUserID int64      `json:"telegram_user_id"`
	Username       string     `json:"username"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	Phone          string     `json:"phone"`
	GlobalStatus   bool       `json:"global_status"`
	ActivePackages []string   `json:"active_packages"`
	NearestExpiry  *time.Time `json:"nearest_expiry"`
	TotalOrders    int64      `json:"total_orders"`
	CreatedAt      time.Time  `json:"created_at"`
}

// MemberDetailResponse is used for single-member detail.
// Includes ALL subscriptions for full history.
type MemberDetailResponse struct {
	ID             uuid.UUID                 `json:"id"`
	TelegramUserID int64                     `json:"telegram_user_id"`
	Username       string                    `json:"username"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	Phone          string                    `json:"phone"`
	Subscriptions  []MemberSubscriptionBrief `json:"subscriptions"`
	TotalOrders    int64                     `json:"total_orders"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

// MemberFilterRequest holds the query parameters for member list.
type MemberFilterRequest struct {
	Page         int        `json:"page" validate:"omitempty,min=1"`
	Limit        int        `json:"limit" validate:"omitempty,min=1,max=100"`
	Status       string     `json:"status" validate:"omitempty,oneof=active expired all"`
	Search       string     `json:"search" validate:"omitempty,max=100"`
	PackageID    uuid.UUID  `json:"package_id" validate:"omitempty,uuid"`
	JoinedStart        *time.Time `json:"joined_start" validate:"omitempty"`
	JoinedEnd          *time.Time `json:"joined_end" validate:"omitempty"`
	ExpiredStart       *time.Time `json:"expired_start" validate:"omitempty"`
	ExpiredEnd         *time.Time `json:"expired_end" validate:"omitempty"`
	NearestExpiryStart *time.Time `json:"nearest_expiry_start" validate:"omitempty"`
	NearestExpiryEnd   *time.Time `json:"nearest_expiry_end" validate:"omitempty"`
}

// Audit Log Models
type AuditLogResponse struct {
	ID         uuid.UUID              `json:"id"`
	ClientID   *uuid.UUID             `json:"client_id,omitempty"`
	EntityType string                 `json:"entity_type"`
	EntityID   uuid.UUID              `json:"entity_id"`
	Action     string                 `json:"action"`
	ActorType  string                 `json:"actor_type"`
	ActorID    string                 `json:"actor_id"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
}

type AuditLogFilterRequest struct {
	EntityType string    `json:"entity_type" validate:"omitempty"`
	Action     string    `json:"action" validate:"omitempty"`
	DateFrom   time.Time `json:"date_from" validate:"omitempty"`
	DateTo     time.Time `json:"date_to" validate:"omitempty"`
	Limit      int       `json:"limit" validate:"omitempty,min=1,max=100"`
	Offset     int       `json:"offset" validate:"omitempty,min=0"`
}

// ExtendMemberRequest holds the payload for manual expiry extension.
type ExtendMemberRequest struct {
	NewExpiryAt time.Time `json:"new_expiry_at" validate:"required"`
}
