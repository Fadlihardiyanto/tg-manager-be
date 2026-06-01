package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// TelegramBotToResponse converts TelegramBot entity to TelegramBotResponse model
func TelegramBotToResponse(bot *entity.TelegramBot) *model.TelegramBotResponse {
	if bot == nil {
		return nil
	}

	return &model.TelegramBotResponse{
		ID:            bot.ID,
		ClientID:      bot.ClientID,
		TelegramBotID: bot.BotID,
		Username:      bot.Username,
		BotRole:       bot.BotRole,
		IsActive:      bot.IsActive,
		CreatedAt:     bot.CreatedAt,
	}
}

// TelegramBotsToResponse converts multiple TelegramBot entities to TelegramBotResponse models
func TelegramBotsToResponse(bots []entity.TelegramBot) []model.TelegramBotResponse {
	if len(bots) == 0 {
		return []model.TelegramBotResponse{}
	}

	responses := make([]model.TelegramBotResponse, len(bots))
	for i, bot := range bots {
		responses[i] = *TelegramBotToResponse(&bot)
	}
	return responses
}

// GroupToResponse converts Group entity to GroupResponse model
func GroupToResponse(group *entity.Group) *model.GroupResponse {
	if group == nil {
		return nil
	}

	return &model.GroupResponse{
		ID:             group.ID,
		ClientID:       group.ClientID,
		BotID:          group.BotID,
		TelegramChatID: group.TelegramChatID,
		Name:           group.Name,
		Description:    group.Description,
		MemberCount:    group.MemberCount,
		IsActive:       group.IsActive,
		InactiveReason: group.InactiveReason,
		CreatedAt:      group.CreatedAt,
		UpdatedAt:      group.UpdatedAt,
	}
}

// GroupsToResponse converts multiple Group entities to GroupResponse models
func GroupsToResponse(groups []entity.Group) []model.GroupResponse {
	if len(groups) == 0 {
		return []model.GroupResponse{}
	}

	responses := make([]model.GroupResponse, len(groups))
	for i, group := range groups {
		responses[i] = *GroupToResponse(&group)
	}
	return responses
}
