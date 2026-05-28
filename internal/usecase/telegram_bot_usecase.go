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

type ITelegramBotUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.TelegramBotCreateRequest) (*model.TelegramBotResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID) ([]model.TelegramBotResponse, error)
	FindByID(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) (*model.TelegramBotResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, botID uuid.UUID, req *model.TelegramBotUpdateRequest) (*model.TelegramBotResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) error
}

type TelegramBotUseCase struct {
	db              *entity.Database
	botRepo         repository.ITelegramBotRepository
	telegramFactory telegram.BotFactory
	log             *zap.Logger
	encryptionKey   string
	webhookBaseURL  string
}

func NewTelegramBotUseCase(
	db *entity.Database,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	log *zap.Logger,
	encryptionKey string,
	webhookBaseURL string,
) ITelegramBotUseCase {
	return &TelegramBotUseCase{
		db:              db,
		botRepo:         botRepo,
		telegramFactory: telegramFactory,
		log:             log,
		encryptionKey:   encryptionKey,
		webhookBaseURL:  webhookBaseURL,
	}
}

func (uc *TelegramBotUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.TelegramBotCreateRequest) (*model.TelegramBotResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase create start", zap.String("client_id", clientID.String()))

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

	fmt.Println("Bot Info:", botInfo)

	// 1.5 Cek apakah bot ini sudah pernah didaftarkan
	existingBot, err := uc.botRepo.FindByBotID(ctx, uc.db.Gorm, botInfo.Self.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("bot usecase create check existing bot failed", zap.Error(err))
		return nil, fmt.Errorf("failed to check existing bot")
	}
	if existingBot != nil {
		log.Warn("bot usecase create bot already exists", zap.Int64("bot_id", botInfo.Self.ID))
		return nil, helper.NewConflict("Bot Telegram ini sudah terdaftar di sistem")
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

	// 3. Begin Transaction
	err = uc.db.Gorm.Transaction(func(tx *gorm.DB) error {
		if err := uc.botRepo.Create(ctx, tx, bot); err != nil {
			return fmt.Errorf("create bot in db: %w", err)
		}

		// 4. Set Webhook
		webhookURL := fmt.Sprintf("%s/webhooks/telegram/%s", uc.webhookBaseURL, bot.ID.String())
		// Webhook secret could be derived or just simple string for validation
		secretToken := "my_webhook_secret_here" // Optionally generate dynamically

		if err := tgClient.SetWebhook(ctx, webhookURL, secretToken); err != nil {
			return fmt.Errorf("set webhook: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Error("bot usecase create transaction failed", zap.Error(err))
		return nil, fmt.Errorf("create bot in db: %w", err)
	}

	log.Info("bot usecase create success", zap.String("bot_id", bot.ID.String()))
	return converter.TelegramBotToResponse(bot), nil
}

func (uc *TelegramBotUseCase) FindAllByClient(ctx context.Context, clientID uuid.UUID) ([]model.TelegramBotResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("bot usecase find all start", zap.String("client_id", clientID.String()))

	bots, err := uc.botRepo.FindByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("bot usecase find all failed", zap.Error(err))
		return nil, err
	}

	return converter.TelegramBotsToResponse(bots), nil
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

	if req.IsActive != nil {
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
			_ = tgClient.DeleteWebhook(ctx)
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
