package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ITenantProfileUseCase interface {
	UpdatePaymentSettings(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateRequest) (*model.PaymentSettingsResponse, error)
	GetPaymentSettings(ctx context.Context, clientID uuid.UUID) (*model.PaymentSettingsResponse, error)
	UpdateProfile(ctx context.Context, clientID uuid.UUID, req *model.ClientUpdateRequest) (*model.ClientResponse, error)
	InitiateKeyExchange(ctx context.Context, clientID uuid.UUID, clientPubKey string) (*model.KeyExchangeResponse, error)
	UpdatePaymentSettingsEncrypted(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateEncryptedRequest) (*model.PaymentSettingsResponse, error)
}

type tenantProfileUseCase struct {
	db            *entity.Database
	clientRepo    repository.IClientRepository
	redisClient   *redis.Client
	encryptionKey string
	log           *zap.Logger
}

func NewTenantProfileUseCase(
	db *entity.Database,
	clientRepo repository.IClientRepository,
	redisClient *redis.Client,
	encryptionKey string,
	log *zap.Logger,
) ITenantProfileUseCase {
	return &tenantProfileUseCase{
		db:            db,
		clientRepo:    clientRepo,
		redisClient:   redisClient,
		encryptionKey: encryptionKey,
		log:           log,
	}
}

type ecdhSession struct {
	ServerPrivateKey string `json:"server_private_key"`
	ClientPublicKey  string `json:"client_public_key"`
	Status           string `json:"status"`
}

func (uc *tenantProfileUseCase) InitiateKeyExchange(ctx context.Context, clientID uuid.UUID, clientPubKeyB64 string) (*model.KeyExchangeResponse, error) {
	serverPriv, serverPub, err := crypto.ECDHGenerateKeyPair()
	if err != nil {
		uc.log.Error("failed to generate ECDH key pair", zap.Error(err))
		return nil, helper.NewUnprocessable("Gagal memulai key exchange")
	}

	if _, err := crypto.ECDHDecodePublicKey(clientPubKeyB64); err != nil {
		return nil, helper.NewBadRequest("Client public key tidak valid")
	}

	sessionID := uuid.New()
	session := ecdhSession{
		ServerPrivateKey: crypto.ECDHEncodePrivateKey(serverPriv),
		ClientPublicKey:  clientPubKeyB64,
		Status:           "initiated",
	}

	payload, marshalErr := json.Marshal(session)
	if marshalErr != nil {
		uc.log.Warn("tenant profile: failed to marshal ECDH session", zap.Error(marshalErr))
	}
	key := fmt.Sprintf("ecdh:%s:%s", clientID.String(), sessionID.String())
	if err := uc.redisClient.Set(ctx, key, payload, 5*time.Minute).Err(); err != nil {
		uc.log.Error("failed to store ECDH session in redis", zap.Error(err))
		return nil, helper.NewUnprocessable("Gagal menyimpan sesi key exchange")
	}

	return &model.KeyExchangeResponse{
		SessionID:       sessionID,
		ServerPublicKey: crypto.ECDHEncodePublicKey(serverPub),
	}, nil
}

