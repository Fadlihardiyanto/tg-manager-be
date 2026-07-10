package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/google/uuid"
)

func CustomCommandToResponse(cc *entity.CustomCommand) *model.CustomCommandResponse {
	if cc == nil {
		return nil
	}

	return &model.CustomCommandResponse{
		ID:             cc.ID,
		BotID:          cc.BotUUID,
		BotUsername:    cc.Bot.Username,
		CommandTrigger: cc.CommandTrigger,
		ResponseType:   cc.ResponseType,
		ResponseText:   cc.ResponseText,
		FileUrl:        cc.FileUrl,
		IsActive:       cc.IsActive,
		AccessScope:    cc.AccessScope,
		ChatTypeScope:  cc.ChatTypeScope,
		PackageIDs:     stringsToUUIDs(cc.PackageIDs),
		GroupIDs:       stringsToUUIDs(cc.GroupIDs),
		CreatedAt:      cc.CreatedAt,
		UpdatedAt:      cc.UpdatedAt,
	}
}

func CustomCommandListToResponse(list []entity.CustomCommand) []model.CustomCommandResponse {
	var res []model.CustomCommandResponse
	for _, item := range list {
		res = append(res, *CustomCommandToResponse(&item))
	}
	return res
}

func stringsToUUIDs(ids []string) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		result = append(result, parsed)
	}
	return result
}
