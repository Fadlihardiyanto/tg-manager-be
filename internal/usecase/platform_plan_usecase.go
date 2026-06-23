package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IPlatformPlanUseCase interface {
	List(ctx context.Context, req *model.PlatformPlanFilterRequest, callerPermissions []string) ([]model.PlatformPlanResponse, int64, error)
	GetByID(ctx context.Context, id uuid.UUID, callerPermissions []string) (*model.PlatformPlanResponse, error)
	Create(ctx context.Context, req *model.CreatePlatformPlanRequest, callerPermissions []string) (*model.PlatformPlanResponse, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdatePlatformPlanRequest, callerPermissions []string) (*model.PlatformPlanResponse, error)
	Delete(ctx context.Context, id uuid.UUID, callerPermissions []string) error
	ListPublic(ctx context.Context) ([]model.PlatformPlanResponse, error)
}

type platformPlanUseCase struct {
	db                *entity.Database
	planRepo          repository.IPlatformPlanRepository
	clientBillingRepo repository.IClientBillingRepository
	log               *zap.Logger
}

func NewPlatformPlanUseCase(
	db *entity.Database,
	planRepo repository.IPlatformPlanRepository,
	clientBillingRepo repository.IClientBillingRepository,
	log *zap.Logger,
) IPlatformPlanUseCase {
	return &platformPlanUseCase{
		db:                db,
		planRepo:          planRepo,
		clientBillingRepo: clientBillingRepo,
		log:               log,
	}
}

