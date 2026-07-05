package usecase

import (
	"context"
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type IFeatureGateUseCase interface {
	CanUseFeature(ctx context.Context, clientID uuid.UUID, featureKey string) (bool, error)
	CheckQuota(ctx context.Context, clientID uuid.UUID, resourceType string) (allowed bool, current int64, limit int, err error)
	GetUsage(ctx context.Context, clientID uuid.UUID) (*model.PlatformPlanUsage, error)
}

type FeatureGateUseCase struct {
	db                    *entity.Database
	billingRepo           repository.IClientBillingRepository
	botRepo               repository.ITelegramBotRepository
	groupRepo             repository.ITelegramGroupRepository
	packageRepo           repository.IPackageRepository
	customCommandRepo     repository.ICustomCommandRepository
	broadcastRepo         repository.IBroadcastRepository
	tenantAnalyticsRepo   repository.ITenantAnalyticsRepository
	log                   *zap.Logger
}

func NewFeatureGateUseCase(
	db *entity.Database,
	billingRepo repository.IClientBillingRepository,
	botRepo repository.ITelegramBotRepository,
	groupRepo repository.ITelegramGroupRepository,
	packageRepo repository.IPackageRepository,
	customCommandRepo repository.ICustomCommandRepository,
	broadcastRepo repository.IBroadcastRepository,
	tenantAnalyticsRepo repository.ITenantAnalyticsRepository,
	log *zap.Logger,
) IFeatureGateUseCase {
	return &FeatureGateUseCase{
		db:                  db,
		billingRepo:         billingRepo,
		botRepo:             botRepo,
		groupRepo:           groupRepo,
		packageRepo:         packageRepo,
		customCommandRepo:   customCommandRepo,
		broadcastRepo:       broadcastRepo,
		tenantAnalyticsRepo: tenantAnalyticsRepo,
		log:                 log,
	}
}

var featureKeyMap = map[string]func(*entity.PlatformPlan) bool{
	"allow_media_broadcast": func(p *entity.PlatformPlan) bool { return p.AllowMediaBroadcast },
	"allow_discount_system": func(p *entity.PlatformPlan) bool { return p.AllowDiscountSystem },
	"allow_reports_export":  func(p *entity.PlatformPlan) bool { return p.AllowReportsExport },
	"allow_high_priority":   func(p *entity.PlatformPlan) bool { return p.AllowHighPriority },
}

func (uc *FeatureGateUseCase) CanUseFeature(ctx context.Context, clientID uuid.UUID, featureKey string) (bool, error) {
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return false, fmt.Errorf("gagal memeriksa billing")
	}
	if billing == nil {
		return false, nil
	}

	check, ok := featureKeyMap[featureKey]
	if !ok {
		return false, nil
	}
	return check(&billing.Plan), nil
}

var resourceKeyMap = map[string]func(*entity.PlatformPlan) int{
	"bots":             func(p *entity.PlatformPlan) int { return p.MaxBots },
	"groups":           func(p *entity.PlatformPlan) int { return p.MaxGroups },
	"packages":         func(p *entity.PlatformPlan) int { return p.MaxPackages },
	"custom_commands":  func(p *entity.PlatformPlan) int { return p.MaxCustomCommands },
	"broadcasts":       func(p *entity.PlatformPlan) int { return p.MaxBroadcasts },
}

func (uc *FeatureGateUseCase) CheckQuota(ctx context.Context, clientID uuid.UUID, resourceType string) (bool, int64, int, error) {
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return false, 0, 0, fmt.Errorf("gagal memeriksa billing")
	}
	if billing == nil {
		return false, 0, 0, nil
	}

	getLimit, ok := resourceKeyMap[resourceType]
	if !ok {
		return false, 0, 0, fmt.Errorf("unknown resource type: %s", resourceType)
	}
	limit := getLimit(&billing.Plan)
	if limit == -1 {
		return true, 0, -1, nil
	}

	current, err := uc.countResource(ctx, clientID, resourceType)
	if err != nil {
		return false, 0, limit, err
	}

	return current < int64(limit), current, limit, nil
}

func (uc *FeatureGateUseCase) countResource(ctx context.Context, clientID uuid.UUID, resourceType string) (int64, error) {
	switch resourceType {
	case "bots":
		return uc.botRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	case "groups":
		return uc.groupRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	case "packages":
		return uc.packageRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	case "custom_commands":
		return uc.customCommandRepo.CountByClientID(ctx, uc.db.Gorm, clientID, nil, nil)
	case "broadcasts":
		return uc.broadcastRepo.CountByClientID(ctx, uc.db.Gorm, clientID, nil)
	default:
		return 0, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func (uc *FeatureGateUseCase) GetUsage(ctx context.Context, clientID uuid.UUID) (*model.PlatformPlanUsage, error) {
	bots, err := uc.botRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung bots: %w", err)
	}
	groups, err := uc.groupRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung groups: %w", err)
	}
	packages, err := uc.packageRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung packages: %w", err)
	}
	members, err := uc.tenantAnalyticsRepo.CountActiveMembers(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung members: %w", err)
	}
	commands, err := uc.customCommandRepo.CountByClientID(ctx, uc.db.Gorm, clientID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung commands: %w", err)
	}
	broadcasts, err := uc.broadcastRepo.CountByClientID(ctx, uc.db.Gorm, clientID, nil)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung broadcasts: %w", err)
	}

	return &model.PlatformPlanUsage{
		Bots:           int(bots),
		Groups:         int(groups),
		Packages:       int(packages),
		Members:        int(members),
		CustomCommands: int(commands),
		Broadcasts:     int(broadcasts),
	}, nil
}
