package usecase

import (
	"context"
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IAdminImpersonationUseCase interface {
	Start(ctx context.Context, req *model.AdminImpersonateClientActionRequest) (*model.AdminImpersonateResponse, error)
	End(ctx context.Context, adminID, logID uuid.UUID) error
	ListByAdmin(ctx context.Context, adminUserID uuid.UUID, page, limit int, callerPermissions []string) ([]model.AdminImpersonationLogResponse, int64, error)
	ListByClient(ctx context.Context, clientID uuid.UUID, page, limit int, callerPermissions []string) ([]model.AdminImpersonationLogResponse, int64, error)
}

type adminImpersonationUseCase struct {
	db                   *entity.Database
	adminRepo            repository.IAdminUserRepository
	clientRepo           repository.IClientRepository
	impersonationLogRepo repository.IAdminImpersonationLogRepository
	jwtConfig            *pkg_jwt.JWTConfig
	log                  *zap.Logger
}

func NewAdminImpersonationUseCase(
	db *entity.Database,
	adminRepo repository.IAdminUserRepository,
	clientRepo repository.IClientRepository,
	impersonationLogRepo repository.IAdminImpersonationLogRepository,
	jwtConfig *pkg_jwt.JWTConfig,
	log *zap.Logger,
) IAdminImpersonationUseCase {
	// Fail-fast: db/log/jwtConfig dipakai di semua method — zero value =
	// panic di runtime tanpa jejak wiring yang salah.
	if db == nil || log == nil || jwtConfig == nil {
		panic("admin impersonation usecase: db, log and jwtConfig are required")
	}
	return &adminImpersonationUseCase{
		db:                   db,
		adminRepo:            adminRepo,
		clientRepo:           clientRepo,
		impersonationLogRepo: impersonationLogRepo,
		jwtConfig:            jwtConfig,
		log:                  log,
	}
}

func (uc *adminImpersonationUseCase) Start(ctx context.Context, req *model.AdminImpersonateClientActionRequest) (*model.AdminImpersonateResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	if req == nil || req.Payload == nil {
		return nil, helper.NewBadRequest("payload impersonation wajib diisi")
	}
	if !rbac.HasPermission(req.Payload.CallerPermissions, "clients.impersonate") {
		return nil, helper.NewForbiddenPermission("clients.impersonate")
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil || client == nil {
		return nil, helper.NewNotFound("client tidak ditemukan")
	}

	// Tentukan target user (default: owner)
	targetUserID := client.OwnerUserID
	if req.Payload.TargetUserID != nil {
		targetUserID = *req.Payload.TargetUserID
	}

	logEntry := &entity.AdminImpersonationLog{
		ID:           uuid.New(),
		AdminUserID:  req.AdminID,
		ClientID:     req.ClientID,
		TargetUserID: &targetUserID,
		Reason:       req.Payload.Reason,
		IPAddress:    req.Payload.IPAddress,
	}
	if err := uc.impersonationLogRepo.Create(ctx, uc.db.Gorm, logEntry); err != nil {
		log.Error("impersonation: failed to create audit log", zap.Error(err))
		return nil, fmt.Errorf("gagal mencatat impersonation log")
	}

	token, err := pkg_jwt.GenerateImpersonationToken(ctx, req.AdminID, req.ClientID, targetUserID, uc.jwtConfig)
	if err != nil {
		return nil, fmt.Errorf("gagal generate impersonation token")
	}

	return &model.AdminImpersonateResponse{
		AccessToken: token,
		ExpiresIn:   3600, // 1 jam
		ClientID:    req.ClientID.String(),
		ClientName:  client.Name,
	}, nil
}

func (uc *adminImpersonationUseCase) End(ctx context.Context, adminID, logID uuid.UUID) error {
	if err := uc.impersonationLogRepo.EndSession(ctx, uc.db.Gorm, logID); err != nil {
		return fmt.Errorf("gagal mengakhiri sesi impersonation")
	}
	return nil
}

func (uc *adminImpersonationUseCase) ListByAdmin(ctx context.Context, adminUserID uuid.UUID, page, limit int, callerPermissions []string) ([]model.AdminImpersonationLogResponse, int64, error) {
	if !rbac.HasPermission(callerPermissions, "clients.read") {
		return nil, 0, helper.NewForbiddenPermission("clients.read")
	}

	page, limit = clampPagination(page, limit)
	offset := (page - 1) * limit
	logs, total, err := uc.impersonationLogRepo.FindByAdminID(ctx, uc.db.Gorm, adminUserID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal mengambil log impersonation")
	}

	return uc.enrichLogs(ctx, logs), total, nil
}

func (uc *adminImpersonationUseCase) ListByClient(ctx context.Context, clientID uuid.UUID, page, limit int, callerPermissions []string) ([]model.AdminImpersonationLogResponse, int64, error) {
	if !rbac.HasPermission(callerPermissions, "clients.read") {
		return nil, 0, helper.NewForbiddenPermission("clients.read")
	}

	page, limit = clampPagination(page, limit)
	offset := (page - 1) * limit
	logs, total, err := uc.impersonationLogRepo.FindByClientID(ctx, uc.db.Gorm, clientID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal mengambil log impersonation")
	}

	return uc.enrichLogs(ctx, logs), total, nil
}

func (uc *adminImpersonationUseCase) enrichLogs(ctx context.Context, logs []entity.AdminImpersonationLog) []model.AdminImpersonationLogResponse {
	result := make([]model.AdminImpersonationLogResponse, len(logs))

	adminCache := make(map[uuid.UUID]string)
	clientCache := make(map[uuid.UUID]string)

	for i, l := range logs {
		resp := converter.AdminImpersonationLogToResponse(&l)

		if name, ok := adminCache[l.AdminUserID]; ok {
			resp.AdminName = name
		} else {
			var admin entity.AdminUser
			if err := uc.adminRepo.FindById(ctx, uc.db.Gorm, &admin, l.AdminUserID); err == nil {
				adminCache[l.AdminUserID] = admin.Name
				resp.AdminName = admin.Name
			}
		}

		if name, ok := clientCache[l.ClientID]; ok {
			resp.ClientName = name
		} else {
			client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, l.ClientID)
			if err == nil && client != nil {
				clientCache[l.ClientID] = client.Name
				resp.ClientName = client.Name
			}
		}

		result[i] = *resp
	}
	return result
}
