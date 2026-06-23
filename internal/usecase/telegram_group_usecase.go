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
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ITelegramGroupUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.GroupCreateRequest) (*model.GroupResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.GroupResponse, int64, error)
	FindByID(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID) (*model.GroupResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID, req *model.GroupUpdateRequest) (*model.GroupResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, groupID uuid.UUID) error
}

type TelegramGroupUseCase struct {
	db              *entity.Database
	groupRepo       repository.ITelegramGroupRepository
	botRepo         repository.ITelegramBotRepository
	billingRepo     repository.IClientBillingRepository
	telegramFactory telegram.BotFactory
	log             *zap.Logger
	encryptionKey   string
}

func NewTelegramGroupUseCase(
	db *entity.Database,
	groupRepo repository.ITelegramGroupRepository,
	botRepo repository.ITelegramBotRepository,
	billingRepo repository.IClientBillingRepository,
	telegramFactory telegram.BotFactory,
	log *zap.Logger,
	encryptionKey string,
) ITelegramGroupUseCase {
	return &TelegramGroupUseCase{
		db:              db,
		groupRepo:       groupRepo,
		botRepo:         botRepo,
		billingRepo:     billingRepo,
		telegramFactory: telegramFactory,
		log:             log,
		encryptionKey:   encryptionKey,
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
		BotID:          bot.ID,
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
		group.BotID = req.BotID
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
