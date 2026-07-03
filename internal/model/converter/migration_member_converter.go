package converter

import (
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

func MigrationMemberToResponse(m *entity.MigrationMember) *model.MigrationMemberResponse {
	if m == nil {
		return nil
	}
	return &model.MigrationMemberResponse{
		ID:        m.ID,
		ClientID:  m.ClientID,
		PackageID: m.PackageID,
		Username:  m.Username,
		ExpiredAt: m.ExpiredAt,
		Status:    m.Status,
		CreatedAt: m.CreatedAt,
		ClaimedAt: m.ClaimedAt,
	}
}

func MigrationMembersToResponse(members []entity.MigrationMember) []model.MigrationMemberResponse {
	if len(members) == 0 {
		return []model.MigrationMemberResponse{}
	}
	responses := make([]model.MigrationMemberResponse, len(members))
	for i, m := range members {
		responses[i] = *MigrationMemberToResponse(&m)
	}
	return responses
}
