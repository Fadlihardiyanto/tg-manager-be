package repository

import (
	"context"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IReportSettingRepository interface {
	FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (*entity.ReportSetting, error)
	Upsert(ctx context.Context, tx *gorm.DB, setting *entity.ReportSetting) error
	FindEnabled(ctx context.Context, tx *gorm.DB) ([]entity.ReportSetting, error)
	UpdateLastSentAt(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, sentAt time.Time) error
}

type ReportSettingRepository struct {
	Repository[entity.ReportSetting]
}

func NewReportSettingRepository() IReportSettingRepository {
	return &ReportSettingRepository{}
}

func (r *ReportSettingRepository) FindByClientID(ctx context.Context, tx *gorm.DB, clientID uuid.UUID) (*entity.ReportSetting, error) {
	var setting entity.ReportSetting
	err := tx.WithContext(ctx).Where("client_id = ?", clientID).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (r *ReportSettingRepository) Upsert(ctx context.Context, tx *gorm.DB, setting *entity.ReportSetting) error {
	return tx.WithContext(ctx).Save(setting).Error
}

func (r *ReportSettingRepository) FindEnabled(ctx context.Context, tx *gorm.DB) ([]entity.ReportSetting, error) {
	var settings []entity.ReportSetting
	err := tx.WithContext(ctx).Where("enabled = ?", true).Find(&settings).Error
	return settings, err
}

func (r *ReportSettingRepository) UpdateLastSentAt(ctx context.Context, tx *gorm.DB, clientID uuid.UUID, sentAt time.Time) error {
	return tx.WithContext(ctx).Model(&entity.ReportSetting{}).
		Where("client_id = ?", clientID).
		Update("last_sent_at", sentAt).Error
}
