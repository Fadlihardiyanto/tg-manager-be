package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// ClientToResponse converts Client entity to ClientResponse model
func ClientToResponse(client *entity.Client) *model.ClientResponse {
	if client == nil {
		return nil
	}

	return &model.ClientResponse{
		ID:               client.ID,
		Name:             client.Name,
		Slug:             client.Slug,
		Category:         client.Category,
		Description:      client.Description,
		LogoURL:          client.LogoURL,
		OwnerUserID:      client.OwnerUserID,
		IsActive:         client.IsActive,
		SubscriptionTier: client.SubscriptionTier,
		CreatedAt:        client.CreatedAt,
		UpdatedAt:        client.UpdatedAt,
	}
}

// ClientsToResponse converts multiple Client entities to ClientResponse models
func ClientsToResponse(clients []entity.Client) []model.ClientResponse {
	if len(clients) == 0 {
		return []model.ClientResponse{}
	}

	responses := make([]model.ClientResponse, len(clients))
	for i, client := range clients {
		responses[i] = *ClientToResponse(&client)
	}
	return responses
}

// ClientUserToResponse converts ClientUser entity to ClientUserResponse model
func ClientUserToResponse(clientUser *entity.ClientUser) *model.ClientUserResponse {
	if clientUser == nil {
		return nil
	}

	return &model.ClientUserResponse{
		ID:         clientUser.ID,
		ClientID:   clientUser.ClientID,
		UserID:     clientUser.UserID,
		Role:       clientUser.Role,
		IsActive:   clientUser.IsActive,
		AcceptedAt: clientUser.AcceptedAt,
		CreatedAt:  clientUser.CreatedAt,
	}
}

// ClientUsersToResponse converts multiple ClientUser entities to ClientUserResponse models
func ClientUsersToResponse(clientUsers []entity.ClientUser) []model.ClientUserResponse {
	if len(clientUsers) == 0 {
		return []model.ClientUserResponse{}
	}

	responses := make([]model.ClientUserResponse, len(clientUsers))
	for i, clientUser := range clientUsers {
		responses[i] = *ClientUserToResponse(&clientUser)
	}
	return responses
}
