package model

import (
	"time"

	"github.com/google/uuid"
)

// User Models
type UserCreateRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=3"`
	Password string `json:"password" validate:"required,min=8"`
	Phone    string `json:"phone" validate:"omitempty,e164"`
}

type UserResponse struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Phone           string     `json:"phone"`
	AvatarURL       string     `json:"avatar_url"`
	IsEmailVerified bool       `json:"is_email_verified"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserUpdateRequest struct {
	Name      string `json:"name" validate:"omitempty,min=3"`
	Phone     string `json:"phone" validate:"omitempty,e164"`
	AvatarURL string `json:"avatar_url" validate:"omitempty,url"`
}

type UserLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type UserLoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
	Role         string       `json:"role"`
	User         UserResponse `json:"user"`
}
