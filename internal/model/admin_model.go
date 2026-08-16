package model

import (
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Auth Request Models
// =============================================================================

// AdminLoginRequest is the payload for POST /admin/auth/login
type AdminLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	ClientIP string `json:"-" validate:"-"`
}

// AdminVerify2FARequest is the payload for POST /admin/auth/2fa/verify
// Used when admin has 2FA enabled — they submit the OTP code sent to their email.
type AdminVerify2FARequest struct {
	// TempToken is the short-lived token issued after password verification
	// (proves the admin already passed step 1)
	TempToken string `json:"temp_token" validate:"required"`
	OTPCode   string `json:"otp_code" validate:"required,len=6"`
	ClientIP  string `json:"-" validate:"-"`
}

// AdminResendOTPRequest is the payload for POST /admin/v1/auth/otp/resend
// Allows admin to request a new OTP code without re-entering their password.
type AdminResendOTPRequest struct {
	TempToken string `json:"temp_token" validate:"required"`
}

// AdminEnable2FARequest is the payload for POST /admin/auth/2fa/enable
type AdminEnable2FARequest struct {
	OTPCode string    `json:"otp_code" validate:"required,len=6"`
	AdminID uuid.UUID `json:"-" validate:"-"`
}

// AdminRefreshTokenRequest is the payload for POST /admin/auth/refresh
type AdminRefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	ClientIP     string `json:"-" validate:"-"`
}

// =============================================================================
// Auth Response Models
// =============================================================================

// AdminLoginResponse is returned after successful login (or partial if 2FA required).
type AdminLoginResponse struct {
	// If Requires2FA is true, only TempToken is set.
	// Client must call /admin/auth/2fa/verify with TempToken + OTP code.
	Requires2FA bool   `json:"requires_2fa"`
	TempToken   string `json:"temp_token,omitempty"`

	// These are only set when login is fully complete (no 2FA, or 2FA already verified)
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"` // seconds until access token expires

	// User information included on successful login
	User  *AdminUserResponse `json:"user,omitempty"`
	Roles []string           `json:"roles,omitempty"`
}

// Admin2FASetupResponse is returned when admin initiates 2FA setup.
// An OTP code is sent to their email — they must verify it to complete setup.
type Admin2FASetupResponse struct {
	Message string `json:"message"` // "OTP code sent to your email"
}

// =============================================================================
// Admin User Models (CRUD)
// =============================================================================

