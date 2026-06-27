package model

// TenantRegisterRequest is the payload for POST /api/v1/auth/register
// Only user account info is required. Business setup is done via onboarding.
type TenantRegisterRequest struct {
	Name            string `json:"name" validate:"required,min=3"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	Phone           string `json:"phone" validate:"omitempty,e164"`
}

type TenantRegisterResponse struct {
	User UserResponse `json:"user"`
}

// TenantOnboardingRequest is the payload for POST /api/v1/clients/onboarding
// Creates a new business (client) and assigns the authenticated user as owner.
type TenantOnboardingRequest struct {
	BusinessName string `json:"business_name" validate:"required,min=3"`
	BusinessSlug string `json:"business_slug" validate:"required,min=3,max=50,slug"`
	Category     string `json:"category" validate:"required,min=3,max=100"`
}

// TenantOnboardingResponse returns the created client along with a new JWT
// that contains the client_id and owner role.
type TenantOnboardingResponse struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresIn    int64          `json:"expires_in"`
	User         UserResponse   `json:"user"`
	Client       ClientResponse `json:"client"`
	Role         string         `json:"role"`
}

// TenantLoginRequest is the payload for POST /api/v1/auth/login
type TenantLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type TenantVerifyEmailRequest struct {
	Token string `query:"token" validate:"required"`
}

type TenantResendVerificationRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type TenantLoginResponse struct {
	AccessToken     string          `json:"access_token"`
	RefreshToken    string          `json:"refresh_token"`
	ExpiresIn       int64           `json:"expires_in"`
	User            UserResponse    `json:"user"`
	Client          *ClientResponse `json:"client"`
	Role            string          `json:"role"`
	NeedsOnboarding bool            `json:"needs_onboarding"`
}

type TenantMeResponse struct {
	User            UserResponse    `json:"user"`
	Client          *ClientResponse `json:"client"`
	Role            string          `json:"role"`
	Permissions     []string        `json:"permissions"`
	NeedsOnboarding bool            `json:"needs_onboarding"`
}
