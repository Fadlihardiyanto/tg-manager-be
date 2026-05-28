package converter

import (
	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// AuditLogToResponse converts AuditLog entity to AuditLogResponse model
func AuditLogToResponse(auditLog *entity.AuditLog) *model.AuditLogResponse {
	if auditLog == nil {
		return nil
	}

	// Parse metadata from JSON
	var metadata map[string]interface{}
	if len(auditLog.Metadata) > 0 {
		_ = json.Unmarshal(auditLog.Metadata, &metadata)
	}

	return &model.AuditLogResponse{
		ID:         auditLog.ID,
		EntityType: auditLog.EntityType,
		EntityID:   auditLog.EntityID,
		Action:     auditLog.Action,
		Metadata:   metadata,
		CreatedAt:  auditLog.CreatedAt,
	}
}

// AuditLogsToResponse converts multiple AuditLog entities to AuditLogResponse models
func AuditLogsToResponse(auditLogs []entity.AuditLog) []model.AuditLogResponse {
	if len(auditLogs) == 0 {
		return []model.AuditLogResponse{}
	}

	responses := make([]model.AuditLogResponse, len(auditLogs))
	for i, auditLog := range auditLogs {
		responses[i] = *AuditLogToResponse(&auditLog)
	}
	return responses
}
