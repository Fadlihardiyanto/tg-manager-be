package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupBroadcastTest(t *testing.T) (*BroadcastUseCase, *entity.Database, uuid.UUID, uuid.UUID, uuid.UUID, func()) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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
	createTable(`CREATE TABLE broadcasts (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL, bot_id TEXT NOT NULL,
		target_type TEXT NOT NULL, message_type TEXT NOT NULL,
		message_text TEXT NOT NULL, file_url TEXT, telegram_file_id TEXT,
		status TEXT NOT NULL DEFAULT 'pending', total_targets INTEGER NOT NULL DEFAULT 0,
		sent_count INTEGER NOT NULL DEFAULT 0, failed_count INTEGER NOT NULL DEFAULT 0,
		failed_details BLOB, group_filter BLOB,
		scheduled_at DATETIME, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE outbox (
		id TEXT PRIMARY KEY, aggregate_type TEXT NOT NULL, aggregate_id TEXT NOT NULL,
		event_type TEXT NOT NULL, payload BLOB NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending', retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3, last_error TEXT,
		process_after DATETIME, processed_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`)
	createTable(`CREATE TABLE clients (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT '', owner_user_id TEXT,
		subscription_tier TEXT NOT NULL DEFAULT 'free', is_active INTEGER DEFAULT 1,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE platform_plans (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, display_name TEXT NOT NULL,
		price_monthly REAL NOT NULL DEFAULT 0, price_yearly REAL NOT NULL DEFAULT 0,
		max_bots INTEGER NOT NULL DEFAULT 1, max_groups INTEGER NOT NULL DEFAULT 1,
		max_packages INTEGER NOT NULL DEFAULT 3, max_members INTEGER NOT NULL DEFAULT 100,
		max_custom_commands INTEGER NOT NULL DEFAULT 5, max_broadcasts INTEGER NOT NULL DEFAULT 3,
		allow_media_broadcast INTEGER DEFAULT 0, allow_discount_system INTEGER DEFAULT 0,
		allow_reports_export INTEGER DEFAULT 0, allow_high_priority INTEGER DEFAULT 0,
		transaction_limit INTEGER NOT NULL DEFAULT -1,
		features BLOB NOT NULL DEFAULT '[]',
		is_active INTEGER DEFAULT 1, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE client_billings (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL, plan_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active', billing_cycle TEXT NOT NULL DEFAULT 'monthly',
		amount REAL NOT NULL,
		started_at DATETIME, expired_at DATETIME, cancelled_at DATETIME,
		external_id TEXT, payment_url TEXT, paid_at DATETIME,
		is_manual INTEGER DEFAULT 0, note TEXT, created_by TEXT, cancelled_by TEXT, updated_by TEXT,
		discount_id TEXT, discount_amount REAL DEFAULT 0, original_amount REAL,
		created_at DATETIME, updated_at DATETIME
	)`)
	createTable(`CREATE TABLE packages (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL,
		name TEXT NOT NULL, description TEXT, price REAL NOT NULL,
		duration_days INTEGER NOT NULL, is_all_access INTEGER DEFAULT 0,
		is_active INTEGER DEFAULT 1,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE telegram_users (
		id TEXT PRIMARY KEY, telegram_user_id INTEGER NOT NULL,
		username TEXT, first_name TEXT, last_name TEXT, phone TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE subscriptions (
		id TEXT PRIMARY KEY, telegram_user_id TEXT NOT NULL,
		package_id TEXT NOT NULL, client_id TEXT NOT NULL,
		order_id TEXT, status TEXT NOT NULL DEFAULT 'active',
		activated_at DATETIME, expired_at DATETIME,
		auto_renew INTEGER DEFAULT 0, grace_period_hours INTEGER DEFAULT 0,
		kicked_at DATETIME, last_checked_at DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE package_groups (
		package_id TEXT NOT NULL, group_id TEXT NOT NULL,
		PRIMARY KEY (package_id, group_id)
	)`)

	database := &entity.Database{Gorm: db}
	logger := zap.NewNop()

	bcRepo := repository.NewBroadcastRepository()
	botRepo := repository.NewTelegramBotRepository()
	groupRepo := repository.NewTelegramGroupRepository()
	subRepo := repository.NewSubscriptionRepository()
	outboxRepo := repository.NewOutboxRepository()
	billingRepo := repository.NewClientBillingRepository()

	uc := &BroadcastUseCase{
		db:            database,
		broadcastRepo: bcRepo,
		botRepo:       botRepo,
		groupRepo:     groupRepo,
		subRepo:       subRepo,
		outboxRepo:    outboxRepo,
		billingRepo:   billingRepo,
		log:           logger,
	}

	clientID := uuid.New()
	botID := uuid.New()
	planID := uuid.New()
	group1ID := uuid.New()
	group2ID := uuid.New()

	now := time.Now()

	db.Exec(`INSERT INTO clients (id, name, slug, category, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		clientID, "Test Client", "test-client", "tech", now, now)
	db.Exec(`INSERT INTO telegram_bots (id, client_id, token, bot_role, is_active, created_at, updated_at) VALUES (?, ?, ?, 'all_in_one', 1, ?, ?)`,
		botID, clientID, "encrypted-token", now, now)
	db.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		group1ID, clientID, botID, -100111, "Group 1", now, now)
	db.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		group2ID, clientID, botID, -100222, "Group 2", now, now)
	db.Exec(`INSERT INTO platform_plans (id, name, display_name, max_broadcasts, max_bots, max_groups, max_packages, max_members, max_custom_commands, allow_media_broadcast, price_monthly, price_yearly, created_at, updated_at) VALUES (?, 'pro', 'Pro Plan', 3, -1, -1, -1, -1, -1, 1, 249000, 2988000, ?, ?)`,
		planID, now, now)
	db.Exec(`INSERT INTO client_billings (id, client_id, plan_id, status, amount, billing_cycle, started_at, expired_at, created_at, updated_at) VALUES (?, ?, ?, 'active', 249000, 'monthly', ?, ?, ?, ?)`,
		uuid.New(), clientID, planID, now, now.AddDate(0, 1, 0), now, now)

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	return uc, database, clientID, botID, group1ID, cleanup
}

func TestCreateBroadcast_Success(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello World",
		IsImmediate: true,
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result == nil {
		t.Fatal("Create() returned nil result")
	}
	if result.Status != "pending" && result.Status != "processing" {
		t.Errorf("Status = %q, want 'pending' or 'processing'", result.Status)
	}
	if result.TotalTargets != 2 {
		t.Errorf("TotalTargets = %d, want 2 (active groups)", result.TotalTargets)
	}
}

func TestCreateBroadcast_InvalidBot(t *testing.T) {
	uc, _, clientID, _, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       uuid.New(),
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: true,
	}

	_, err := uc.Create(context.Background(), clientID, req)
	if err == nil {
		t.Fatal("expected error for invalid bot")
	}
}

func TestCreateBroadcast_WrongClient(t *testing.T) {
	uc, _, _, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: true,
	}

	_, err := uc.Create(context.Background(), uuid.New(), req)
	if err == nil {
		t.Fatal("expected error for wrong client")
	}
}

func TestCreateBroadcast_ImmediateWithScheduledAt(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	tomorrow := time.Now().Add(24 * time.Hour)
	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: true,
		ScheduledAt: &tomorrow,
	}

	_, err := uc.Create(context.Background(), clientID, req)
	if err == nil {
		t.Fatal("expected error for immediate broadcast with scheduled_at")
	}
}

func TestCreateBroadcast_ScheduledWithoutScheduledAt(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: false,
	}

	_, err := uc.Create(context.Background(), clientID, req)
	if err == nil {
		t.Fatal("expected error for scheduled broadcast without scheduled_at")
	}
}

func TestCreateBroadcast_ScheduledSucceeds(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	tomorrow := time.Now().Add(24 * time.Hour)
	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: false,
		ScheduledAt: &tomorrow,
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Status != "scheduled" {
		t.Errorf("Status = %q, want 'scheduled'", result.Status)
	}
}

func TestCreateBroadcast_ScheduledTooSoon(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	soon := time.Now().Add(30 * time.Second)
	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: false,
		ScheduledAt: &soon,
	}

	_, err := uc.Create(context.Background(), clientID, req)
	if err == nil {
		t.Fatal("expected error for scheduled_at too soon")
	}
}

func TestCreateBroadcast_WithSpecificGroupIDs(t *testing.T) {
	uc, _, clientID, botID, group1ID, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello Group 1 only",
		IsImmediate: true,
		GroupIDs:    []uuid.UUID{group1ID},
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.TotalTargets != 1 {
		t.Errorf("TotalTargets = %d, want 1 (only Group 1)", result.TotalTargets)
	}
}

func TestCreateBroadcast_NoActiveTargets(t *testing.T) {
	uc, db, clientID, _, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	db.Gorm.Exec("UPDATE groups SET is_active = 0")

	botID := uuid.New()
	now := time.Now()
	db.Gorm.Exec(`INSERT INTO telegram_bots (id, client_id, token, bot_role, is_active, created_at, updated_at) VALUES (?, ?, ?, 'all_in_one', 1, ?, ?)`,
		botID, clientID, "encrypted-token", now, now)

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello",
		IsImmediate: true,
	}

	_, err := uc.Create(context.Background(), clientID, req)
	if err == nil {
		t.Fatal("expected error when no active targets")
	}
}

func TestCreateBroadcast_NilGroupIDsTreatsAllActive(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello All",
		IsImmediate: true,
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.TotalTargets != 2 {
		t.Errorf("TotalTargets = %d, want 2 (all active groups)", result.TotalTargets)
	}
}

func TestCreateBroadcast_EmptyGroupIDsTreatsAllActive(t *testing.T) {
	uc, _, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "group",
		MessageType: "text",
		MessageText: "Hello All",
		IsImmediate: true,
		GroupIDs:    []uuid.UUID{},
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.TotalTargets != 2 {
		t.Errorf("TotalTargets = %d, want 2 (all active groups)", result.TotalTargets)
	}
}

func TestCreateBroadcast_WithMemberTarget(t *testing.T) {
	uc, db, clientID, botID, group1ID, cleanup := setupBroadcastTest(t)
	defer cleanup()

	now := time.Now()
	packageID := uuid.New()
	tgUserID := uuid.New()

	db.Gorm.Exec(`INSERT INTO packages (id, client_id, name, price, duration_days, is_active, created_at, updated_at) VALUES (?, ?, ?, 50000, 30, 1, ?, ?)`,
		packageID, clientID, "Basic", now, now)
	db.Gorm.Exec(`INSERT INTO package_groups (package_id, group_id) VALUES (?, ?)`, packageID, group1ID)
	db.Gorm.Exec(`INSERT INTO telegram_users (id, telegram_user_id, first_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		tgUserID, 123456, "Test User", now, now)
	db.Gorm.Exec(`INSERT INTO subscriptions (id, telegram_user_id, package_id, client_id, status, activated_at, expired_at, created_at, updated_at) VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?)`,
		uuid.New(), tgUserID, packageID, clientID, now, now.AddDate(0, 1, 0), now, now)

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "member",
		MessageType: "text",
		MessageText: "Hello Members",
		IsImmediate: true,
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.TotalTargets != 1 {
		t.Errorf("TotalTargets = %d, want 1", result.TotalTargets)
	}
}

