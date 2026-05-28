package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// ToAdminUserResponse converts entity.AdminUser to model.AdminUserResponse
func ToAdminUserResponse(u *entity.AdminUser) model.AdminUserResponse {
	resp := model.AdminUserResponse{
		ID:               u.ID,
		Email:            u.Email,
		Name:             u.Name,
		IsActive:         u.IsActive,
		IsTwoFAEnabled:   u.IsTwoFAEnabled,
		LastLoginAt:      u.LastLoginAt,
		LastLoginIP:      u.LastLoginIP,
		FailedLoginCount: u.FailedLoginCount,
		LockedUntil:      u.LockedUntil,
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
	for _, r := range u.Roles {
		resp.Roles = append(resp.Roles, r.Name)
	}
	return resp
}

// ToAdminLoginResponse builds model.AdminLoginResponse given user entity and tokens
func ToAdminLoginResponse(u *entity.AdminUser, accessToken, refreshToken string, expiresIn int64) model.AdminLoginResponse {
	userResp := ToAdminUserResponse(u)
	return model.AdminLoginResponse{
		Requires2FA:  false,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User:         &userResp,
		Roles:        userResp.Roles,
	}
}
