package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupGroupInactiveTest(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	createTable := func(ddl string) {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table error: %v\nDDL: %s", err, ddl)
		}
	}

	createTable(`CREATE TABLE telegram_bots (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL,
		token TEXT NOT NULL, username TEXT, bot_id INTEGER,
		bot_role TEXT NOT NULL DEFAULT 'all_in_one', is_active INTEGER DEFAULT 1,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE groups (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL, bot_id TEXT NOT NULL,
		telegram_chat_id INTEGER NOT NULL, name TEXT, description TEXT,
		is_active INTEGER DEFAULT 1, inactive_reason TEXT, member_count INTEGER DEFAULT 0,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE report_settings (
		client_id TEXT PRIMARY KEY, enabled INTEGER DEFAULT 0,
		target_chat_id INTEGER NOT NULL, bot_id TEXT NOT NULL,
		report_time TEXT NOT NULL DEFAULT '08:00', last_sent_at DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	clientID := uuid.New()
	botID := uuid.New()

	if err := db.Create(&entity.TelegramBot{ID: botID, ClientID: clientID, Token: "raw-token"}).Error; err != nil {
		t.Fatalf("seed bot error = %v", err)
	}

	return db, clientID, botID
}

// stubBotFactory: NewClient selalu gagal supaya test tidak memanggil API Telegram.
type stubBotFactory struct{}

func (f *stubBotFactory) NewClient(token string) (telegram.BotClient, error) {
	return nil, errors.New("stub: no telegram network in test")
}

func newBotDeleteUC(db *gorm.DB, botRepo repository.ITelegramBotRepository) ITelegramBotUseCase {
	return NewTelegramBotUseCase(
		&entity.Database{Gorm: db},
		botRepo,
		nil,
		&stubBotFactory{},
		zap.NewNop(),
		"test-key",
		"",
		"",
	)
}

func TestFindByClientID_ExcludesInactiveGroups(t *testing.T) {
	db, clientID, botID := setupGroupInactiveTest(t)

	activeID := uuid.New()
	inactiveID := uuid.New()
	otherClientID := uuid.New()

	seedGroup := func(id uuid.UUID, clientID uuid.UUID, chatID int64, name string, isActive bool) {
		active := 0
		if isActive {
			active = 1
		}
		if err := db.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			id.String(), clientID.String(), botID.String(), chatID, name, active).Error; err != nil {
			t.Fatalf("seed group %s error = %v", name, err)
		}
	}
	seedGroup(activeID, clientID, 111, "Aktif", true)
	seedGroup(inactiveID, clientID, 222, "Nonaktif", false)
	seedGroup(uuid.New(), otherClientID, 333, "Client lain", true)

	groupRepo := repository.NewTelegramGroupRepository()
	groups, err := groupRepo.FindByClientID(context.Background(), db, clientID, 1, 10)
	if err != nil {
		t.Fatalf("FindByClientID() error = %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("FindByClientID() len = %d, want 1 (grup nonaktif dan client lain tidak boleh muncul)", len(groups))
	}
	if groups[0].ID != activeID {
		t.Fatalf("FindByClientID() returned group %s, want active group %s", groups[0].ID, activeID)
	}
}

func TestBotDelete_AllowsWhenAllGroupsDisconnected(t *testing.T) {
	db, clientID, botID := setupGroupInactiveTest(t)

	// Grup sudah disconnect (is_active=false) — bot_id masih menunjuk ke bot,
	// tapi tidak boleh memblokir penghapusan bot.
	if err := db.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		uuid.New().String(), clientID.String(), botID.String(), 111, "Sudah disconnect").Error; err != nil {
		t.Fatalf("seed inactive group error = %v", err)
	}

	botUC := newBotDeleteUC(db, repository.NewTelegramBotRepository())
	if err := botUC.Delete(context.Background(), clientID, botID); err != nil {
		t.Fatalf("Delete() error = %v, want sukses walau ada grup nonaktif", err)
	}

	var remaining int64
	if err := db.Model(&entity.TelegramBot{}).Where("id = ? AND deleted_at IS NULL", botID).Count(&remaining).Error; err != nil {
		t.Fatalf("count bot error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("bot masih ada setelah delete (%d), want 0", remaining)
	}
}

func TestBotDelete_RejectedWhenActiveGroupUsesBot(t *testing.T) {
	db, clientID, botID := setupGroupInactiveTest(t)

	if err := db.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		uuid.New().String(), clientID.String(), botID.String(), 111, "Masih aktif").Error; err != nil {
		t.Fatalf("seed active group error = %v", err)
	}

	botUC := newBotDeleteUC(db, repository.NewTelegramBotRepository())
	err := botUC.Delete(context.Background(), clientID, botID)
	if err == nil {
		t.Fatal("Delete() nil error, want ditolak karena grup aktif masih memakai bot")
	}
	var badReq *helper.ErrBadRequest
	if !errors.As(err, &badReq) {
		t.Fatalf("Delete() error = %v, want ErrBadRequest", err)
	}

	var remaining int64
	if err := db.Model(&entity.TelegramBot{}).Where("id = ? AND deleted_at IS NULL", botID).Count(&remaining).Error; err != nil {
		t.Fatalf("count bot error = %v", err)
	}
	if remaining != 1 {
		t.Fatalf("bot terhapus padahal seharusnya ditolak (%d tersisa, want 1)", remaining)
	}
}

func TestBotDelete_RejectedWhenReportSettingsUseBot(t *testing.T) {
	db, clientID, botID := setupGroupInactiveTest(t)

	if err := db.Exec(`INSERT INTO report_settings (client_id, enabled, target_chat_id, bot_id) VALUES (?, 1, 999, ?)`, clientID.String(), botID.String()).Error; err != nil {
		t.Fatalf("seed report_settings error = %v", err)
	}

	botUC := newBotDeleteUC(db, repository.NewTelegramBotRepository())
	err := botUC.Delete(context.Background(), clientID, botID)
	if err == nil {
		t.Fatal("Delete() nil error, want ditolak karena bot masih terhubung ke laporan")
	}
	var badReq *helper.ErrBadRequest
	if !errors.As(err, &badReq) {
		t.Fatalf("Delete() error = %v, want ErrBadRequest", err)
	}

	var remaining int64
	if err := db.Model(&entity.TelegramBot{}).Where("id = ? AND deleted_at IS NULL", botID).Count(&remaining).Error; err != nil {
		t.Fatalf("count bot error = %v", err)
	}
	if remaining != 1 {
		t.Fatalf("bot terhapus padahal seharusnya ditolak (%d tersisa, want 1)", remaining)
	}
}