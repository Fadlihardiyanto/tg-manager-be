package usecase

import (
	"context"
	"crypto/rand"
	json "github.com/bytedance/sonic"
	"errors"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ITelegramGroupUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.GroupCreateRequest) (*model.GroupResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.GroupResponse, int64, error)
	FindByID(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID) (*model.GroupResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID, req *model.GroupUpdateRequest) (*model.GroupResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID) error
	GenerateConnectToken(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (string, string, error)
	CheckConnectStatus(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, token string) (string, error)
	SyncMemberCounts(ctx context.Context) error
}

type TelegramGroupUseCase struct {
	db              *entity.Database
	groupRepo       repository.ITelegramGroupRepository
	botRepo         repository.ITelegramBotRepository
	billingRepo     repository.IClientBillingRepository
	telegramFactory telegram.BotFactory
	redisClient     *redis.Client
	log             *zap.Logger
	encryptionKey   string
	syncFunc        func(ctx context.Context)
}

func NewTelegramGroupUseCase(
	db *entity.Database,
	groupRepo repository.ITelegramGroupRepository,
	botRepo repository.ITelegramBotRepository,
	billingRepo repository.IClientBillingRepository,
	telegramFactory telegram.BotFactory,
	redisClient *redis.Client,
	log *zap.Logger,
	encryptionKey string,
	syncFunc func(ctx context.Context),
) ITelegramGroupUseCase {
	return &TelegramGroupUseCase{
		db:              db,
		groupRepo:       groupRepo,
		botRepo:         botRepo,
		billingRepo:     billingRepo,
		telegramFactory: telegramFactory,
		redisClient:     redisClient,
		log:             log,
		encryptionKey:   encryptionKey,
		syncFunc:        syncFunc,
	}
}

func (uc *TelegramGroupUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.GroupCreateRequest) (*model.GroupResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("group usecase create start", zap.String("client_id", clientID.String()))

	// 1. Check quota: ambil active billing plan milik client
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("group usecase create find billing failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal memeriksa status billing")
	}
	if billing != nil && billing.Plan.MaxGroups != -1 {
		currentCount, err := uc.groupRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
		if err != nil {
			log.Error("group usecase create count groups failed", zap.Error(err))
			return nil, fmt.Errorf("Gagal menghitung jumlah grup")
		}
		if currentCount >= int64(billing.Plan.MaxGroups) {
			return nil, helper.NewBadRequest(fmt.Sprintf(
				"Kuota grup Anda sudah penuh (%d/%d). Silakan upgrade paket untuk menambah lebih banyak grup.",
				currentCount, billing.Plan.MaxGroups,
			))
		}
	}

	// 2. Verify Bot belongs to client
	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, req.BotID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Bot tidak ditemukan")
		}
		log.Error("group usecase create find bot failed", zap.Error(err))
		return nil, err
	}

	if bot.ClientID != clientID {
		return nil, helper.NewNotFound("Bot tidak ditemukan")
	}

	// 2. Check if group already exists
	existingGroup, err := uc.groupRepo.FindByTelegramID(ctx, uc.db.Gorm, req.TelegramChatID)
	if err == nil && existingGroup != nil {
		return nil, helper.NewBadRequest("Grup ini sudah terdaftar sebelumnya")
	}

	// 3. Decrypt bot token
	token, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err != nil {
		log.Error("group usecase create decrypt token failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal mendekripsi token bot")
	}

	// 4. Verify bot is admin in the chat via Telegram API
	tgClient, err := uc.telegramFactory.NewClient(token)
	if err != nil {
		log.Error("group usecase create tg client init failed", zap.Error(err))
		return nil, helper.NewBadRequest("Gagal menghubungi Telegram API")
	}

	chatMember, err := tgClient.GetChatMember(ctx, req.TelegramChatID, bot.BotID)
	if err != nil {
		log.Warn("group usecase create get chat member failed", zap.Error(err))
		return nil, helper.NewBadRequest("Gagal memverifikasi bot di grup. Pastikan ID Grup benar dan bot sudah ditambahkan ke grup.")
	}

	if chatMember.Status != "administrator" && chatMember.Status != "creator" {
		log.Warn("group usecase create bot not admin", zap.String("status", chatMember.Status))
		return nil, helper.NewBadRequest("Bot harus dijadikan administrator di grup Telegram tersebut terlebih dahulu.")
	}

	// Note: We might want to verify if the bot has specific admin rights (e.g. invite users, restrict members)
	// For now, checking if it's admin/creator is enough.

	// 5. Create Group
	group := &entity.Group{
		ID:             uuid.New(),
		ClientID:       clientID,
		BotUUID:        bot.ID,
		TelegramChatID: req.TelegramChatID,
		Name:           req.Name,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := uc.groupRepo.Create(ctx, uc.db.Gorm, group); err != nil {
		log.Error("group usecase create save db failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal menyimpan data grup")
	}

	log.Info("group usecase create success", zap.String("group_id", group.ID.String()))
	return converter.GroupToResponse(group), nil
}

func (uc *TelegramGroupUseCase) FindAllByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.GroupResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("group usecase find all start", zap.String("client_id", clientID.String()))

	groups, err := uc.groupRepo.FindByClientID(ctx, uc.db.Gorm, clientID, page, limit)
	if err != nil {
		log.Error("group usecase find all failed", zap.Error(err))
		return nil, 0, err
	}

	total, err := uc.groupRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("group usecase count failed", zap.Error(err))
		return nil, 0, err
	}

	return converter.GroupsToResponse(groups), total, nil
}

