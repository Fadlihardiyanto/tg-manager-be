package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// TenantUserToResponse converts ClientUser entity (with User preloaded) to TenantUserResponse model.
func TenantUserToResponse(clientUser *entity.ClientUser) model.TenantUserResponse {
	resp := model.TenantUserResponse{
		ID:         clientUser.ID,
		ClientID:   clientUser.ClientID,
		UserID:     clientUser.UserID,
		Role:       clientUser.Role,
		IsActive:   clientUser.IsActive,
		AcceptedAt: clientUser.AcceptedAt,
		CreatedAt:  clientUser.CreatedAt,
		User:       *UserToResponse(&clientUser.User),
	}

	return resp
}

// TenantUsersToResponse converts multiple ClientUser entities to TenantUserResponse models.
func TenantUsersToResponse(clientUsers []entity.ClientUser) []model.TenantUserResponse {
	if len(clientUsers) == 0 {
		return []model.TenantUserResponse{}
	}

	responses := make([]model.TenantUserResponse, len(clientUsers))
	for i, clientUser := range clientUsers {
		responses[i] = TenantUserToResponse(&clientUser)
	}
	return responses
}
