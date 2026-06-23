package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ITenantProfileUseCase interface {
	UpdatePaymentSettings(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateRequest) (*model.PaymentSettingsResponse, error)
	UpdateProfile(ctx context.Context, clientID uuid.UUID, req *model.ClientUpdateRequest) (*model.ClientResponse, error)
}

type tenantProfileUseCase struct {
	db            *entity.Database
	clientRepo    repository.IClientRepository
	encryptionKey string
	log           *zap.Logger
}

func NewTenantProfileUseCase(
	db *entity.Database,
	clientRepo repository.IClientRepository,
	encryptionKey string,
	log *zap.Logger,
) ITenantProfileUseCase {
	return &tenantProfileUseCase{
		db:            db,
		clientRepo:    clientRepo,
		encryptionKey: encryptionKey,
		log:           log,
	}
}

func (uc *tenantProfileUseCase) UpdatePaymentSettings(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateRequest) (*model.PaymentSettingsResponse, error) {
	uc.log.Info("updating payment settings", zap.String("client_id", clientID.String()))

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		uc.log.Error("failed to find client", zap.Error(err))
		return nil, helper.NewNotFound("Klien tidak ditemukan")
	}

	// Validate and encrypt Sandbox keys if provided
	if req.SandboxServerKey != nil && *req.SandboxServerKey != "" {
		if err := midtrans.ValidateServerKey(ctx, *req.SandboxServerKey, true); err != nil {
			uc.log.Warn("sandbox server key validation failed", zap.Error(err))
			return nil, helper.NewBadRequest(fmt.Sprintf("sandbox : %v", err))
		}
		enc, err := crypto.Encrypt(*req.SandboxServerKey, uc.encryptionKey)
		if err != nil {
			uc.log.Error("failed to encrypt sandbox server key", zap.Error(err))
			return nil, fmt.Errorf("gagal mengenkripsi kredensial sandbox")
		}
		client.MidtransSandboxServerKey = &enc
	}

	if req.SandboxClientKey != nil && *req.SandboxClientKey != "" {
		enc, err := crypto.Encrypt(*req.SandboxClientKey, uc.encryptionKey)
		if err != nil {
			uc.log.Error("failed to encrypt sandbox client key", zap.Error(err))
			return nil, fmt.Errorf("gagal mengenkripsi kredensial sandbox")
		}
		client.MidtransSandboxClientKey = &enc
	}

	if req.SandboxMerchantID != nil && *req.SandboxMerchantID != "" {
		client.MidtransSandboxMerchantID = req.SandboxMerchantID
	}

	// Validate and encrypt Production keys if provided
	if req.ProductionServerKey != nil && *req.ProductionServerKey != "" {
		if err := midtrans.ValidateServerKey(ctx, *req.ProductionServerKey, false); err != nil {
			uc.log.Warn("production server key validation failed", zap.Error(err))
			return nil, helper.NewBadRequest(fmt.Sprintf("production : %v", err))
		}
		enc, err := crypto.Encrypt(*req.ProductionServerKey, uc.encryptionKey)
		if err != nil {
			uc.log.Error("failed to encrypt production server key", zap.Error(err))
			return nil, fmt.Errorf("gagal mengenkripsi kredensial production")
		}
		client.MidtransProductionServerKey = &enc
	}

	if req.ProductionClientKey != nil && *req.ProductionClientKey != "" {
		enc, err := crypto.Encrypt(*req.ProductionClientKey, uc.encryptionKey)
		if err != nil {
			uc.log.Error("failed to encrypt production client key", zap.Error(err))
			return nil, fmt.Errorf("gagal mengenkripsi kredensial production")
		}
		client.MidtransProductionClientKey = &enc
	}

	if req.ProductionMerchantID != nil && *req.ProductionMerchantID != "" {
		client.MidtransProductionMerchantID = req.ProductionMerchantID
	}

	// Update environment preference
	client.MidtransIsSandbox = *req.IsSandbox

	if err := uc.clientRepo.Update(ctx, uc.db.Gorm, client); err != nil {
		uc.log.Error("failed to update client payment settings", zap.Error(err))
		return nil, fmt.Errorf("gagal menyimpan pengaturan pembayaran")
	}

	return &model.PaymentSettingsResponse{
		HasSandboxServerKey:     client.MidtransSandboxServerKey != nil && *client.MidtransSandboxServerKey != "",
		HasSandboxClientKey:     client.MidtransSandboxClientKey != nil && *client.MidtransSandboxClientKey != "",
		HasSandboxMerchantID:    client.MidtransSandboxMerchantID != nil && *client.MidtransSandboxMerchantID != "",
		HasProductionServerKey:  client.MidtransProductionServerKey != nil && *client.MidtransProductionServerKey != "",
		HasProductionClientKey:  client.MidtransProductionClientKey != nil && *client.MidtransProductionClientKey != "",
		HasProductionMerchantID: client.MidtransProductionMerchantID != nil && *client.MidtransProductionMerchantID != "",
		IsSandbox:               client.MidtransIsSandbox,
	}, nil
}

func (uc *tenantProfileUseCase) UpdateProfile(ctx context.Context, clientID uuid.UUID, req *model.ClientUpdateRequest) (*model.ClientResponse, error) {
	uc.log.Info("updating client profile", zap.String("client_id", clientID.String()))

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		uc.log.Error("failed to find client", zap.Error(err))
		return nil, helper.NewNotFound("Klien tidak ditemukan")
	}

	// If slug is being changed, validate availability
	if req.Slug != "" && req.Slug != client.Slug {
		existing, err := uc.clientRepo.FindBySlug(ctx, uc.db.Gorm, req.Slug)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			uc.log.Error("failed to check slug availability", zap.Error(err))
			return nil, fmt.Errorf("gagal memvalidasi slug")
		}
		if existing != nil {
			return nil, helper.NewConflict("Slug sudah digunakan oleh bisnis lain")
		}
		client.Slug = req.Slug
	}

	if req.Name != "" {
		client.Name = req.Name
	}
	if req.Category != "" {
		client.Category = req.Category
	}
	if req.Description != "" {
		client.Description = req.Description
	}
	if req.LogoURL != "" {
		client.LogoURL = req.LogoURL
	}

	if err := uc.clientRepo.Update(ctx, uc.db.Gorm, client); err != nil {
		uc.log.Error("failed to update client profile", zap.Error(err))
		return nil, fmt.Errorf("gagal menyimpan profil bisnis")
	}

	return &model.ClientResponse{
		ID:               client.ID,
		Name:             client.Name,
		Slug:             client.Slug,
		Description:      client.Description,
		LogoURL:          client.LogoURL,
		OwnerUserID:      client.OwnerUserID,
		IsActive:         client.IsActive,
		SubscriptionTier: client.SubscriptionTier,
		CreatedAt:        client.CreatedAt,
		UpdatedAt:        client.UpdatedAt,
	}, nil
}
