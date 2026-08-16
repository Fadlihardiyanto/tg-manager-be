package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	pkg_s3 "github.com/Fadlihardiyanto/telegram-management-app/pkg/s3"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ICustomCommandUseCase interface {
	Create(ctx context.Context, clientID uuid.UUID, req *model.CreateCustomCommandRequest) (*model.CustomCommandResponse, error)
	FindAllByClient(ctx context.Context, clientID uuid.UUID, req *model.CustomCommandFilterRequest) ([]model.CustomCommandResponse, int64, error)
	FindByID(ctx context.Context, clientID uuid.UUID, id uuid.UUID) (*model.CustomCommandResponse, error)
	Update(ctx context.Context, clientID uuid.UUID, id uuid.UUID, req *model.UpdateCustomCommandRequest) (*model.CustomCommandResponse, error)
	Delete(ctx context.Context, clientID uuid.UUID, id uuid.UUID) error
	BulkDelete(ctx context.Context, clientID uuid.UUID, ids []uuid.UUID) model.BulkDeleteResult
}

type CustomCommandUseCase struct {
	db          *entity.Database
	commandRepo repository.ICustomCommandRepository
	botRepo     repository.ITelegramBotRepository
	billingRepo repository.IClientBillingRepository
	redisClient *redis.Client
	s3Client    *pkg_s3.Client
	log         *zap.Logger
}

func NewCustomCommandUseCase(
	db *entity.Database,
	commandRepo repository.ICustomCommandRepository,
	botRepo repository.ITelegramBotRepository,
	billingRepo repository.IClientBillingRepository,
	redisClient *redis.Client,
	s3Client *pkg_s3.Client,
	log *zap.Logger,
) ICustomCommandUseCase {
	return &CustomCommandUseCase{
		db:          db,
		commandRepo: commandRepo,
		botRepo:     botRepo,
		billingRepo: billingRepo,
		redisClient: redisClient,
		s3Client:    s3Client,
		log:         log,
	}
}