func (uc *platformPlanUseCase) List(ctx context.Context, req *model.PlatformPlanFilterRequest, callerPermissions []string) ([]model.PlatformPlanResponse, int64, error) {
	if !rbac.HasPermission(callerPermissions, "billing.read") {
		return nil, 0, helper.NewForbidden("forbidden: requires 'billing.read' permission")
	}

	onlyActive := false
	if req.IsActive != nil {
		onlyActive = *req.IsActive
	}

	plans, err := uc.planRepo.FindAll(ctx, uc.db.Gorm, onlyActive, req.IsLandingPage, req.Page, req.Limit)
	if err != nil {
		uc.log.Error("platform plan: list", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to fetch plans")
	}

	total, err := uc.planRepo.CountAll(ctx, uc.db.Gorm, onlyActive, req.IsLandingPage)
	if err != nil {
		uc.log.Error("platform plan: count", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to fetch plans count")
	}

	result := make([]model.PlatformPlanResponse, len(plans))
	for i, p := range plans {
		result[i] = *converter.PlatformPlanToResponse(&p)
	}
	return result, total, nil
}

func (uc *platformPlanUseCase) ListPublic(ctx context.Context) ([]model.PlatformPlanResponse, error) {
	isActive := true
	isLandingPage := true
	plans, err := uc.planRepo.FindAll(ctx, uc.db.Gorm, isActive, &isLandingPage, 0, 0)
	if err != nil {
		uc.log.Error("platform plan: list public", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch public plans")
	}

	result := make([]model.PlatformPlanResponse, len(plans))
	for i, p := range plans {
		result[i] = *converter.PlatformPlanToResponse(&p)
	}
	return result, nil
}

func (uc *platformPlanUseCase) GetByID(ctx context.Context, id uuid.UUID, callerPermissions []string) (*model.PlatformPlanResponse, error) {
	if !rbac.HasPermission(callerPermissions, "billing.read") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.read' permission")
	}

	plan, err := uc.planRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plan")
	}
	if plan == nil {
		return nil, helper.NewNotFound("plan not found")
	}

	resp := *converter.PlatformPlanToResponse(plan)
	return &resp, nil
}

func (uc *platformPlanUseCase) Create(ctx context.Context, req *model.CreatePlatformPlanRequest, callerPermissions []string) (*model.PlatformPlanResponse, error) {
	if !rbac.HasPermission(callerPermissions, "billing.manage") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	// Cek duplikasi nama
	taken, err := uc.planRepo.IsNameTaken(ctx, uc.db.Gorm, req.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to validate plan name")
	}
	if taken {
		return nil, helper.NewConflict(fmt.Sprintf("plan with name '%s' already exists", req.Name))
	}

	now := time.Now()
	var features entity.JSONFeatureList
	if len(req.Features) > 0 {
		features = make(entity.JSONFeatureList, len(req.Features))
		for i, f := range req.Features {
			features[i] = entity.FeatureItem{
				Name:     f.Name,
				Included: f.Included,
			}
		}
	} else {
		features = entity.JSONFeatureList{}
	}

	plan := &entity.PlatformPlan{
		ID:           uuid.New(),
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		PriceMonthly: req.PriceMonthly,
		PriceYearly:  req.PriceYearly,
		MaxBots:      req.MaxBots,
		MaxGroups:    req.MaxGroups,
		MaxPackages:  req.MaxPackages,
		MaxMembers:   req.MaxMembers,
		MaxCustomCommands: req.MaxCustomCommands,
		Features:     features,
		IsActive:     req.IsActive,
		IsLandingPage: req.IsLandingPage,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := uc.planRepo.Create(ctx, uc.db.Gorm, plan); err != nil {
		uc.log.Error("platform plan: create", zap.Error(err))
		return nil, fmt.Errorf("failed to create plan")
	}

	resp := *converter.PlatformPlanToResponse(plan)
	return &resp, nil
}

func (uc *platformPlanUseCase) Update(ctx context.Context, id uuid.UUID, req *model.UpdatePlatformPlanRequest, callerPermissions []string) (*model.PlatformPlanResponse, error) {
	if !rbac.HasPermission(callerPermissions, "billing.manage") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	plan, err := uc.planRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plan")
	}
	if plan == nil {
		return nil, helper.NewNotFound("plan not found")
	}

	// Patch — hanya update field yang dikirim
	if req.DisplayName != "" {
		plan.DisplayName = req.DisplayName
	}
	if req.PriceMonthly != nil {
		plan.PriceMonthly = *req.PriceMonthly
	}
	if req.PriceYearly != nil {
		plan.PriceYearly = *req.PriceYearly
	}
	if req.MaxBots != nil {
		plan.MaxBots = *req.MaxBots
	}
	if req.MaxGroups != nil {
		plan.MaxGroups = *req.MaxGroups
	}
	if req.MaxPackages != nil {
		plan.MaxPackages = *req.MaxPackages
	}
	if req.MaxMembers != nil {
		plan.MaxMembers = *req.MaxMembers
	}
	if req.MaxCustomCommands != nil {
		plan.MaxCustomCommands = *req.MaxCustomCommands
	}
	if req.Features != nil {
		features := make(entity.JSONFeatureList, len(req.Features))
		for i, f := range req.Features {
			features[i] = entity.FeatureItem{
				Name:     f.Name,
				Included: f.Included,
			}
		}
		plan.Features = features
	}
	if req.IsActive != nil {
		plan.IsActive = *req.IsActive
	}
	if req.IsLandingPage != nil {
		plan.IsLandingPage = *req.IsLandingPage
	}
	plan.UpdatedAt = time.Now()

	if err := uc.planRepo.Update(ctx, uc.db.Gorm, plan); err != nil {
		uc.log.Error("platform plan: update", zap.Error(err))
		return nil, fmt.Errorf("failed to update plan")
	}

	resp := *converter.PlatformPlanToResponse(plan)
	return &resp, nil
}

func (uc *platformPlanUseCase) Delete(ctx context.Context, id uuid.UUID, callerPermissions []string) error {
	if !rbac.HasPermission(callerPermissions, "billing.manage") {
		return helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	plan, err := uc.planRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil {
		return fmt.Errorf("failed to fetch plan")
	}
	if plan == nil {
		return helper.NewNotFound("plan not found")
	}

	// Proteksi: plan 'free' tidak boleh dihapus
	if plan.Name == "free" {
		return helper.NewBadRequest("cannot delete the default 'free' plan")
	}

	// TODO: cek apakah ada client aktif yang pakai plan ini sebelum hapus
	clientCount, _ := uc.clientBillingRepo.CountActiveByPlanID(ctx, uc.db.Gorm, id)
	if clientCount > 0 {
		return helper.NewConflict(fmt.Sprintf("cannot delete plan: %d active clients are using this plan", clientCount))
	}

	return uc.planRepo.Delete(ctx, uc.db.Gorm, id)
}
