package usecase

import (
	"context"
	"fmt"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ITenantProfileUseCase interface {
	UpdatePaymentSettings(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateRequest) (*model.PaymentSettingsResponse, error)
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

	// Encrypt keys
	encryptedServerKey, err := crypto.Encrypt(req.MidtransServerKey, uc.encryptionKey)
	if err != nil {
		uc.log.Error("failed to encrypt server key", zap.Error(err))
		return nil, fmt.Errorf("gagal mengenkripsi kredensial")
	}

	encryptedClientKey, err := crypto.Encrypt(req.MidtransClientKey, uc.encryptionKey)
	if err != nil {
		uc.log.Error("failed to encrypt client key", zap.Error(err))
		return nil, fmt.Errorf("gagal mengenkripsi kredensial")
	}

	client.MidtransServerKey = &encryptedServerKey
	client.MidtransClientKey = &encryptedClientKey
	client.MidtransIsSandbox = *req.MidtransIsSandbox

	err = uc.clientRepo.Update(ctx, uc.db.Gorm, client)
	if err != nil {
		uc.log.Error("failed to update client payment settings", zap.Error(err))
		return nil, fmt.Errorf("gagal menyimpan pengaturan pembayaran")
	}

	return &model.PaymentSettingsResponse{
		MidtransClientKey: req.MidtransClientKey, // Safe to return client key
		MidtransIsSandbox: client.MidtransIsSandbox,
	}, nil
}
