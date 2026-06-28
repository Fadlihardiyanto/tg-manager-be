package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IPackageUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.PackageCreateRequest) (*model.PackageResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID, filter model.PackageFilterRequest) ([]model.PackageResponse, int64, error)
	FindByID(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID) (*model.PackageResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID, req *model.PackageUpdateRequest) (*model.PackageResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID) error
	AssociateGroups(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID, req *model.PackageGroupAssociateRequest) error
}

type PackageUseCase struct {
	db          *entity.Database
	packageRepo repository.IPackageRepository
	groupRepo   repository.ITelegramGroupRepository
	billingRepo repository.IClientBillingRepository
	log         *zap.Logger
}

func NewPackageUseCase(
	db *entity.Database,
	packageRepo repository.IPackageRepository,
	groupRepo repository.ITelegramGroupRepository,
	billingRepo repository.IClientBillingRepository,
	log *zap.Logger,
) IPackageUseCase {
	return &PackageUseCase{
		db:          db,
		packageRepo: packageRepo,
		groupRepo:   groupRepo,
		billingRepo: billingRepo,
		log:         log,
	}
}

func (uc *PackageUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.PackageCreateRequest) (*model.PackageResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("package usecase create start", zap.String("client_id", clientID.String()))

	// 1. Check quota: ambil active billing plan milik client
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("package usecase create find billing failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal memeriksa status billing")
	}
	if billing != nil && billing.Plan.MaxPackages != -1 {
		currentCount, err := uc.packageRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
		if err != nil {
			log.Error("package usecase create count packages failed", zap.Error(err))
			return nil, fmt.Errorf("Gagal menghitung jumlah paket")
		}
		if currentCount >= int64(billing.Plan.MaxPackages) {
			return nil, helper.NewBadRequest(fmt.Sprintf(
				"Kuota paket Anda sudah penuh (%d/%d). Silakan upgrade paket platform untuk menambah lebih banyak paket.",
				currentCount, billing.Plan.MaxPackages,
			))
		}
	}

	pkg := &entity.Package{
		ID:           uuid.New(),
		ClientID:     clientID,
		Name:         req.Name,
		Price:        req.Price,
		DurationDays: req.DurationDays,
		IsAllAccess:  req.IsAllAccess,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := uc.packageRepo.Create(ctx, uc.db.Gorm, pkg); err != nil {
		log.Error("package usecase create save failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal menyimpan data paket")
	}

	log.Info("package usecase create success", zap.String("package_id", pkg.ID.String()))
	return converter.PackageToResponse(pkg), nil
}

func (uc *PackageUseCase) FindAllByClient(ctx context.Context, clientID uuid.UUID, filter model.PackageFilterRequest) ([]model.PackageResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("package usecase find all start", zap.String("client_id", clientID.String()))

	packages, err := uc.packageRepo.FindPackages(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("package usecase find all failed", zap.Error(err))
		return nil, 0, err
	}

	total, err := uc.packageRepo.CountPackages(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("package usecase count failed", zap.Error(err))
		return nil, 0, err
	}

	return converter.PackagesToResponse(packages), total, nil
}

func (uc *PackageUseCase) FindByID(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID) (*model.PackageResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("package usecase find by id start", zap.String("package_id", packageID.String()))

	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, packageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Paket tidak ditemukan")
		}
		log.Error("package usecase find by id failed", zap.Error(err))
		return nil, err
	}

	if pkg.ClientID != clientID {
		return nil, helper.NewNotFound("Paket tidak ditemukan")
	}

	return converter.PackageToResponse(pkg), nil
}

func (uc *PackageUseCase) Update(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID, req *model.PackageUpdateRequest) (*model.PackageResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("package usecase update start", zap.String("package_id", packageID.String()))

	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, packageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Paket tidak ditemukan")
		}
		log.Error("package usecase update find failed", zap.Error(err))
		return nil, err
	}

	if pkg.ClientID != clientID {
		return nil, helper.NewNotFound("Paket tidak ditemukan")
	}

	if req.Name != "" {
		pkg.Name = req.Name
	}
	if req.Price.GreaterThanOrEqual(req.Price.Sub(req.Price)) { // Simple check since minimum is 0 in validation
		pkg.Price = req.Price
	}
	if req.DurationDays > 0 {
		pkg.DurationDays = req.DurationDays
	}
	if req.IsAllAccess != nil {
		pkg.IsAllAccess = *req.IsAllAccess
	}
	if req.IsActive != nil {
		pkg.IsActive = *req.IsActive
	}
	pkg.UpdatedAt = time.Now()

	if err := uc.packageRepo.Update(ctx, uc.db.Gorm, pkg); err != nil {
		log.Error("package usecase update save failed", zap.Error(err))
		return nil, err
	}

	log.Info("package usecase update success", zap.String("package_id", packageID.String()))
	return converter.PackageToResponse(pkg), nil
}

func (uc *PackageUseCase) Delete(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("package usecase delete start", zap.String("package_id", packageID.String()))

	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, packageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("Paket tidak ditemukan")
		}
		log.Error("package usecase delete find failed", zap.Error(err))
		return err
	}

	if pkg.ClientID != clientID {
		return helper.NewNotFound("Paket tidak ditemukan")
	}

	if err := uc.packageRepo.Delete(ctx, uc.db.Gorm, pkg); err != nil {
		log.Error("package usecase delete failed", zap.Error(err))
		return err
	}

	log.Info("package usecase delete success", zap.String("package_id", packageID.String()))
	return nil
}

func (uc *PackageUseCase) AssociateGroups(ctx context.Context, clientID uuid.UUID, packageID uuid.UUID, req *model.PackageGroupAssociateRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("package usecase associate start", zap.String("package_id", packageID.String()))

	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, packageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("Paket tidak ditemukan")
		}
		log.Error("package usecase associate find pkg failed", zap.Error(err))
		return err
	}

	if pkg.ClientID != clientID {
		return helper.NewNotFound("Paket tidak ditemukan")
	}

	var groups []entity.Group
	for _, groupID := range req.GroupIDs {
		group, err := uc.groupRepo.FindByID(ctx, uc.db.Gorm, groupID)
		if err != nil {
			log.Warn("package usecase associate find group failed", zap.String("group_id", groupID.String()), zap.Error(err))
			return helper.NewBadRequest("Salah satu grup tidak ditemukan")
		}

		if group.ClientID != clientID {
			return helper.NewBadRequest("Salah satu grup tidak valid")
		}
		groups = append(groups, *group)
	}

	if err := uc.packageRepo.AssociateGroups(ctx, uc.db.Gorm, pkg, groups); err != nil {
		log.Error("package usecase associate save failed", zap.Error(err))
		return fmt.Errorf("Gagal mengaitkan paket dengan grup")
	}

	log.Info("package usecase associate success", zap.String("package_id", packageID.String()), zap.Int("group_count", len(groups)))
	return nil
}
