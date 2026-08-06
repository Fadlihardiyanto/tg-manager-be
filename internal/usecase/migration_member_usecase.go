package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
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

const templateCSVContent = "username,expired_at\ncontoh_username,2026-12-31\n"

type IMigrationMemberUseCase interface {
	GenerateTemplate(ctx context.Context) (string, error)
	ImportMembers(ctx context.Context, clientID uuid.UUID, req *model.MigrationMemberImportRequest) (*model.MigrationMemberImportResponse, error)
	List(ctx context.Context, clientID uuid.UUID, filter model.MigrationMemberFilterRequest) ([]model.MigrationMemberResponse, int64, error)
	ExportCSV(ctx context.Context, clientID uuid.UUID) (string, error)
}

type MigrationMemberUseCase struct {
	db            *entity.Database
	migrationRepo repository.IMigrationMemberRepository
	packageRepo   repository.IPackageRepository
	featureGateUC IFeatureGateUseCase
	log           *zap.Logger
}

func NewMigrationMemberUseCase(
	db *entity.Database,
	migrationRepo repository.IMigrationMemberRepository,
	packageRepo repository.IPackageRepository,
	featureGateUC IFeatureGateUseCase,
	log *zap.Logger,
) IMigrationMemberUseCase {
	return &MigrationMemberUseCase{
		db:            db,
		migrationRepo: migrationRepo,
		packageRepo:   packageRepo,
		featureGateUC: featureGateUC,
		log:           log,
	}
}

func (uc *MigrationMemberUseCase) GenerateTemplate(ctx context.Context) (string, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("migration member generate template")

	return templateCSVContent, nil
}

func (uc *MigrationMemberUseCase) ImportMembers(ctx context.Context, clientID uuid.UUID, req *model.MigrationMemberImportRequest) (*model.MigrationMemberImportResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("migration member import start", zap.Int("count", len(req.Members)))

	packageID, err := uuid.Parse(req.PackageID)
	if err != nil {
		return nil, helper.NewBadRequest("ID paket tidak valid")
	}

	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, packageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, helper.NewNotFound("Paket tidak ditemukan")
		}
		return nil, fmt.Errorf("gagal mencari paket: %w", err)
	}
	if pkg.ClientID != clientID {
		return nil, helper.NewNotFound("Paket tidak ditemukan")
	}

	// sanitasi username dan collect unique usernames
	sanitized := make([]model.MigrationMemberCSVRow, 0, len(req.Members))
	usernameSet := make(map[string]bool)
	var validationErrors []model.MigrationMemberImportRowError

	for i, row := range req.Members {
		clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(row.Username), "@"))
		if clean == "" {
			validationErrors = append(validationErrors, model.MigrationMemberImportRowError{
				Row: i + 1, Username: row.Username, Error: "Username tidak boleh kosong",
			})
			continue
		}

		expiredAt, err := parseDate(row.ExpiredAt)
		if err != nil {
			validationErrors = append(validationErrors, model.MigrationMemberImportRowError{
				Row: i + 1, Username: clean, Error: fmt.Sprintf("Format tanggal tidak valid: %s", row.ExpiredAt),
			})
			continue
		}

		if expiredAt.Before(time.Now()) {
			validationErrors = append(validationErrors, model.MigrationMemberImportRowError{
				Row: i + 1, Username: clean, Error: "Tanggal kadaluarsa sudah lewat",
			})
			continue
		}

		if usernameSet[clean] {
			validationErrors = append(validationErrors, model.MigrationMemberImportRowError{
				Row: i + 1, Username: clean, Error: "Username duplikat dalam file",
			})
			continue
		}

		usernameSet[clean] = true
		sanitized = append(sanitized, model.MigrationMemberCSVRow{Username: clean, ExpiredAt: row.ExpiredAt})
	}

	// collect sanitized usernames
	usernames := make([]string, len(sanitized))
	for i, row := range sanitized {
		usernames[i] = row.Username
	}

	// check existing pending members
	// security: check-then-insert has a race window — two concurrent requests
	// may both find no duplicate and attempt insert. The DB unique constraint
	// uq_migration_members_pending prevents actual data corruption on collision.
	// Transaction wrapping not justified for this non-critical path.
	existingSet, err := uc.migrationRepo.FindExistingUsernames(ctx, uc.db.Gorm, clientID, packageID, usernames)
	if err != nil {
		log.Error("migration member import check existing failed", zap.Error(err))
		return nil, fmt.Errorf("Gagal memeriksa data yang ada")
	}

	// build entities, skip duplicates
	now := time.Now()
	toInsert := make([]entity.MigrationMember, 0, len(sanitized))
	skipped := 0

	for i, row := range sanitized {
		if existingSet[row.Username] {
			skipped++
			validationErrors = append(validationErrors, model.MigrationMemberImportRowError{
				Row: i + 1, Username: row.Username, Error: "Username sudah terdaftar dan masih pending",
			})
			continue
		}

		expiredAt, err := parseDate(row.ExpiredAt)
		if err != nil {
			log.Warn("migration member import: invalid expired_at, skipping row",
				zap.String("username", row.Username),
				zap.String("expired_at", row.ExpiredAt),
				zap.Error(err))
			skipped++
			validationErrors = append(validationErrors, model.MigrationMemberImportRowError{
				Row: i + 1, Username: row.Username, Error: "Format tanggal expired_at tidak valid: " + row.ExpiredAt,
			})
			continue
		}
		toInsert = append(toInsert, entity.MigrationMember{
			ID:        uuid.New(),
			ClientID:  clientID,
			PackageID: packageID,
			Username:  row.Username,
			ExpiredAt: expiredAt,
			Status:    "pending",
			CreatedAt: now,
		})
	}

	imported := 0
	if len(toInsert) > 0 {
		if uc.featureGateUC != nil {
			_, current, limit, err := uc.featureGateUC.CheckQuota(ctx, clientID, "members")
			if err != nil {
				log.Warn("migration member import quota check failed", zap.Error(err))
			} else if limit != -1 && current+int64(len(toInsert)) > int64(limit) {
				return nil, helper.NewBadRequest(fmt.Sprintf(
					"Jumlah member yang diimpor melebihi batas maksimum paket Anda (Sisa kuota: %d member). Silakan upgrade paket.",
					limit-int(current),
				))
			}
		}

		if err := uc.migrationRepo.BulkInsert(ctx, uc.db.Gorm, toInsert); err != nil {
			log.Error("migration member import bulk insert failed", zap.Error(err))
			return nil, fmt.Errorf("Gagal menyimpan data migrasi")
		}
		imported = len(toInsert)
	}

	log.Info("migration member import complete",
		zap.Int("total", len(req.Members)),
		zap.Int("imported", imported),
		zap.Int("skipped", skipped),
		zap.Int("errors", len(validationErrors)),
	)

	return &model.MigrationMemberImportResponse{
		TotalSubmitted: len(req.Members),
		Imported:       imported,
		Skipped:        skipped,
		Errors:         validationErrors,
	}, nil
}

