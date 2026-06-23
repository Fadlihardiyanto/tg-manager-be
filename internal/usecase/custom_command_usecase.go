package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type ICustomCommandUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.CreateCustomCommandRequest) (*model.CustomCommandResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID, req *model.CustomCommandFilterRequest) ([]model.CustomCommandResponse, int64, error)
	FindByID(ctx context.Context, clientID uuid.UUID, id uuid.UUID) (*model.CustomCommandResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, id uuid.UUID, req *model.UpdateCustomCommandRequest) (*model.CustomCommandResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, id uuid.UUID) error
}

type CustomCommandUseCase struct {
	db          *entity.Database
	commandRepo repository.ICustomCommandRepository
	botRepo     repository.ITelegramBotRepository
	billingRepo repository.IClientBillingRepository
	log         *zap.Logger
}

func NewCustomCommandUseCase(
	db *entity.Database,
	commandRepo repository.ICustomCommandRepository,
	botRepo repository.ITelegramBotRepository,
	billingRepo repository.IClientBillingRepository,
	log *zap.Logger,
) ICustomCommandUseCase {
	return &CustomCommandUseCase{
		db:          db,
		commandRepo: commandRepo,
		botRepo:     botRepo,
		billingRepo: billingRepo,
		log:         log,
	}
}

func (uc *CustomCommandUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.CreateCustomCommandRequest) (*model.CustomCommandResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("custom command usecase create start", zap.String("client_id", clientID.String()))

	// Validasi Bot
	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, req.BotID)
	if err != nil {
		return nil, helper.NewBadRequest("Bot tidak valid")
	}
	if bot.ClientID != clientID {
		return nil, helper.NewBadRequest("Bot tidak valid")
	}

	// 1. Check quota
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("custom command usecase create find billing failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal memeriksa status billing")
	}
	if billing != nil && billing.Plan.MaxCustomCommands != -1 {
		currentCount, err := uc.commandRepo.CountByClientID(ctx, uc.db.Gorm, clientID, nil, nil)
		if err != nil {
			log.Error("custom command usecase create count failed", zap.Error(err))
			return nil, fmt.Errorf("Gagal menghitung jumlah command")
		}
		if currentCount >= int64(billing.Plan.MaxCustomCommands) {
			return nil, helper.NewBadRequest(fmt.Sprintf(
				"Kuota custom command Anda sudah penuh (%d/%d). Silakan upgrade paket platform.",
				currentCount, billing.Plan.MaxCustomCommands,
			))
		}
	}

	trigger := strings.ToLower(req.CommandTrigger)
	if !strings.HasPrefix(trigger, "/") {
		trigger = "/" + trigger
	}

	cmd := &entity.CustomCommand{
		ID:             uuid.New(),
		ClientID:       clientID,
		BotID:          req.BotID,
		CommandTrigger: trigger,
		ResponseType:   req.ResponseType,
		ResponseText:   req.ResponseText,
		FileUrl:        req.FileUrl,
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := uc.commandRepo.Create(ctx, uc.db.Gorm, cmd); err != nil {
		if strings.Contains(err.Error(), "unique_bot_command_trigger") {
			return nil, helper.NewConflict("Command trigger sudah digunakan pada bot ini")
		}
		log.Error("custom command usecase create save failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal menyimpan custom command")
	}

	return converter.CustomCommandToResponse(cmd), nil
}

func (uc *CustomCommandUseCase) FindAllByClient(ctx context.Context, clientID uuid.UUID, req *model.CustomCommandFilterRequest) ([]model.CustomCommandResponse, int64, error) {
	commands, err := uc.commandRepo.FindByClientID(ctx, uc.db.Gorm, clientID, req.BotID, req.IsActive, req.Page, req.Limit)
	if err != nil {
		return nil, 0, err
	}

	total, err := uc.commandRepo.CountByClientID(ctx, uc.db.Gorm, clientID, req.BotID, req.IsActive)
	if err != nil {
		return nil, 0, err
	}

	return converter.CustomCommandListToResponse(commands), total, nil
}

func (uc *CustomCommandUseCase) FindByID(ctx context.Context, clientID uuid.UUID, id uuid.UUID) (*model.CustomCommandResponse, error) {
	cmd, err := uc.commandRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Custom command tidak ditemukan")
		}
		return nil, err
	}

	if cmd.ClientID != clientID {
		return nil, helper.NewNotFound("Custom command tidak ditemukan")
	}

	return converter.CustomCommandToResponse(cmd), nil
}

func (uc *CustomCommandUseCase) Update(ctx context.Context, clientID uuid.UUID, id uuid.UUID, req *model.UpdateCustomCommandRequest) (*model.CustomCommandResponse, error) {
	cmd, err := uc.commandRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Custom command tidak ditemukan")
		}
		return nil, err
	}

	if cmd.ClientID != clientID {
		return nil, helper.NewNotFound("Custom command tidak ditemukan")
	}

	if req.CommandTrigger != nil {
		trigger := strings.ToLower(*req.CommandTrigger)
		if !strings.HasPrefix(trigger, "/") {
			trigger = "/" + trigger
		}
		cmd.CommandTrigger = trigger
	}
	if req.ResponseType != nil {
		cmd.ResponseType = *req.ResponseType
	}
	if req.ResponseText != nil {
		cmd.ResponseText = *req.ResponseText
	}
	if req.FileUrl != nil {
		cmd.FileUrl = req.FileUrl
		// Reset telegram_file_id karena file mungkin berubah
		cmd.TelegramFileID = nil
	}
	if req.IsActive != nil {
		cmd.IsActive = *req.IsActive
	}

	cmd.UpdatedAt = time.Now()

	if err := uc.commandRepo.Update(ctx, uc.db.Gorm, cmd); err != nil {
		if strings.Contains(err.Error(), "unique_bot_command_trigger") {
			return nil, helper.NewConflict("Command trigger sudah digunakan pada bot ini")
		}
		return nil, err
	}

	return converter.CustomCommandToResponse(cmd), nil
}

func (uc *CustomCommandUseCase) Delete(ctx context.Context, clientID uuid.UUID, id uuid.UUID) error {
	cmd, err := uc.commandRepo.FindByID(ctx, uc.db.Gorm, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return helper.NewNotFound("Custom command tidak ditemukan")
		}
		return err
	}

	if cmd.ClientID != clientID {
		return helper.NewNotFound("Custom command tidak ditemukan")
	}

	return uc.commandRepo.Delete(ctx, uc.db.Gorm, cmd)
}
