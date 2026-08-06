package usecase

import (
	"context"
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
)

type ITelegramBotUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.TelegramBotCreateRequest) (*model.TelegramBotResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.TelegramBotResponse, int64, error)
	FindByID(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (*model.TelegramBotResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, req *model.TelegramBotUpdateRequest) (*model.TelegramBotResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) error
	BulkDelete(ctx context.Context, clientID uuid.UUID, ids []uuid.UUID) model.BulkDeleteResult
}

type TelegramBotUseCase struct {
	db              *entity.Database
	botRepo         repository.ITelegramBotRepository
	billingRepo     repository.IClientBillingRepository
	telegramFactory telegram.BotFactory
	log             *zap.Logger
	encryptionKey   string
	webhookBaseURL  string
	webhookSecret   string
}

func NewTelegramBotUseCase(
	db *entity.Database,
	botRepo repository.ITelegramBotRepository,
	billingRepo repository.IClientBillingRepository,
	telegramFactory telegram.BotFactory,
	log *zap.Logger,
	encryptionKey string,
	webhookBaseURL string,
	webhookSecret string,
) ITelegramBotUseCase {
	return &TelegramBotUseCase{
		db:              db,
		botRepo:         botRepo,
		billingRepo:     billingRepo,
		telegramFactory: telegramFactory,
		log:             log,
		encryptionKey:   encryptionKey,
		webhookBaseURL:  webhookBaseURL,
		webhookSecret:   webhookSecret,
	}
}

func (uc *TelegramBotUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.TelegramBotCreateRequest) (*model.TelegramBotResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase create start", zap.String("client_id", clientID.String()))

	// 0. Check quota: ambil active billing plan milik client
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("bot usecase create find billing failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal memeriksa status billing")
	}
	if billing != nil && billing.Plan.MaxBots != -1 {
		currentCount, err := uc.botRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
		if err != nil {
			log.Error("bot usecase create count bots failed", zap.Error(err))
			return nil, fmt.Errorf("Gagal menghitung jumlah bot")
		}
		if currentCount >= int64(billing.Plan.MaxBots) {
			return nil, helper.NewBadRequest(fmt.Sprintf(
				"Kuota bot Anda sudah penuh (%d/%d). Silakan upgrade paket platform untuk menambah lebih banyak bot.",
				currentCount, billing.Plan.MaxBots,
			))
		}
	}

	// 1. Verify token via Telegram API
	tgClient, err := uc.telegramFactory.NewClient(req.Token)
	if err != nil {
		log.Error("bot usecase create tg client init failed", zap.Error(err))
		return nil, helper.NewBadRequest("Token bot tidak valid atau Telegram API tidak dapat dihubungi")
	}

	botInfo := tgClient.GetBot()
	if botInfo.Self.UserName == "" {
		log.Error("bot usecase create tg getme failed")
		return nil, helper.NewBadRequest("Gagal memvalidasi token bot dengan Telegram API")
	}

	// 1.5 Cek apakah bot ini sudah pernah didaftarkan oleh client ini
	existingBot, err := uc.botRepo.FindByBotIDAndClientID(ctx, uc.db.Gorm, botInfo.Self.ID, clientID)
	if err != nil {
		log.Error("bot usecase create check existing bot failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check existing bot")
	}
	if existingBot != nil {
		log.Warn("bot usecase create bot already exists for this client", zap.Int64("bot_id", botInfo.Self.ID), zap.String("client_id", clientID.String()))
		return nil, helper.NewConflict("Bot Telegram ini sudah terdaftar di akun Anda")
	}

	// 2. Encrypt token before saving
	encryptedToken, err := crypto.Encrypt(req.Token, uc.encryptionKey)
	if err != nil {
		log.Error("bot usecase create encrypt token failed", zap.Error(err))
		return nil, fmt.Errorf("failed to encrypt bot token")
	}

	botRole := req.BotRole
	if botRole == "" {
		botRole = "all_in_one"
	}

	bot := &entity.TelegramBot{
		ID:        uuid.New(),
		ClientID:  clientID,
		Token:     encryptedToken,
		Username:  botInfo.Self.UserName,
		BotID:     botInfo.Self.ID,
		BotRole:   botRole,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 3. Persist bot in DB
	if err := uc.botRepo.Create(ctx, uc.db.Gorm, bot); err != nil {
		log.Error("bot usecase create transaction failed", zap.Error(err))
		return nil, fmt.Errorf("create bot in db: %w", err)
	}

	// 4. Configure webhook outside DB transaction to avoid external I/O inside tx.
	webhookURL := fmt.Sprintf("%s/webhooks/telegram/%s", uc.webhookBaseURL, bot.ID.String())
	if err := tgClient.SetWebhook(ctx, webhookURL, uc.webhookSecret); err != nil {
		log.Error("bot usecase create set webhook failed, rolling back bot record", zap.Error(err), zap.String("bot_id", bot.ID.String()))
		if delErr := uc.botRepo.Delete(ctx, uc.db.Gorm, bot); delErr != nil {
			log.Error("bot usecase create rollback delete failed", zap.Error(delErr), zap.String("bot_id", bot.ID.String()))
		}
		return nil, fmt.Errorf("set webhook: %w", err)
	}

	log.Info("bot usecase create success", zap.String("bot_id", bot.ID.String()))
	return converter.TelegramBotToResponse(bot), nil
}