func (uc *tenantProfileUseCase) UpdatePaymentSettingsEncrypted(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateEncryptedRequest) (*model.PaymentSettingsResponse, error) {
	key := fmt.Sprintf("ecdh:%s:%s", clientID.String(), req.SessionID.String())
	raw, err := uc.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, helper.NewBadRequest("Sesi key exchange tidak valid atau sudah kadaluarsa")
	}

	var session ecdhSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, helper.NewUnprocessable("Gagal membaca sesi key exchange")
	}

	if session.Status != "initiated" {
		return nil, helper.NewBadRequest("Sesi key exchange sudah digunakan")
	}

	serverPriv, err := crypto.ECDHDecodePrivateKey(session.ServerPrivateKey)
	if err != nil {
		return nil, helper.NewUnprocessable("Gagal memproses key exchange")
	}

	clientPub, err := crypto.ECDHDecodePublicKey(session.ClientPublicKey)
	if err != nil {
		return nil, helper.NewUnprocessable("Gagal memproses key exchange")
	}

	sharedSecret, err := crypto.ECDHComputeSharedSecret(serverPriv, clientPub)
	if err != nil {
		return nil, helper.NewUnprocessable("Gagal compute shared secret")
	}

	info, marshalErr := req.SessionID.MarshalBinary()
	if marshalErr != nil {
		uc.log.Warn("tenant profile: failed to marshal session id", zap.Error(marshalErr))
	}
	derivedKey, err := crypto.ECDHDeriveKey(sharedSecret, info)
	if err != nil {
		return nil, helper.NewUnprocessable("Gagal derive encryption key")
	}

	if err := uc.redisClient.Del(ctx, key).Err(); err != nil {
		uc.log.Warn("failed to delete ECDH session", zap.Error(err))
	}

	encrypted := &paymentSettingsInput{
		isSandbox:              *req.IsSandbox,
		sandboxServerKey:       nil,
		sandboxClientKey:       nil,
		sandboxMerchantID:      nil,
		productionServerKey:    nil,
		productionClientKey:    nil,
		productionMerchantID:   nil,
	}

	if req.SandboxServerKey != "" {
		plain, err := crypto.ECDHDecryptPayload(req.SandboxServerKey, derivedKey)
		if err != nil {
			return nil, helper.NewBadRequest("Gagal mendekripsi sandbox server key")
		}
		encrypted.sandboxServerKey = &plain
	}
	if req.SandboxClientKey != "" {
		plain, err := crypto.ECDHDecryptPayload(req.SandboxClientKey, derivedKey)
		if err != nil {
			return nil, helper.NewBadRequest("Gagal mendekripsi sandbox client key")
		}
		encrypted.sandboxClientKey = &plain
	}
	if req.SandboxMerchantID != "" {
		plain, err := crypto.ECDHDecryptPayload(req.SandboxMerchantID, derivedKey)
		if err != nil {
			return nil, helper.NewBadRequest("Gagal mendekripsi sandbox merchant ID")
		}
		encrypted.sandboxMerchantID = &plain
	}
	if req.ProductionServerKey != "" {
		plain, err := crypto.ECDHDecryptPayload(req.ProductionServerKey, derivedKey)
		if err != nil {
			return nil, helper.NewBadRequest("Gagal mendekripsi production server key")
		}
		encrypted.productionServerKey = &plain
	}
	if req.ProductionClientKey != "" {
		plain, err := crypto.ECDHDecryptPayload(req.ProductionClientKey, derivedKey)
		if err != nil {
			return nil, helper.NewBadRequest("Gagal mendekripsi production client key")
		}
		encrypted.productionClientKey = &plain
	}
	if req.ProductionMerchantID != "" {
		plain, err := crypto.ECDHDecryptPayload(req.ProductionMerchantID, derivedKey)
		if err != nil {
			return nil, helper.NewBadRequest("Gagal mendekripsi production merchant ID")
		}
		encrypted.productionMerchantID = &plain
	}

	return uc.applyPaymentSettings(ctx, clientID, encrypted)
}

type paymentSettingsInput struct {
	sandboxServerKey, sandboxClientKey, sandboxMerchantID         *string
	productionServerKey, productionClientKey, productionMerchantID *string
	isSandbox bool
}

func (uc *tenantProfileUseCase) applyPaymentSettings(ctx context.Context, clientID uuid.UUID, in *paymentSettingsInput) (*model.PaymentSettingsResponse, error) {
	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, helper.NewNotFound("Klien tidak ditemukan")
	}

	if in.sandboxServerKey != nil && *in.sandboxServerKey != "" {
		if err := midtrans.ValidateServerKey(ctx, *in.sandboxServerKey, true); err != nil {
			return nil, helper.NewBadRequest(fmt.Sprintf("sandbox: %v", err))
		}
		enc, err := crypto.Encrypt(*in.sandboxServerKey, uc.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("gagal mengenkripsi kredensial sandbox")
		}
		client.MidtransSandboxServerKey = &enc
	}
	if in.sandboxClientKey != nil && *in.sandboxClientKey != "" {
		enc, err := crypto.Encrypt(*in.sandboxClientKey, uc.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("gagal mengenkripsi kredensial sandbox")
		}
		client.MidtransSandboxClientKey = &enc
	}
	if in.sandboxMerchantID != nil && *in.sandboxMerchantID != "" {
		client.MidtransSandboxMerchantID = in.sandboxMerchantID
	}
	if in.productionServerKey != nil && *in.productionServerKey != "" {
		if err := midtrans.ValidateServerKey(ctx, *in.productionServerKey, false); err != nil {
			return nil, helper.NewBadRequest(fmt.Sprintf("production: %v", err))
		}
		enc, err := crypto.Encrypt(*in.productionServerKey, uc.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("gagal mengenkripsi kredensial production")
		}
		client.MidtransProductionServerKey = &enc
	}
	if in.productionClientKey != nil && *in.productionClientKey != "" {
		enc, err := crypto.Encrypt(*in.productionClientKey, uc.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("gagal mengenkripsi kredensial production")
		}
		client.MidtransProductionClientKey = &enc
	}
	if in.productionMerchantID != nil && *in.productionMerchantID != "" {
		client.MidtransProductionMerchantID = in.productionMerchantID
	}

	client.MidtransIsSandbox = in.isSandbox

	if err := uc.clientRepo.Update(ctx, uc.db.Gorm, client); err != nil {
		return nil, fmt.Errorf("gagal menyimpan pengaturan pembayaran")
	}

	return toPaymentSettingsResponse(client), nil
}

