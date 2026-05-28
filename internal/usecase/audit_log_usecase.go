package usecase

import (
	"context"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type IAuditLogUseCase interface {
	Record(ctx context.Context, tx *gorm.DB, clientID *uuid.UUID, entityType string, entityID uuid.UUID, action string, actorType string, actorID string, metadata map[string]interface{}) error
	GetLogsByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.AuditLogResponse, int64, error)
	GetPlatformLogs(ctx context.Context, page, limit int) ([]model.AuditLogResponse, int64, error)
}

type auditLogUseCase struct {
	db        *gorm.DB
	auditRepo repository.IAuditLogRepository
	log       *zap.Logger
}

func NewAuditLogUseCase(db *gorm.DB, auditRepo repository.IAuditLogRepository, log *zap.Logger) IAuditLogUseCase {
	return &auditLogUseCase{
		db:        db,
		auditRepo: auditRepo,
		log:       log,
	}
}

func (uc *auditLogUseCase) Record(ctx context.Context, tx *gorm.DB, clientID *uuid.UUID, entityType string, entityID uuid.UUID, action string, actorType string, actorID string, metadata map[string]interface{}) error {
	metaBytes, _ := json.Marshal(metadata)

	log := &entity.AuditLog{
		ClientID:   clientID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		ActorType:  actorType,
		ActorID:    actorID,
		Metadata:   datatypes.JSON(metaBytes),
		CreatedAt:  time.Now(),
	}

	if err := uc.auditRepo.Create(ctx, tx, log); err != nil {
		uc.log.Error("failed to create audit log", zap.Error(err))
		return err
	}
	return nil
}

func (uc *auditLogUseCase) GetLogsByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.AuditLogResponse, int64, error) {
	offset := (page - 1) * limit
	logs, total, err := uc.auditRepo.FindAllByClient(ctx, uc.db, clientID, limit, offset)
	if err != nil {
		uc.log.Error("failed to get client audit logs", zap.Error(err))
		return nil, 0, err
	}

	return uc.toResponseList(logs), total, nil
}

func (uc *auditLogUseCase) GetPlatformLogs(ctx context.Context, page, limit int) ([]model.AuditLogResponse, int64, error) {
	offset := (page - 1) * limit
	logs, total, err := uc.auditRepo.FindAllPlatform(ctx, uc.db, limit, offset)
	if err != nil {
		uc.log.Error("failed to get platform audit logs", zap.Error(err))
		return nil, 0, err
	}

	return uc.toResponseList(logs), total, nil
}

func (uc *auditLogUseCase) toResponseList(logs []entity.AuditLog) []model.AuditLogResponse {
	res := make([]model.AuditLogResponse, len(logs))
	for i, l := range logs {
		var meta map[string]interface{}
		_ = json.Unmarshal(l.Metadata, &meta)

		res[i] = model.AuditLogResponse{
			ID:         l.ID,
			ClientID:   l.ClientID,
			EntityType: l.EntityType,
			EntityID:   l.EntityID,
			Action:     l.Action,
			ActorType:  l.ActorType,
			ActorID:    l.ActorID,
			Metadata:   meta,
			CreatedAt:  l.CreatedAt,
		}
	}
	return res
}
