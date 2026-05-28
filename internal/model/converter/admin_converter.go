package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// AdminUserToResponse converts AdminUser entity to AdminUserResponse model
func AdminUserToResponse(u *entity.AdminUser) *model.AdminUserResponse {
	if u == nil {
		return nil
	}

	return &model.AdminUserResponse{
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
}

// AdminRoleToResponse converts AdminRole entity to AdminRoleResponse model
func AdminRoleToResponse(r *entity.AdminRole) *model.AdminRoleResponse {
	if r == nil {
		return nil
	}

	return &model.AdminRoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
	}
}

// AdminPermissionToResponse converts AdminPermission entity to AdminPermissionResponse model
func AdminPermissionToResponse(p *entity.AdminPermission) *model.AdminPermissionResponse {
	if p == nil {
		return nil
	}

	return &model.AdminPermissionResponse{
		ID:          p.ID,
		Name:        p.Name,
		Module:      p.Module,
		Action:      p.Action,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
	}
}
