package usecase

import (
	"context"
	"errors"
	"sort"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IAdminPermissionUseCase interface {
	ListPermissions(ctx context.Context, req *model.AdminPermissionListRequest) ([]model.AdminPermissionModuleGroupResponse, error)
	GetPermission(ctx context.Context, req *model.AdminPermissionGetRequest) (*model.AdminPermissionResponse, error)
}

type adminPermissionUseCase struct {
	db             *entity.Database
	permissionRepo repository.IAdminPermissionRepository
	log            *zap.Logger
}

func NewAdminPermissionUseCase(db *entity.Database, permissionRepo repository.IAdminPermissionRepository, log *zap.Logger) IAdminPermissionUseCase {
	return &adminPermissionUseCase{db: db, permissionRepo: permissionRepo, log: log}
}

func (uc *adminPermissionUseCase) ListPermissions(ctx context.Context, req *model.AdminPermissionListRequest) ([]model.AdminPermissionModuleGroupResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin permission usecase list start")

	// Authz — konsisten dengan sibling admin usecase (katalog permission
	// tidak boleh dibaca oleh admin mana pun).
	if err := requirePermission(req.CallerPermissions, "roles.read"); err != nil {
		return nil, err
	}

	permissions, err := uc.permissionRepo.FindAll(ctx, uc.db.Gorm)
	if err != nil {
		log.Error("admin permission usecase list failed", zap.Error(err))
		return nil, err
	}

	groups := groupPermissions(permissions)
	log.Info("admin permission usecase list success", zap.Int("count", len(groups)))
	return groups, nil
}

func (uc *adminPermissionUseCase) GetPermission(ctx context.Context, req *model.AdminPermissionGetRequest) (*model.AdminPermissionResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin permission usecase get start", zap.String("permission_id", req.PermissionID.String()))

	// Authz — konsisten dengan sibling admin usecase.
	if err := requirePermission(req.CallerPermissions, "roles.read"); err != nil {
		return nil, err
	}

	permission, err := uc.permissionRepo.FindByID(ctx, uc.db.Gorm, req.PermissionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin permission usecase not found", zap.String("permission_id", req.PermissionID.String()))
			return nil, helper.NewNotFound("permission")
		}
		log.Error("admin permission usecase get failed", zap.Error(err))
		return nil, err
	}

	resp := model.AdminPermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Module:      permission.Module,
		Action:      permission.Action,
		Description: permission.Description,
		CreatedAt:   permission.CreatedAt,
	}
	log.Info("admin permission usecase get success", zap.String("permission_id", req.PermissionID.String()))
	return &resp, nil
}

func groupPermissions(permissions []entity.AdminPermission) []model.AdminPermissionModuleGroupResponse {
	moduleMap := make(map[string][]model.AdminPermissionResponse)
	moduleOrder := make([]string, 0)

	for _, permission := range permissions {
		if _, exists := moduleMap[permission.Module]; !exists {
			moduleOrder = append(moduleOrder, permission.Module)
		}

		moduleMap[permission.Module] = append(moduleMap[permission.Module], model.AdminPermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Module:      permission.Module,
			Action:      permission.Action,
			Description: permission.Description,
			CreatedAt:   permission.CreatedAt,
		})
	}

	sort.Strings(moduleOrder)

	result := make([]model.AdminPermissionModuleGroupResponse, 0, len(moduleOrder))
	for _, module := range moduleOrder {
		result = append(result, model.AdminPermissionModuleGroupResponse{
			Module:      module,
			Permissions: moduleMap[module],
		})
	}

	return result
}