func TestCreateBroadcast_GetReach(t *testing.T) {
	uc, db, clientID, botID, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	now := time.Now()
	packageID := uuid.New()
	tgUserID := uuid.New()
	group1ID := uuid.New()
	db.Gorm.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		group1ID, clientID, botID, -100333, "Group 3", now, now)
	db.Gorm.Exec(`INSERT INTO packages (id, client_id, name, price, duration_days, is_active, created_at, updated_at) VALUES (?, ?, ?, 50000, 30, 1, ?, ?)`,
		packageID, clientID, "Basic", now, now)
	db.Gorm.Exec(`INSERT INTO package_groups (package_id, group_id) VALUES (?, ?)`, packageID, group1ID)
	db.Gorm.Exec(`INSERT INTO telegram_users (id, telegram_user_id, first_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		tgUserID, 789012, "Another User", now, now)
	db.Gorm.Exec(`INSERT INTO subscriptions (id, telegram_user_id, package_id, client_id, status, activated_at, expired_at, created_at, updated_at) VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?)`,
		uuid.New(), tgUserID, packageID, clientID, now, now.AddDate(0, 1, 0), now, now)

	result, err := uc.GetReach(context.Background(), clientID, botID)
	if err != nil {
		t.Fatalf("GetReach() error = %v", err)
	}
	if result.GroupCount < 1 {
		t.Errorf("GroupCount = %d, want at least 1", result.GroupCount)
	}
	if result.MemberCount < 1 {
		t.Errorf("MemberCount = %d, want at least 1", result.MemberCount)
	}
}

func TestCreateBroadcast_WithTargetTypeMemberAndSpecificGroupIDs(t *testing.T) {
	uc, db, clientID, botID, group1ID, cleanup := setupBroadcastTest(t)
	defer cleanup()

	now := time.Now()
	pkg1 := uuid.New()
	pkg2 := uuid.New()
	pkg2GroupID := uuid.New()
	tgUser1 := uuid.New()

	db.Gorm.Exec(`INSERT INTO packages (id, client_id, name, price, duration_days, is_active, created_at, updated_at) VALUES (?, ?, ?, 50000, 30, 1, ?, ?)`,
		pkg1, clientID, "Pkg1", now, now)
	db.Gorm.Exec(`INSERT INTO packages (id, client_id, name, price, duration_days, is_active, created_at, updated_at) VALUES (?, ?, ?, 50000, 30, 1, ?, ?)`,
		pkg2, clientID, "Pkg2", now, now)
	db.Gorm.Exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		pkg2GroupID, clientID, botID, -100444, "Group for Pkg2", now, now)
	db.Gorm.Exec(`INSERT INTO package_groups (package_id, group_id) VALUES (?, ?)`, pkg1, group1ID)
	db.Gorm.Exec(`INSERT INTO package_groups (package_id, group_id) VALUES (?, ?)`, pkg2, pkg2GroupID)
	db.Gorm.Exec(`INSERT INTO telegram_users (id, telegram_user_id, first_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		tgUser1, 111222, "User 1", now, now)
	db.Gorm.Exec(`INSERT INTO subscriptions (id, telegram_user_id, package_id, client_id, status, activated_at, expired_at, created_at, updated_at) VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?)`,
		uuid.New(), tgUser1, pkg2, clientID, now, now.AddDate(0, 1, 0), now, now)

	req := &model.CreateBroadcastRequest{
		BotID:       botID,
		TargetType:  "member",
		MessageType: "text",
		MessageText: "Only Group 1 Members",
		IsImmediate: true,
		GroupIDs:    []uuid.UUID{group1ID},
	}

	result, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		if err.Error() == "tidak ditemukan target penerima aktif untuk broadcast ini" {
			t.Log("correctly found 0 members for Group 1 only (subscription is on Pkg2→other group)")
			return
		}
		t.Fatalf("Create() error = %v", err)
	}
	t.Errorf("TotalTargets = %d, expected 0 (user subscribed via Pkg2 → different group)", result.TotalTargets)
}

func TestGetReach_InvalidBot(t *testing.T) {
	uc, _, _, _, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	_, err := uc.GetReach(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error for invalid bot")
	}
}

func TestGetReach_ErrorText(t *testing.T) {
	_, _, _, _, _, cleanup := setupBroadcastTest(t)
	defer cleanup()

	badReqErr := helper.NewBadRequest("test")
	if badReqErr == nil || badReqErr.Error() == "" {
		t.Error("NewBadRequest() should return a non-empty error")
	}
}
