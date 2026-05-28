package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// RoleToResponse converts Role entity to RoleResponse model
func RoleToResponse(role *entity.Role) *model.RoleResponse {
	if role == nil {
		return nil
	}

	return &model.RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		DisplayName: role.DisplayName,
		Description: role.Description,
		CreatedAt:   role.CreatedAt,
	}
}

// RolesToResponse converts multiple Role entities to RoleResponse models
func RolesToResponse(roles []entity.Role) []model.RoleResponse {
	if len(roles) == 0 {
		return []model.RoleResponse{}
	}

	responses := make([]model.RoleResponse, len(roles))
	for i, role := range roles {
		responses[i] = *RoleToResponse(&role)
	}
	return responses
}

// PermissionToResponse converts Permission entity to PermissionResponse model
func PermissionToResponse(permission *entity.Permission) *model.PermissionResponse {
	if permission == nil {
		return nil
	}

	return &model.PermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Module:      permission.Module,
		Action:      permission.Action,
		Description: permission.Description,
		CreatedAt:   permission.CreatedAt,
	}
}

// PermissionsToResponse converts multiple Permission entities to PermissionResponse models
func PermissionsToResponse(permissions []entity.Permission) []model.PermissionResponse {
	if len(permissions) == 0 {
		return []model.PermissionResponse{}
	}

	responses := make([]model.PermissionResponse, len(permissions))
	for i, permission := range permissions {
		responses[i] = *PermissionToResponse(&permission)
	}
	return responses
}
