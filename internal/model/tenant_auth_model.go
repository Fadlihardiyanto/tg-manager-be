package model

// TenantRegisterRequest is the payload for POST /api/v1/auth/register
type TenantRegisterRequest struct {
	// User Info
	UserName        string `json:"username" validate:"required,min=3"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	Phone           string `json:"phone" validate:"omitempty,e164"`

	// Client (Business) Info
	BusinessName string `json:"business_name" validate:"required,min=3"`
	BusinessSlug string `json:"business_slug" validate:"required,min=3"`
}

type TenantRegisterResponse struct {
	User   UserResponse   `json:"user"`
	Client ClientResponse `json:"client"`
}

// TenantLoginRequest is the payload for POST /api/v1/auth/login
type TenantLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type TenantVerifyEmailRequest struct {
	Token string `query:"token" validate:"required"`
}

type TenantLoginResponse struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	ExpiresIn    int64          `json:"expires_in"` // seconds until access token expires
	User         UserResponse   `json:"user"`
	Client       ClientResponse `json:"client"`
	Role         string         `json:"role"`
}