func (uc *CustomCommandUseCase) Create(ctx context.Context, clientID uuid.UUID, req *model.CreateCustomCommandRequest) (*model.CustomCommandResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("custom command usecase create start", zap.String("client_id", clientID.String()))

	// Validasi Bot (botRepo return (nil, nil) saat not-found — guard bot == nil)
	bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, req.BotID)
	if err != nil || bot == nil {
		return nil, helper.NewBadRequest("Bot tidak valid")
	}
	if bot.ClientID != clientID {
		return nil, helper.NewBadRequest("Bot tidak valid")
	}

	// 1. Check quota
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("custom command usecase create find billing failed", zap.Error(err))
		return nil, fmt.Errorf("gagal memeriksa status billing")
	}
	if billing != nil && billing.Plan.MaxCustomCommands != -1 {
		currentCount, err := uc.commandRepo.CountByClientID(ctx, uc.db.Gorm, clientID, nil, nil)
		if err != nil {
			log.Error("custom command usecase create count failed", zap.Error(err))
			return nil, fmt.Errorf("gagal menghitung jumlah command")
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
	if isBlacklistedCommand(trigger) {
		return nil, helper.NewBadRequest(fmt.Sprintf("Perintah '%s' adalah perintah bawaan sistem dan tidak dapat digunakan", trigger))
	}

	// Validasi panjang karakter response_text
	if err := validateResponseTextLength(req.ResponseType, req.ResponseText); err != nil {
		return nil, helper.NewBadRequestWrap(err)
	}

	cmd := &entity.CustomCommand{
		ID:             uuid.New(),
		ClientID:       clientID,
		BotUUID:        req.BotID,
		CommandTrigger: trigger,
		ResponseType:   req.ResponseType,
		ResponseText:   req.ResponseText,
		FileUrl:        req.FileUrl,
		IsActive:       true,
		AccessScope:    req.AccessScope,
		ChatTypeScope:  req.ChatTypeScope,
		PackageIDs:     uuidsToStringArray(req.PackageIDs),
		GroupIDs:       uuidsToStringArray(req.GroupIDs),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if cmd.AccessScope == "" {
		cmd.AccessScope = "public"
	}
	if cmd.ChatTypeScope == "" {
		cmd.ChatTypeScope = "all"
	}

	if err := uc.commandRepo.Create(ctx, uc.db.Gorm, cmd); err != nil {
		if strings.Contains(err.Error(), "unique_bot_command_trigger") {
			return nil, helper.NewConflict("Command trigger sudah digunakan pada bot ini")
		}
		log.Error("custom command usecase create save failed", zap.Error(err))
		return nil, fmt.Errorf("gagal menyimpan custom command")
	}

	uc.invalidateCommand(ctx, clientID, cmd.BotUUID, trigger)

	return converter.CustomCommandToResponse(cmd), nil
}

func (uc *CustomCommandUseCase) FindAllByClient(ctx context.Context, clientID uuid.UUID, req *model.CustomCommandFilterRequest) ([]model.CustomCommandResponse, int64, error) {
	if req == nil {
		return nil, 0, helper.NewBadRequest("filter tidak valid")
	}
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
	log := logger.FromContext(ctx, uc.log)
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

	oldTrigger := cmd.CommandTrigger

	if req.CommandTrigger != nil {
		trigger := strings.ToLower(*req.CommandTrigger)
		if !strings.HasPrefix(trigger, "/") {
			trigger = "/" + trigger
		}
		if isBlacklistedCommand(trigger) {
			return nil, helper.NewBadRequest(fmt.Sprintf("Perintah '%s' adalah perintah bawaan sistem dan tidak dapat digunakan", trigger))
		}
		cmd.CommandTrigger = trigger
	}
	// Validasi panjang karakter response_text
	nextType := cmd.ResponseType
	if req.ResponseType != nil {
		nextType = *req.ResponseType
	}
	nextText := cmd.ResponseText
	if req.ResponseText != nil {
		nextText = *req.ResponseText
	}
	if err := validateResponseTextLength(nextType, nextText); err != nil {
		return nil, helper.NewBadRequestWrap(err)
	}

	if req.ResponseType != nil {
		cmd.ResponseType = *req.ResponseType
	}
	if req.ResponseText != nil {
		cmd.ResponseText = *req.ResponseText
	}
	if req.FileUrl != nil {
		// Jika ada gambar lama dan gambar baru berbeda, hapus gambar lama dari S3
		if cmd.FileUrl != nil && *cmd.FileUrl != "" && *req.FileUrl != *cmd.FileUrl {
			oldKey := extractS3Key(*cmd.FileUrl)
			if oldKey != "" && uc.s3Client != nil {
				// Abaikan error agar proses update tetap berjalan meskipun hapus file lama gagal
				if delErr := uc.s3Client.Delete(ctx, oldKey); delErr != nil {
					log.Warn("custom command: failed to delete old file from S3", zap.String("key", oldKey), zap.Error(delErr))
				}
			}
		}
		// ponytail: salin nilai, jangan simpan pointer milik caller ke entity
		fileUrl := *req.FileUrl
		cmd.FileUrl = &fileUrl
		// Reset telegram_file_id karena file mungkin berubah
		cmd.TelegramFileID = nil
	}
	if req.IsActive != nil {
		cmd.IsActive = *req.IsActive
	}
	if req.AccessScope != nil {
		cmd.AccessScope = *req.AccessScope
	}
	if req.ChatTypeScope != nil {
		cmd.ChatTypeScope = *req.ChatTypeScope
	}
	if req.PackageIDs != nil {
		cmd.PackageIDs = uuidsToStringArray(req.PackageIDs)
	}
	if req.GroupIDs != nil {
		cmd.GroupIDs = uuidsToStringArray(req.GroupIDs)
	}

	cmd.UpdatedAt = time.Now()

	if err := uc.commandRepo.Update(ctx, uc.db.Gorm, cmd); err != nil {
		if strings.Contains(err.Error(), "unique_bot_command_trigger") {
			return nil, helper.NewConflict("Command trigger sudah digunakan pada bot ini")
		}
		return nil, err
	}

	uc.invalidateCommand(ctx, clientID, cmd.BotUUID, oldTrigger)
	uc.invalidateCommand(ctx, clientID, cmd.BotUUID, cmd.CommandTrigger)

	return converter.CustomCommandToResponse(cmd), nil
}

func (uc *CustomCommandUseCase) Delete(ctx context.Context, clientID uuid.UUID, id uuid.UUID) error {
	log := logger.FromContext(ctx, uc.log)
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

	uc.invalidateCommand(ctx, cmd.ClientID, cmd.BotUUID, cmd.CommandTrigger)

	// Hapus file dari S3 jika ada
	if cmd.FileUrl != nil && *cmd.FileUrl != "" && uc.s3Client != nil {
		key := extractS3Key(*cmd.FileUrl)
		if key != "" {
			if errS3 := uc.s3Client.Delete(ctx, key); errS3 != nil {
				log.Warn("failed to delete S3 file during custom command deletion", zap.String("key", key), zap.Error(errS3))
			}
		}
	}

	return uc.commandRepo.Delete(ctx, uc.db.Gorm, cmd)
}

func (uc *CustomCommandUseCase) BulkDelete(ctx context.Context, clientID uuid.UUID, ids []uuid.UUID) model.BulkDeleteResult {
	return RunBulkDelete(ids, func(id uuid.UUID) error {
		return uc.Delete(ctx, clientID, id)
	})
}

func (uc *CustomCommandUseCase) invalidateCommand(ctx context.Context, clientID, botID uuid.UUID, trigger string) {
	log := logger.FromContext(ctx, uc.log)
	if uc.redisClient == nil {
		return
	}
	key := customCmdCacheKey(clientID, botID, trigger)
	if err := uc.redisClient.Del(ctx, key).Err(); err != nil {
		log.Warn("failed to invalidate custom command cache", zap.String("key", key), zap.Error(err))
	}
}

func isBlacklistedCommand(trigger string) bool {
	blacklist := []string{"/start", "/packages", "/mysub", "/status", "/myorders", "/connect"}
	for _, cmd := range blacklist {
		if trigger == cmd {
			return true
		}
	}
	return false
}

func extractS3Key(fileURL string) string {
	if fileURL == "" {
		return ""
	}
	// Find "tenant_uploads/"
	idx := strings.Index(fileURL, "tenant_uploads/")
	if idx == -1 {
		return ""
	}
	return fileURL[idx:]
}

func validateResponseTextLength(responseType string, responseText string) error {
	charCount := utf8.RuneCountInString(responseText)
	if responseType == "text" {
		if charCount > 4096 {
			return fmt.Errorf("panjang isi pesan balasan teks maksimal 4096 karakter (saat ini %d karakter)", charCount)
		}
	} else if responseType == "photo" || responseType == "document" {
		if charCount > 1024 {
			return fmt.Errorf("panjang keterangan (caption) untuk tipe %s maksimal 1024 karakter (saat ini %d karakter)", responseType, charCount)
		}
	}
	return nil
}

func uuidsToStringArray(ids []uuid.UUID) pq.StringArray {
	result := make(pq.StringArray, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}
