package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// TelegramUserToResponse converts TelegramUser entity to TelegramUserResponse model
func TelegramUserToResponse(user *entity.TelegramUser) *model.TelegramUserResponse {
	if user == nil {
		return nil
	}

	return &model.TelegramUserResponse{
		ID:             user.ID,
		TelegramUserID: user.TelegramUserID,
		Username:       user.Username,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		CreatedAt:      user.CreatedAt,
	}
}

// TelegramUsersToResponse converts multiple TelegramUser entities to TelegramUserResponse models
func TelegramUsersToResponse(users []entity.TelegramUser) []model.TelegramUserResponse {
	if len(users) == 0 {
		return []model.TelegramUserResponse{}
	}

	responses := make([]model.TelegramUserResponse, len(users))
	for i, user := range users {
		responses[i] = *TelegramUserToResponse(&user)
	}
	return responses
}