func (uc *TelegramBotUseCase) FindAllByClient(ctx context.Context, clientID uuid.UUID, page, limit int) ([]model.TelegramBotResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase find all start", zap.String("client_id", clientID.String()))

	bots, err := uc.botRepo.FindByClientID(ctx, uc.db.Gorm, clientID, page, limit)
	if err != nil {
		log.Error("bot usecase find all failed", zap.Error(err))
		return nil, 0, err
	}

	total, err := uc.botRepo.CountByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("bot usecase count failed", zap.Error(err))
		return nil, 0, err
	}

	return converter.TelegramBotsToResponse(bots), total, nil
}

func (uc *TelegramBotUseCase) FindByID(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (*model.TelegramBotResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase find by id start", zap.String("bot_id", botID.String()))

	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil {
		log.Error("bot usecase find by id failed", zap.Error(err))
		return nil, err
	}
	if bot == nil {
		return nil, helper.NewNotFound("Bot tidak ditemukan")
	}

	if bot.ClientID != clientID {
		return nil, helper.NewNotFound("Bot tidak ditemukan")
	}

	return converter.TelegramBotToResponse(bot), nil
}

func (uc *TelegramBotUseCase) Update(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, req *model.TelegramBotUpdateRequest) (*model.TelegramBotResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase update start", zap.String("bot_id", botID.String()))

	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil {
		log.Error("bot usecase update find failed", zap.Error(err))
		return nil, err
	}
	if bot == nil {
		return nil, helper.NewNotFound("Bot tidak ditemukan")
	}

	if bot.ClientID != clientID {
		return nil, helper.NewNotFound("Bot tidak ditemukan")
	}

	isActiveChanged := false
	if req.IsActive != nil {
		isActiveChanged = bot.IsActive != *req.IsActive
		bot.IsActive = *req.IsActive
	}
	if req.BotRole != nil {
		bot.BotRole = *req.BotRole
	}
	bot.UpdatedAt = time.Now()

	if err := uc.botRepo.Update(ctx, uc.db.Gorm, bot); err != nil {
		log.Error("bot usecase update failed", zap.Error(err))
		return nil, err
	}

	if isActiveChanged {
		token, decErr := crypto.Decrypt(bot.Token, uc.encryptionKey)
		if decErr != nil {
			log.Warn("bot usecase update failed to decrypt token for webhook sync", zap.Error(decErr), zap.String("bot_id", botID.String()))
		} else {
			tgClient, tgErr := uc.telegramFactory.NewClient(token)
			if tgErr != nil {
				log.Warn("bot usecase update failed to init telegram client for webhook sync", zap.Error(tgErr), zap.String("bot_id", botID.String()))
			} else {
				if bot.IsActive {
					webhookURL := fmt.Sprintf("%s/webhooks/telegram/%s", uc.webhookBaseURL, bot.ID.String())
					if err := tgClient.SetWebhook(ctx, webhookURL, uc.webhookSecret); err != nil {
						log.Warn("bot usecase update failed to set webhook on activation", zap.Error(err), zap.String("bot_id", botID.String()))
					}
				} else {
					if err := tgClient.DeleteWebhook(ctx); err != nil {
						log.Warn("bot usecase update failed to delete webhook on deactivation", zap.Error(err), zap.String("bot_id", botID.String()))
					}
				}
			}
		}
	}

	log.Info("bot usecase update success", zap.String("bot_id", botID.String()))
	return converter.TelegramBotToResponse(bot), nil
}

func (uc *TelegramBotUseCase) Delete(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase delete start", zap.String("bot_id", botID.String()))

	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, botID)
	if err != nil {
		log.Error("bot usecase delete find failed", zap.Error(err))
		return err
	}
	if bot == nil {
		return helper.NewNotFound("Bot tidak ditemukan")
	}

	if bot.ClientID != clientID {
		return helper.NewNotFound("Bot tidak ditemukan")
	}

	// 1. Decrypt token to talk to Telegram
	token, err := crypto.Decrypt(bot.Token, uc.encryptionKey)
	if err == nil {
		tgClient, tgErr := uc.telegramFactory.NewClient(token)
		if tgErr == nil {
			// 2. Delete webhook
			if wbErr := tgClient.DeleteWebhook(ctx); wbErr != nil {
				log.Warn("bot usecase delete failed to remove webhook", zap.String("bot_id", bot.ID.String()), zap.Error(wbErr))
			}
		} else {
			log.Warn("bot usecase delete failed to init bot client for webhook removal", zap.Error(tgErr))
		}
	} else {
		log.Warn("bot usecase delete failed to decrypt token for webhook removal", zap.Error(err))
	}

	// 3. Delete from DB (Soft Delete if we want, but usually bot delete is hard or soft depending on DB. We'll use repo delete)
	// We need a Delete method in repo
	if err := uc.botRepo.Delete(ctx, uc.db.Gorm, bot); err != nil {
		log.Error("bot usecase delete failed", zap.Error(err))
		return err
	}

	log.Info("bot usecase delete success", zap.String("bot_id", botID.String()))
	return nil
}

func (uc *TelegramBotUseCase) BulkDelete(ctx context.Context, clientID uuid.UUID, ids []uuid.UUID) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.Delete(ctx, clientID, id)
	})
}
