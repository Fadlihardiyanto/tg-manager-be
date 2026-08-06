package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IAdminRoleUseCase interface {
	// Role management
	ListRoles(ctx context.Context, req *model.AdminRoleListRequest) ([]model.AdminRoleResponse, error)
	GetRole(ctx context.Context, req *model.AdminRoleGetRequest) (*model.AdminRoleResponse, error)
	CreateRole(ctx context.Context, req *model.AdminRoleCreateRequest) (*model.AdminRoleResponse, error)
	UpdateRole(ctx context.Context, req *model.AdminRoleUpdateRequest) (*model.AdminRoleResponse, error)
	DeleteRole(ctx context.Context, req *model.AdminRoleDeleteRequest) error
	BulkDeleteRoles(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult

	// Permission assignment
	SyncRolePermissions(ctx context.Context, req *model.AdminSyncPermissionsRequest) error

	// Role assignment ke admin user
	AssignRolesToAdmin(ctx context.Context, req *model.AdminAssignRolesRequest) error
	AssignRoleToAdmin(ctx context.Context, req *model.AdminAssignRoleRequest) error
	RevokeRoleFromAdmin(ctx context.Context, req *model.AdminRevokeRoleRequest) error
}

type adminRoleUseCase struct {
	db             *entity.Database
	roleRepo       repository.IAdminRoleRepository
	permissionRepo repository.IAdminPermissionRepository
	log            *zap.Logger
}

func NewAdminRoleUseCase(
	db *entity.Database,
	roleRepo repository.IAdminRoleRepository,
	permissionRepo repository.IAdminPermissionRepository,
	log *zap.Logger,
) IAdminRoleUseCase {
	return &adminRoleUseCase{
		db:             db,
		roleRepo:       roleRepo,
		permissionRepo: permissionRepo,
		log:            log,
	}
}

func (uc *adminRoleUseCase) ListRoles(ctx context.Context, req *model.AdminRoleListRequest) ([]model.AdminRoleResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role list start")

	// Layer 2: cek permission di usecase
	if err := requirePermission(req.CallerPermissions, "roles.read"); err != nil {
		log.Warn("admin role list forbidden", zap.Error(err))
		return nil, err
	}

	roles, err := uc.roleRepo.FindAll(ctx, uc.db.Gorm)
	if err != nil {
		log.Error("admin role list query failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role list success", zap.Int("count", len(roles)))

	return toRoleResponses(roles), nil
}

func (uc *adminRoleUseCase) GetRole(ctx context.Context, req *model.AdminRoleGetRequest) (*model.AdminRoleResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role get start", zap.String("role_id", req.RoleID.String()))

	if err := requirePermission(req.CallerPermissions, "roles.read"); err != nil {
		log.Warn("admin role get forbidden", zap.Error(err))
		return nil, err
	}

	role, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, req.RoleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin role not found", zap.String("role_id", req.RoleID.String()))
			return nil, helper.NewNotFound("role")
		}
		log.Error("admin role get query failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role get success", zap.String("role_id", req.RoleID.String()))

	resp := toRoleResponse(role)
	return &resp, nil
}

func (uc *adminRoleUseCase) CreateRole(ctx context.Context, req *model.AdminRoleCreateRequest) (*model.AdminRoleResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role create start", zap.String("name", req.Name))

	if err := requirePermission(req.CallerPermissions, "roles.create"); err != nil {
		log.Warn("admin role create forbidden", zap.Error(err))
		return nil, err
	}

	// Cek duplikasi nama
	existing, _ := uc.roleRepo.FindByName(ctx, uc.db.Gorm, req.Name)
	if existing != nil {
		log.Warn("admin role create duplicate", zap.String("name", req.Name))
		return nil, helper.NewConflict(fmt.Sprintf("role with name '%s' already exists", req.Name))
	}

	role := &entity.AdminRole{
		ID:          uuid.New(),
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	if err := uc.roleRepo.Create(ctx, uc.db.Gorm, role); err != nil {
		log.Error("admin role create failed", zap.Error(err))
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	// Assign permissions jika ada
	if len(req.PermissionIDs) > 0 {
		if err := uc.roleRepo.SyncPermissions(ctx, uc.db.Gorm, role.ID, req.PermissionIDs); err != nil {
			log.Error("admin role create sync permissions failed", zap.Error(err))
			return nil, fmt.Errorf("failed to assign permissions: %w", err)
		}
	}

	// Fetch ulang untuk dapat relasi yang lengkap
	created, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, role.ID)
	if err != nil {
		log.Error("admin role create reload failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role create success", zap.String("role_id", role.ID.String()))

	resp := toRoleResponse(created)
	return &resp, nil
}

func (uc *adminRoleUseCase) UpdateRole(ctx context.Context, req *model.AdminRoleUpdateRequest) (*model.AdminRoleResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role update start", zap.String("role_id", req.RoleID.String()))

	if err := requirePermission(req.CallerPermissions, "roles.update"); err != nil {
		log.Warn("admin role update forbidden", zap.Error(err))
		return nil, err
	}

	role, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, req.RoleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin role update not found", zap.String("role_id", req.RoleID.String()))
			return nil, helper.NewNotFound("role")
		}
		log.Error("admin role update load failed", zap.Error(err))
		return nil, err
	}

	// Proteksi: role bawaan sistem tidak boleh diubah namanya
	if isSystemRole(role.Name) && req.Name != "" && req.Name != role.Name {
		return nil, helper.NewBadRequest(fmt.Sprintf("cannot rename system role '%s'", role.Name))
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.DisplayName != "" {
		role.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		role.Description = req.Description
	}

	if err := uc.roleRepo.Update(ctx, uc.db.Gorm, role); err != nil {
		log.Error("admin role update save failed", zap.Error(err))
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	// Sync permissions jika dikirim
	if req.PermissionIDs != nil {
		if err := uc.roleRepo.SyncPermissions(ctx, uc.db.Gorm, role.ID, req.PermissionIDs); err != nil {
			log.Error("admin role update sync permissions failed", zap.Error(err))
			return nil, fmt.Errorf("failed to sync permissions: %w", err)
		}
	}

	updated, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, role.ID)
	if err != nil {
		log.Error("admin role update reload failed", zap.Error(err))
		return nil, err
	}
	log.Info("admin role update success", zap.String("role_id", role.ID.String()))

	resp := toRoleResponse(updated)
	return &resp, nil
}

func (uc *adminRoleUseCase) DeleteRole(ctx context.Context, req *model.AdminRoleDeleteRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role delete start", zap.String("role_id", req.RoleID.String()))

	if err := requirePermission(req.CallerPermissions, "roles.delete"); err != nil {
		log.Warn("admin role delete forbidden", zap.Error(err))
		return err
	}

	role, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, req.RoleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin role delete not found", zap.String("role_id", req.RoleID.String()))
			return helper.NewNotFound("role")
		}
		log.Error("admin role delete load failed", zap.Error(err))
		return err
	}

	// Proteksi: role sistem tidak bisa dihapus
	if isSystemRole(role.Name) {
		log.Warn("admin role delete blocked for system role", zap.String("name", role.Name))
		return helper.NewBadRequest(fmt.Sprintf("cannot delete system role '%s'", role.Name))
	}

	if err := uc.roleRepo.Delete(ctx, uc.db.Gorm, req.RoleID); err != nil {
		log.Error("admin role delete failed", zap.Error(err))
		return err
	}
	log.Info("admin role delete success", zap.String("role_id", req.RoleID.String()))
	return nil
}