func (uc *TelegramGroupUseCase) FindByID(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID) (*model.GroupResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("group usecase find by id start", zap.String("group_id", groupID.String()))

	group, err := uc.groupRepo.FindByID(ctx, uc.db.Gorm, groupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Grup tidak ditemukan")
		}
		log.Error("group usecase find by id failed", zap.Error(err))
		return nil, err
	}

	if group.ClientID != clientID {
		return nil, helper.NewNotFound("Grup tidak ditemukan")
	}

	return converter.GroupToResponse(group), nil
}

func (uc *TelegramGroupUseCase) Update(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID, req *model.GroupUpdateRequest) (*model.GroupResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("group usecase update start", zap.String("group_id", groupID.String()))

	group, err := uc.groupRepo.FindByID(ctx, uc.db.Gorm, groupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Grup tidak ditemukan")
		}
		log.Error("group usecase update find failed", zap.Error(err))
		return nil, err
	}

	if group.ClientID != clientID {
		return nil, helper.NewNotFound("Grup tidak ditemukan")
	}

	if req.Name != "" {
		group.Name = req.Name
	}
	if req.BotID != uuid.Nil {
		// Verify new Bot belongs to client
		bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, req.BotID)
		if err != nil || bot.ClientID != clientID {
			return nil, helper.NewBadRequest("Bot tidak valid")
		}
		group.BotUUID = req.BotID
	}
	group.UpdatedAt = time.Now()

	if err := uc.groupRepo.Update(ctx, uc.db.Gorm, group); err != nil {
		log.Error("group usecase update save failed", zap.Error(err))
		return nil, err
	}

	log.Info("group usecase update success", zap.String("group_id", groupID.String()))
	return converter.GroupToResponse(group), nil
}

func (uc *TelegramGroupUseCase) Delete(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("group usecase delete start", zap.String("group_id", groupID.String()))

	group, err := uc.groupRepo.FindByID(ctx, uc.db.Gorm, groupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("Grup tidak ditemukan")
		}
		log.Error("group usecase delete find failed", zap.Error(err))
		return err
	}

	if group.ClientID != clientID {
		return helper.NewNotFound("Grup tidak ditemukan")
	}

	if err := uc.groupRepo.Delete(ctx, uc.db.Gorm, group); err != nil {
		log.Error("group usecase delete failed", zap.Error(err))
		return err
	}

	log.Info("group usecase delete success", zap.String("group_id", groupID.String()))
	return nil
}

func (uc *TelegramGroupUseCase) GenerateConnectToken(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (string, string, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("generating group connect token", zap.String("client_id", clientID.String()), zap.String("bot_id", botID.String()))

	// 1. Verify Bot belongs to client
	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", helper.NewNotFound("Bot tidak ditemukan")
		}
		log.Error("group usecase generate token find bot failed", zap.Error(err))
		return "", "", err
	}

	if bot.ClientID != clientID {
		return "", "", helper.NewNotFound("Bot tidak ditemukan")
	}

	// 2. Generate random 6 characters code
	code := uc.generateRandomCode(6)

	// 3. Save to Redis
	redisKey := fmt.Sprintf("connect_group:%s", code)
	payload := map[string]interface{}{
		"client_id": clientID,
		"bot_id":    botID,
	}
	val, err := json.Marshal(payload)
	if err != nil {
		log.Error("failed to marshal connect token payload", zap.Error(err))
		return "", "", err
	}

	err = uc.redisClient.Set(ctx, redisKey, string(val), 15*time.Minute).Err()
	if err != nil {
		log.Error("failed to save connect token to redis", zap.Error(err))
		return "", "", fmt.Errorf("Gagal membuat token koneksi")
	}

	return code, bot.Username, nil
}

func (uc *TelegramGroupUseCase) generateRandomCode(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // avoid confusing characters
	b := make([]byte, length)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func (uc *TelegramGroupUseCase) CheckConnectStatus(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, token string) (string, error) {
	// 1. Check if status key in Redis is "success"
	statusKey := fmt.Sprintf("connect_group_status:%s", token)
	status, err := uc.redisClient.Get(ctx, statusKey).Result()
	if err == nil && status == "success" {
		return "success", nil
	}

	// 2. Check if connect token exists in Redis
	redisKey := fmt.Sprintf("connect_group:%s", token)
	val, err := uc.redisClient.Get(ctx, redisKey).Result()
	if err != nil {
		// Token doesn't exist (expired/deleted) or redis error
		return "expired", nil
	}

	// 3. Unmarshal and verify client ID and bot ID directly from Redis payload
	var payload struct {
		ClientID uuid.UUID `json:"client_id"`
		BotID    uuid.UUID `json:"bot_id"`
	}
	if err := json.Unmarshal([]byte(val), &payload); err != nil {
		return "expired", nil
	}

	if payload.ClientID != clientID || payload.BotID != botID {
		return "expired", nil
	}

	return "pending", nil
}

func (uc *TelegramGroupUseCase) SyncMemberCounts(ctx context.Context) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("group usecase sync member counts start")

	go uc.syncFunc(context.Background())

	return nil
}
