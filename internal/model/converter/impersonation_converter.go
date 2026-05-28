package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// AdminImpersonationLogToResponse converts AdminImpersonationLog entity to AdminImpersonationLogResponse model
func AdminImpersonationLogToResponse(l *entity.AdminImpersonationLog) *model.AdminImpersonationLogResponse {
	if l == nil {
		return nil
	}

	return &model.AdminImpersonationLogResponse{
		ID:           l.ID,
		AdminUserID:  l.AdminUserID,
		ClientID:     l.ClientID,
		TargetUserID: l.TargetUserID,
		Reason:       l.Reason,
		IPAddress:    l.IPAddress,
		StartedAt:    l.StartedAt,
		EndedAt:      l.EndedAt,
	}
}