func (uc *adminRoleUseCase) BulkDeleteRoles(ctx context.Context, ids []uuid.UUID, callerPermissions []string) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.DeleteRole(ctx, &model.AdminRoleDeleteRequest{RoleID: id, CallerPermissions: callerPermissions})
	})
}

// SyncRolePermissions is used for bulk updating permissions of a role, typically in the role edit page in admin panel.
// so when admin updates a role, frontend will send the complete list of permission IDs that should be assigned to the role, and this method will replace the old permissions with the new list.
// example: if a role initially has permissions A, B, C and admin wants to update it to have permissions B, D, then frontend will send [B, D] and this method will remove A and C from the role and assign D.
func (uc *adminRoleUseCase) SyncRolePermissions(ctx context.Context, req *model.AdminSyncPermissionsRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role sync permissions start", zap.String("role_id", req.RoleID.String()))

	if err := requirePermission(req.CallerPermissions, "roles.update"); err != nil {
		log.Warn("admin role sync permissions forbidden", zap.Error(err))
		return err
	}
	if err := uc.roleRepo.SyncPermissions(ctx, uc.db.Gorm, req.RoleID, req.PermissionIDs); err != nil {
		log.Error("admin role sync permissions failed", zap.Error(err))
		return err
	}
	log.Info("admin role sync permissions success", zap.String("role_id", req.RoleID.String()))
	return nil
}

