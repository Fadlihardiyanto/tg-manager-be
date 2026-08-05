package converter

import (
	"encoding/json"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

func BroadcastToResponse(b *entity.Broadcast) *model.BroadcastResponse {
	if b == nil {
		return nil
	}

	resp := &model.BroadcastResponse{
		ID:           b.ID,
		ClientID:     b.ClientID,
		BotID:        b.BotUUID,
		TargetType:   b.TargetType,
		MessageType:  b.MessageType,
		MessageText:  b.MessageText,
		FileUrl:      b.FileUrl,
		Status:       b.Status,
		TotalTargets: b.TotalTargets,
		SentCount:    b.SentCount,
		FailedCount:  b.FailedCount,
		ScheduledAt:  b.ScheduledAt,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}

	if b.FailedDetails != nil && *b.FailedDetails != "" {
		var details []model.BroadcastFailure
		if err := json.Unmarshal([]byte(*b.FailedDetails), &details); err == nil && len(details) > 0 {
			resp.FailedDetails = &details
		}
	}

	return resp
}

func BroadcastListToResponse(list []entity.Broadcast) []model.BroadcastResponse {
	var res []model.BroadcastResponse
	for _, item := range list {
		res = append(res, *BroadcastToResponse(&item))
	}
	return res
}