func (uc *MigrationMemberUseCase) List(ctx context.Context, clientID uuid.UUID, filter model.MigrationMemberFilterRequest) ([]model.MigrationMemberResponse, int64, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("migration member list start")

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}

	members, err := uc.migrationRepo.FindByClientID(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("migration member list find failed", zap.Error(err))
		return nil, 0, fmt.Errorf("Gagal mengambil data migrasi")
	}

	total, err := uc.migrationRepo.CountByClientID(ctx, uc.db.Gorm, clientID, filter)
	if err != nil {
		log.Error("migration member list count failed", zap.Error(err))
		return nil, 0, fmt.Errorf("Gagal menghitung data migrasi")
	}

	return converter.MigrationMembersToResponse(members), total, nil
}

func (uc *MigrationMemberUseCase) ExportCSV(ctx context.Context, clientID uuid.UUID) (string, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("migration member export start")

	members, err := uc.migrationRepo.FindAllByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		log.Error("migration member export find failed", zap.Error(err))
		return "", fmt.Errorf("Gagal mengambil data migrasi")
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Write([]string{"username", "package_id", "expired_at", "status", "claimed_at", "created_at"})

	for _, m := range members {
		claimedAt := ""
		if m.ClaimedAt != nil {
			claimedAt = m.ClaimedAt.Format("2006-01-02 15:04:05")
		}
		writer.Write([]string{
			escapeCSVField(m.Username),
			m.PackageID.String(),
			m.ExpiredAt.Format("2006-01-02 15:04:05"),
			m.Status,
			claimedAt,
			m.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Error("migration member export csv write failed", zap.Error(err))
		return "", fmt.Errorf("Gagal membuat file CSV")
	}

	return buf.String(), nil
}

// parseDate mencoba beberapa format tanggal umum.
func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"02-01-2006",
		"02/01/2006",
		time.RFC3339,
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}

// escapeCSVField prefixes fields starting with =, +, -, @ with a tab
// to prevent CSV injection when the file is opened in Excel.
// security: mitigates CWE-1236 formula injection in CSV export
func escapeCSVField(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	}
	return s
}