// AssignRolesToAdmin assigns multiple roles to an admin user at once, replacing any existing role assignments.
// This is typically used in the admin user edit page where you can check/uncheck multiple roles for an admin, and when you save, frontend will send the complete list of role IDs that should be assigned to the admin, and this method will sync it accordingly.
func (uc *adminRoleUseCase) AssignRolesToAdmin(ctx context.Context, req *model.AdminAssignRolesRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role assign roles start", zap.String("admin_id", req.AdminID.String()))

	// Assign role hanya bisa dilakukan oleh superadmin
	if !rbac.IsSuperAdmin(req.CallerRoles) {
		log.Warn("admin role assign roles forbidden")
		return helper.NewForbidden("forbidden: only superadmin can assign roles")
	}
	if err := uc.roleRepo.SyncAdminRoles(ctx, uc.db.Gorm, req.AdminID, req.RoleIDs); err != nil {
		log.Error("admin role assign roles failed", zap.Error(err))
		return err
	}
	log.Info("admin role assign roles success", zap.String("admin_id", req.AdminID.String()))
	return nil
}

// AssignRoleToAdmin assigns single role to an admin user
func (uc *adminRoleUseCase) AssignRoleToAdmin(ctx context.Context, req *model.AdminAssignRoleRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role assign single role start", zap.String("admin_id", req.AdminID.String()), zap.String("role_id", req.RoleID.String()))

	// Only superadmin can assign roles
	if !rbac.IsSuperAdmin(req.CallerPermissions) {
		log.Warn("admin role assign single role forbidden")
		return helper.NewForbidden("forbidden: only superadmin can assign roles")
	}

	// Verify role exists
	_, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, req.RoleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin role assign single role not found", zap.String("role_id", req.RoleID.String()))
			return helper.NewNotFound("role")
		}
		log.Error("admin role assign single role load failed", zap.Error(err))
		return err
	}

	if err := uc.roleRepo.AssignRoleToAdmin(ctx, uc.db.Gorm, req.AdminID, req.RoleID); err != nil {
		log.Error("admin role assign single role failed", zap.Error(err))
		return fmt.Errorf("failed to assign role to admin: %w", err)
	}
	log.Info("admin role assign single role success", zap.String("admin_id", req.AdminID.String()), zap.String("role_id", req.RoleID.String()))
	return nil
}

// RevokeRoleFromAdmin revokes single role from an admin user
func (uc *adminRoleUseCase) RevokeRoleFromAdmin(ctx context.Context, req *model.AdminRevokeRoleRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("admin role revoke single role start", zap.String("admin_id", req.AdminID.String()), zap.String("role_id", req.RoleID.String()))

	// Only superadmin can revoke roles
	if !rbac.IsSuperAdmin(req.CallerPermissions) {
		log.Warn("admin role revoke single role forbidden")
		return helper.NewForbidden("forbidden: only superadmin can revoke roles")
	}

	// Verify role exists
	_, err := uc.roleRepo.FindByID(ctx, uc.db.Gorm, req.RoleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn("admin role revoke single role not found", zap.String("role_id", req.RoleID.String()))
			return helper.NewNotFound("role")
		}
		log.Error("admin role revoke single role load failed", zap.Error(err))
		return err
	}

	if err := uc.roleRepo.RevokeRoleFromAdmin(ctx, uc.db.Gorm, req.AdminID, req.RoleID); err != nil {
		log.Error("admin role revoke single role failed", zap.Error(err))
		return fmt.Errorf("failed to revoke role from admin: %w", err)
	}
	log.Info("admin role revoke single role success", zap.String("admin_id", req.AdminID.String()), zap.String("role_id", req.RoleID.String()))
	return nil
}

// ── Private helpers ───────────────────────────────────────────────────────────

// requirePermission adalah layer 2 enforcement di dalam usecase.
// Superadmin otomatis bypass karena permissions-nya sudah include semua
// saat token di-generate di finalizeLogin.
func requirePermission(callerPermissions []string, required string) error {
	if !rbac.HasPermission(callerPermissions, required) {
		return helper.NewForbidden(fmt.Sprintf("forbidden: requires '%s' permission", required))
	}
	return nil
}

// isSystemRole melindungi role bawaan dari modifikasi berbahaya
func isSystemRole(name string) bool {
	systemRoles := map[string]struct{}{
		"superadmin": {},
		"support":    {},
	}
	_, ok := systemRoles[name]
	return ok
}

func toRoleResponse(role *entity.AdminRole) model.AdminRoleResponse {
	resp := model.AdminRoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		DisplayName: role.DisplayName,
		Description: role.Description,
		CreatedAt:   role.CreatedAt,
	}
	for _, p := range role.Permissions {
		resp.Permissions = append(resp.Permissions, model.AdminPermissionResponse{
			ID:          p.ID,
			Name:        p.Name,
			Module:      p.Module,
			Action:      p.Action,
			Description: p.Description,
		})
	}
	return resp
}

func toRoleResponses(roles []entity.AdminRole) []model.AdminRoleResponse {
	result := make([]model.AdminRoleResponse, len(roles))
	for i, r := range roles {
		result[i] = toRoleResponse(&r)
	}
	return result
}