// AdminUserCreateRequest is the payload for creating a new admin user.
type AdminUserCreateRequest struct {
	Email             string   `json:"email" validate:"required,email"`
	Name              string   `json:"name" validate:"required"`
	Password          string   `json:"password" validate:"required,min=8"`
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

// AdminUserUpdateRequest is the payload for updating an admin user.
type AdminUserUpdateRequest struct {
	Name              string    `json:"name" validate:"omitempty"`
	Email             string    `json:"email" validate:"omitempty,email"`
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminUserListRequest struct {
	Offset            int
	Limit             int
	Search            string   `json:"search" validate:"omitempty,max=100"`
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string `json:"-" validate:"-"`
}

type AdminUserGetRequest struct {
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminUserDeleteRequest struct {
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminUserActivateRequest struct {
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminUserDeactivateRequest struct {
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

// AdminUserResponse is the public representation of an admin user.
type AdminUserResponse struct {
	ID               uuid.UUID  `json:"id"`
	Email            string     `json:"email"`
	Name             string     `json:"name"`
	IsActive         bool       `json:"is_active"`
	IsTwoFAEnabled   bool       `json:"is_two_fa_enabled"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP      string     `json:"last_login_ip,omitempty"`
	FailedLoginCount int        `json:"failed_login_count"`
	LockedUntil      *time.Time `json:"locked_until,omitempty"`
	Roles            []string   `json:"roles,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// =============================================================================
// Admin Role / Permission Response Models
// =============================================================================

type AdminRoleCreateRequest struct {
	Name              string      `json:"name" validate:"required,min=2,max=50"`
	DisplayName       string      `json:"display_name" validate:"required,max=100"`
	Description       string      `json:"description"`
	PermissionIDs     []uuid.UUID `json:"permission_ids"`
	CallerPermissions []string    `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminRoleUpdateRequest struct {
	Name              string      `json:"name" validate:"omitempty,min=2,max=50"`
	DisplayName       string      `json:"display_name" validate:"omitempty,max=100"`
	Description       string      `json:"description"`
	PermissionIDs     []uuid.UUID `json:"permission_ids"` // nil = tidak diubah, [] = hapus semua
	RoleID            uuid.UUID   `json:"-" validate:"-"`
	CallerPermissions []string    `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminSyncPermissionsRequest struct {
	PermissionIDs     []uuid.UUID `json:"permission_ids" validate:"required"`
	RoleID            uuid.UUID   `json:"-" validate:"-"`
	CallerPermissions []string    `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminAssignRolesRequest struct {
	RoleIDs     []uuid.UUID `json:"role_ids" validate:"required"`
	AdminID     uuid.UUID   `json:"-" validate:"-"`

	// Dari JWT + request context
	CallerRoles       []string `json:"-" validate:"-"`
	CallerPermissions []string `json:"-" validate:"-"`
}

// =============================================================================
// Admin Role Operational Request Models
// =============================================================================

type AdminRoleListRequest struct {
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminRoleGetRequest struct {
	RoleID            uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminRoleDeleteRequest struct {
	RoleID            uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminAssignRoleRequest struct {
	RoleID            uuid.UUID `json:"role_id" validate:"required"`
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string  `json:"-" validate:"-"`
}

type AdminRevokeRoleRequest struct {
	RoleID            uuid.UUID `json:"role_id" validate:"required"`
	AdminID           uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string  `json:"-" validate:"-"`
}

// =============================================================================
// Admin Auth Operational Request Models
// =============================================================================

type AdminSetup2FARequest struct {
	AdminID uuid.UUID `json:"-" validate:"-"`
}

type AdminLogoutRequest struct {
	AdminID     uuid.UUID `json:"-" validate:"-"`
	AccessToken string    `json:"-" validate:"-"`
}

// =============================================================================
// Admin Tenant/Billing/Analytics Operational Request Models
// =============================================================================

type AdminVerifyEmailRequest struct {
	Token string `json:"-" validate:"required"`
}

type AdminListAllClientsRequest struct {
	ClientID          string   `json:"client_id" validate:"omitempty,uuid4"`
	Name              string   `json:"name" validate:"omitempty,max=200"`
	Slug              string   `json:"slug" validate:"omitempty,max=200"`
	Active            string   `json:"active" validate:"omitempty,oneof=true false"`
	SubscriptionTier  string   `json:"subscription_tier" validate:"omitempty,max=100"`
	Page              int      `json:"page" validate:"min=1"`
	Size              int      `json:"size" validate:"min=1,max=100"`
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminGetClientDetailRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminCreateClientRequest struct {
	Name              string                       `json:"name" validate:"required,min=3"`
	Slug              string                       `json:"slug" validate:"required,min=3"`
	Description       string                       `json:"description" validate:"omitempty"`
	LogoURL           string                       `json:"logo_url" validate:"omitempty,url"`
	SubscriptionTier  string                       `json:"subscription_tier" validate:"omitempty,oneof=free basic pro enterprise"`
	OwnerUserID       *uuid.UUID                   `json:"owner_user_id" validate:"omitempty"`
	OwnerUser         *AdminOwnerUserCreateRequest `json:"owner_user" validate:"omitempty"`
	CallerPermissions []string                     `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminOwnerUserCreateRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=3"`
	Password string `json:"password" validate:"required,min=8"`
	Phone    string `json:"phone" validate:"omitempty,e164"`
}

type AdminUpdateClientRequest struct {
	ClientID          uuid.UUID  `json:"-" validate:"-"`
	Name              string     `json:"name" validate:"omitempty,min=3"`
	Slug              string     `json:"slug" validate:"omitempty,min=3"`
	Description       string     `json:"description" validate:"omitempty"`
	LogoURL           string     `json:"logo_url" validate:"omitempty,url"`
	SubscriptionTier  string     `json:"subscription_tier" validate:"omitempty,oneof=free basic pro enterprise"`
	IsActive          *bool      `json:"is_active" validate:"omitempty"`
	OwnerUserID       *uuid.UUID `json:"owner_user_id" validate:"omitempty"`
	CallerPermissions []string   `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminDeleteClientRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminTenantUserListRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	UserID            string    `json:"user_id" validate:"omitempty,uuid4"`
	Email             string    `json:"email" validate:"omitempty,max=255"`
	Role              string    `json:"role" validate:"omitempty,max=50"`
	Verified          string    `json:"verified" validate:"omitempty,oneof=true false"`
	Page              int       `json:"page" validate:"omitempty,min=1"`
	Limit             int       `json:"limit" validate:"omitempty,min=1,max=100"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminTenantUserGetRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	UserID            uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminTenantUserCreateRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	Email             string    `json:"email" validate:"required,email"`
	Name              string    `json:"name" validate:"required,min=3"`
	Password          string    `json:"password" validate:"required,min=8"`
	Phone             string    `json:"phone" validate:"omitempty,e164"`
	Role              string    `json:"role" validate:"required,oneof=owner admin manager viewer"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminTenantUserUpdateRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	UserID            uuid.UUID `json:"-" validate:"-"`
	Name              string    `json:"name" validate:"omitempty,min=3"`
	Phone             string    `json:"phone" validate:"omitempty,e164"`
	AvatarURL         string    `json:"avatar_url" validate:"omitempty,url"`
	Role              string    `json:"role" validate:"omitempty,oneof=owner admin manager viewer"`
	IsActive          *bool     `json:"is_active" validate:"omitempty"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminTenantUserDeleteRequest struct {
	ClientID          uuid.UUID `json:"-" validate:"-"`
	UserID            uuid.UUID `json:"-" validate:"-"`
	CallerPermissions []string  `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminSuspendClientRequest struct {
	ClientID uuid.UUID
	Reason   string
}

type AdminActivateClientRequest struct {
	ClientID          uuid.UUID
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminDeactivateClientRequest struct {
	ClientID          uuid.UUID
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}

type AdminImpersonateClientActionRequest struct {
	AdminID  uuid.UUID
	ClientID uuid.UUID
	Payload  *AdminImpersonateRequest
}

type AdminListPlansRequest struct {
	IncludeInactive bool
}

type AdminAssignPlanRequest struct {
	ClientID     uuid.UUID  `json:"client_id" validate:"required,uuid"`
	PlanID       uuid.UUID  `json:"plan_id" validate:"required,uuid"`
	BillingCycle string     `json:"billing_cycle" validate:"required,oneof=monthly yearly"`
	StartedAt    *time.Time `json:"started_at"`
	// nil = sekarang
	Note       string `json:"note"`
	IsTrialing bool   `json:"is_trialing"`

	// Diisi dari JWT claims
	AdminID           uuid.UUID `json:"-"`
	CallerPermissions []string  `json:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
}
type AdminPlatformOverviewRequest struct{}

type AdminRevenueReportRequest struct {
	StartDate string
	EndDate   string
}

// ── Response ─────────────────────────────────────────────────

type AdminRoleResponse struct {
	ID          uuid.UUID                 `json:"id"`
	Name        string                    `json:"name"`
	DisplayName string                    `json:"display_name"`
	Description string                    `json:"description"`
	Permissions []AdminPermissionResponse `json:"permissions"`
	CreatedAt   time.Time                 `json:"created_at"`
}

type AdminPermissionResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Module      string    `json:"module"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type AdminPermissionModuleGroupResponse struct {
	Module      string                    `json:"module"`
	Permissions []AdminPermissionResponse `json:"permissions"`
}

type AdminPermissionListRequest struct {
	// Dari JWT + request context
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string `json:"-" validate:"-"`
}

type AdminPermissionGetRequest struct {
	PermissionID uuid.UUID `json:"-" validate:"-"`

	// Dari JWT + request context
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string `json:"-" validate:"-"`
}

// =============================================================================
// Impersonation Models
// =============================================================================

// AdminImpersonateRequest is the payload for POST /admin/clients/:id/impersonate
type AdminImpersonateRequest struct {
	Reason       string     `json:"reason" validate:"required,min=10"`
	TargetUserID *uuid.UUID `json:"target_user_id" validate:"omitempty"`

	// Dari JWT + request context
	CallerPermissions []string `json:"-" validate:"-"`
	CallerRoles       []string              `json:"-" validate:"-"`
	IPAddress         string   `json:"-" validate:"-"`
}

// AdminImpersonateResponse is returned after successful impersonation setup.
type AdminImpersonateResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"` // seconds
	ClientID    string `json:"client_id"`
	ClientName  string `json:"client_name"`
}

// AdminImpersonationLogResponse is the audit trail of impersonation sessions.
type AdminImpersonationLogResponse struct {
	ID           uuid.UUID  `json:"id"`
	AdminUserID  uuid.UUID  `json:"admin_user_id"`
	AdminName    string     `json:"admin_name,omitempty"`
	ClientID     uuid.UUID  `json:"client_id"`
	ClientName   string     `json:"client_name,omitempty"`
	TargetUserID *uuid.UUID `json:"target_user_id,omitempty"`
	Reason       string     `json:"reason"`
	IPAddress    string     `json:"ip_address"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}
