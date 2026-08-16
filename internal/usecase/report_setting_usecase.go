package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/google/uuid"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IReportSettingUseCase interface {
	Get(ctx context.Context, clientID uuid.UUID) (*entity.ReportSetting, error)
	Upsert(ctx context.Context, clientID uuid.UUID, req *model.ReportSettingRequest) (*entity.ReportSetting, error)
	ListFailures(ctx context.Context, clientID uuid.UUID, date string) ([]model.ReportFailureItem, error)
}

type ReportSettingUseCase struct {
	db       *gorm.DB
	repo     repository.IReportSettingRepository
	botRepo  repository.ITelegramBotRepository
	validate func(ctx context.Context, req any) error
	log      *zap.Logger
}

func NewReportSettingUseCase(db *gorm.DB, repo repository.IReportSettingRepository, botRepo repository.ITelegramBotRepository, log *zap.Logger) IReportSettingUseCase {
	return &ReportSettingUseCase{
		db:      db,
		repo:    repo,
		botRepo: botRepo,
		log:     log,
	}
}

func (uc *ReportSettingUseCase) Get(ctx context.Context, clientID uuid.UUID) (*entity.ReportSetting, error) {
	setting, err := uc.repo.FindByClientID(ctx, uc.db, clientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Belum pernah di-set — dashboard render state default
			return &entity.ReportSetting{ClientID: clientID, ReportTime: "08:00"}, nil
		}
		return nil, err
	}
	return setting, nil
}

func (uc *ReportSettingUseCase) Upsert(ctx context.Context, clientID uuid.UUID, req *model.ReportSettingRequest) (*entity.ReportSetting, error) {
	// Validasi format HH:MM
	if _, err := time.Parse("15:04", req.ReportTime); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "report_time harus format HH:MM (contoh: 08:00)")
	}

	// Bot pengirim wajib milik tenant ini
	if !uc.botExists(ctx, clientID, req.BotID) {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bot pengirim tidak ditemukan di tenant ini")
	}

	setting := &entity.ReportSetting{
		ClientID:     clientID,
		Enabled:      req.Enabled,
		TargetChatID: req.TargetChatID,
		BotID:        req.BotID,
		ReportTime:   req.ReportTime,
		UpdatedAt:    time.Now(),
	}
	if err := uc.repo.Upsert(ctx, uc.db, setting); err != nil {
		return nil, err
	}
	return setting, nil
}

func (uc *ReportSettingUseCase) botExists(ctx context.Context, clientID uuid.UUID, botID uuid.UUID) bool {
	var count int64
	err := uc.db.WithContext(ctx).
		Table("telegram_bots").
		Where("id = ? AND client_id = ? AND deleted_at IS NULL", botID, clientID).
		Count(&count).Error
	return err == nil && count > 0
}

func (uc *ReportSettingUseCase) ListFailures(ctx context.Context, clientID uuid.UUID, date string) ([]model.ReportFailureItem, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	if date != "" {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest, "date harus format YYYY-MM-DD")
		}
		start = parsed.UTC()
	}
	end := start.Add(24 * time.Hour)

	rows, err := uc.db.WithContext(ctx).Raw(
		`SELECT event_type, telegram_chat_id, detail, created_at
		 FROM job_events
		 WHERE client_id = ? AND status = 'failed' AND created_at >= ? AND created_at < ?
		 ORDER BY created_at DESC
		 LIMIT 500`,
		clientID, start, end,
	).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ReportFailureItem
	for rows.Next() {
		var item model.ReportFailureItem
		var chatID *int64
		var createdAt time.Time
		if err := rows.Scan(&item.EventType, &chatID, &item.Detail, &createdAt); err != nil {
			continue
		}
		item.TelegramChatID = chatID
		item.Detail = fmt.Sprintf("%.200s", item.Detail)
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}