func (uc *tenantProfileUseCase) UpdatePaymentSettings(ctx context.Context, clientID uuid.UUID, req *model.PaymentSettingsUpdateRequest) (*model.PaymentSettingsResponse, error) {
	uc.log.Info("updating payment settings", zap.String("client_id", clientID.String()))

	return uc.applyPaymentSettings(ctx, clientID, &paymentSettingsInput{
		sandboxServerKey:       req.SandboxServerKey,
		sandboxClientKey:       req.SandboxClientKey,
		sandboxMerchantID:      req.SandboxMerchantID,
		productionServerKey:    req.ProductionServerKey,
		productionClientKey:    req.ProductionClientKey,
		productionMerchantID:   req.ProductionMerchantID,
		isSandbox:              *req.IsSandbox,
	})
}

func (uc *tenantProfileUseCase) GetPaymentSettings(ctx context.Context, clientID uuid.UUID) (*model.PaymentSettingsResponse, error) {
	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, helper.NewNotFound("Klien tidak ditemukan")
	}

	resp := toPaymentSettingsResponse(client)

	if client.MidtransSandboxMerchantID != nil {
		resp.SandboxMerchantID = *client.MidtransSandboxMerchantID
	}
	if client.MidtransSandboxClientKey != nil && *client.MidtransSandboxClientKey != "" {
		if decrypted, err := crypto.Decrypt(*client.MidtransSandboxClientKey, uc.encryptionKey); err == nil {
			resp.SandboxClientKey = maskKey(decrypted)
		}
	}
	resp.SandboxServerKey = maskServerKey()

	if client.MidtransProductionMerchantID != nil {
		resp.ProductionMerchantID = *client.MidtransProductionMerchantID
	}
	if client.MidtransProductionClientKey != nil && *client.MidtransProductionClientKey != "" {
		if decrypted, err := crypto.Decrypt(*client.MidtransProductionClientKey, uc.encryptionKey); err == nil {
			resp.ProductionClientKey = maskKey(decrypted)
		}
	}
	resp.ProductionServerKey = maskServerKey()

	return resp, nil
}

func (uc *tenantProfileUseCase) UpdateProfile(ctx context.Context, clientID uuid.UUID, req *model.ClientUpdateRequest) (*model.ClientResponse, error) {
	uc.log.Info("updating client profile", zap.String("client_id", clientID.String()))

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		uc.log.Error("failed to find client", zap.Error(err))
		return nil, helper.NewNotFound("Klien tidak ditemukan")
	}

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

func toPaymentSettingsResponse(client *entity.Client) *model.PaymentSettingsResponse {
	return &model.PaymentSettingsResponse{
		HasSandboxServerKey:     client.MidtransSandboxServerKey != nil && *client.MidtransSandboxServerKey != "",
		HasSandboxClientKey:     client.MidtransSandboxClientKey != nil && *client.MidtransSandboxClientKey != "",
		HasSandboxMerchantID:    client.MidtransSandboxMerchantID != nil && *client.MidtransSandboxMerchantID != "",
		HasProductionServerKey:  client.MidtransProductionServerKey != nil && *client.MidtransProductionServerKey != "",
		HasProductionClientKey:  client.MidtransProductionClientKey != nil && *client.MidtransProductionClientKey != "",
		HasProductionMerchantID: client.MidtransProductionMerchantID != nil && *client.MidtransProductionMerchantID != "",
		IsSandbox:               client.MidtransIsSandbox,
	}
}

func maskKey(key string) string {
	if len(key) <= 4 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func maskServerKey() string {
	return "********"
}
