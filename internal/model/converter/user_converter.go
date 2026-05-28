package converter

import (
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// UserToResponse converts User entity to UserResponse model
func UserToResponse(user *entity.User) *model.UserResponse {
	if user == nil {
		return nil
	}

	return &model.UserResponse{
		ID:              user.ID,
		Email:           user.Email,
		Name:            user.Name,
		Phone:           user.Phone,
		AvatarURL:       user.AvatarURL,
		IsEmailVerified: user.IsEmailVerified,
		LastLoginAt:     user.LastLoginAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

// UsersToResponse converts multiple User entities to UserResponse models
func UsersToResponse(users []entity.User) []model.UserResponse {
	if len(users) == 0 {
		return []model.UserResponse{}
	}

	responses := make([]model.UserResponse, len(users))
	for i, user := range users {
		responses[i] = *UserToResponse(&user)
	}
	return responses
}

// UserToLoginResponse converts User entity to UserLoginResponse model with tokens
func UserToLoginResponse(user *entity.User, accessToken string, refreshToken string, expiresAt time.Time, role string) *model.UserLoginResponse {
	if user == nil {
		return nil
	}

	return &model.UserLoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		Role:         role,
		User:         *UserToResponse(user),
	}
}
