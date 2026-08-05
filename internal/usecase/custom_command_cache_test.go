package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCustomCommandCacheTest(t *testing.T) (*CustomCommandUseCase, *redis.Client, uuid.UUID, uuid.UUID, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

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
	createTable(`CREATE TABLE custom_commands (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL, bot_id TEXT NOT NULL,
		command_trigger TEXT NOT NULL, response_type TEXT NOT NULL DEFAULT 'text',
		response_text TEXT NOT NULL, file_url TEXT, telegram_file_id TEXT,
		is_active INTEGER NOT NULL DEFAULT 1,
		access_scope TEXT NOT NULL DEFAULT 'public', chat_type_scope TEXT NOT NULL DEFAULT 'all',
		package_ids TEXT, group_ids TEXT,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
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

	clientID := uuid.New()
	botID := uuid.New()
	planID := uuid.New()
	now := time.Now()

	db.Exec(`INSERT INTO clients (id, name, slug, category, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		clientID, "Test Client", "test-client", "tech", now, now)
	db.Exec(`INSERT INTO telegram_bots (id, client_id, token, bot_role, is_active, created_at, updated_at) VALUES (?, ?, ?, 'all_in_one', 1, ?, ?)`,
		botID, clientID, "encrypted-token", now, now)
	db.Exec(`INSERT INTO platform_plans (id, name, display_name, max_custom_commands, max_bots, max_groups, max_packages, max_members, max_broadcasts, price_monthly, price_yearly, created_at, updated_at) VALUES (?, 'pro', 'Pro Plan', -1, -1, -1, -1, -1, -1, 249000, 2988000, ?, ?)`,
		planID, now, now)
	db.Exec(`INSERT INTO client_billings (id, client_id, plan_id, status, amount, billing_cycle, started_at, expired_at, created_at, updated_at) VALUES (?, ?, ?, 'active', 249000, 'monthly', ?, ?, ?, ?)`,
		uuid.New(), clientID, planID, now, now.AddDate(0, 1, 0), now, now)

	database := &entity.Database{Gorm: db}
	logger := zap.NewNop()

	uc := NewCustomCommandUseCase(
		database,
		repository.NewCustomCommandRepository(),
		repository.NewTelegramBotRepository(),
		repository.NewClientBillingRepository(),
		redisClient,
		nil,
		logger,
	).(*CustomCommandUseCase)

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
		redisClient.Close()
		mr.Close()
	}

	return uc, redisClient, clientID, botID, cleanup
}

func cacheKey(clientID, botID uuid.UUID, trigger string) string {
	return "custom_cmd:" + clientID.String() + ":" + botID.String() + ":" + trigger
}

func TestGetCustomCommand_WriteThrough(t *testing.T) {
	uc, redisClient, clientID, botID, cleanup := setupCustomCommandCacheTest(t)
	defer cleanup()

	req := &model.CreateCustomCommandRequest{
		BotID:          botID,
		CommandTrigger: "rules",
		ResponseType:   "text",
		ResponseText:   "Ini aturan grup.",
	}
	resp, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.CommandTrigger != "/rules" {
		t.Errorf("CommandTrigger = %q, want %q", resp.CommandTrigger, "/rules")
	}

	webhookUC := &TelegramWebhookUseCase{
		db:          uc.db,
		commandRepo: repository.NewCustomCommandRepository(),
		redisClient: redisClient,
		log:         zap.NewNop(),
	}

	cmd, err := webhookUC.getCustomCommand(context.Background(), clientID, botID, "/rules")
	if err != nil {
		t.Fatalf("getCustomCommand() error = %v", err)
	}
	if cmd == nil {
		t.Fatal("expected custom command from getCustomCommand")
	}
	if cmd.ResponseText != "Ini aturan grup." {
		t.Errorf("ResponseText = %q, want %q", cmd.ResponseText, "Ini aturan grup.")
	}

	key := cacheKey(clientID, botID, "/rules")
	val, err := redisClient.Get(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("expected cache key to exist after read-through: %v", err)
	}
	if val == "" {
		t.Error("cache value should not be empty")
	}
}

func TestCreateCustomCommand_NegativeCache(t *testing.T) {
	uc, redisClient, clientID, botID, cleanup := setupCustomCommandCacheTest(t)
	defer cleanup()

	webhookUC := &TelegramWebhookUseCase{
		db:          uc.db,
		commandRepo: repository.NewCustomCommandRepository(),
		redisClient: redisClient,
		log:         zap.NewNop(),
	}

	cmd, err := webhookUC.getCustomCommand(context.Background(), clientID, botID, "/does-not-exist")
	if err != nil {
		t.Fatalf("getCustomCommand() error = %v", err)
	}
	if cmd != nil {
		t.Fatalf("expected nil command, got %+v", cmd)
	}

	val, err := redisClient.Get(context.Background(), cacheKey(clientID, botID, "/does-not-exist")).Result()
	if err != nil {
		t.Fatalf("expected negative cache sentinel to exist: %v", err)
	}
	if val != "nil" {
		t.Errorf("sentinel = %q, want %q", val, "nil")
	}
}

func TestUpdateCustomCommand_InvalidatesOldAndNewTrigger(t *testing.T) {
	uc, redisClient, clientID, botID, cleanup := setupCustomCommandCacheTest(t)
	defer cleanup()

	webhookUC := &TelegramWebhookUseCase{
		db:          uc.db,
		commandRepo: repository.NewCustomCommandRepository(),
		redisClient: redisClient,
		log:         zap.NewNop(),
	}

	req := &model.CreateCustomCommandRequest{
		BotID:          botID,
		CommandTrigger: "rules",
		ResponseType:   "text",
		ResponseText:   "Aturan grup.",
	}
	created, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	oldKey := cacheKey(clientID, botID, "/rules")
	if _, err := webhookUC.getCustomCommand(context.Background(), clientID, botID, "/rules"); err != nil {
		t.Fatalf("getCustomCommand() error = %v", err)
	}
	if _, err := redisClient.Get(context.Background(), oldKey).Result(); err != nil {
		t.Fatalf("expected old key present after read-through: %v", err)
	}

	newTrigger := "faq"
	updateReq := &model.UpdateCustomCommandRequest{CommandTrigger: &newTrigger}
	updated, err := uc.Update(context.Background(), clientID, created.ID, updateReq)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.CommandTrigger != "/faq" {
		t.Errorf("CommandTrigger = %q, want %q", updated.CommandTrigger, "/faq")
	}

	if err := redisClient.Get(context.Background(), oldKey).Err(); err != redis.Nil {
		t.Errorf("old cache key should be invalidated, got err = %v", err)
	}
	newKey := cacheKey(clientID, botID, "/faq")
	if err := redisClient.Get(context.Background(), newKey).Err(); err != redis.Nil {
		t.Errorf("new cache key should be invalidated (not yet read), got err = %v", err)
	}

	if _, err := webhookUC.getCustomCommand(context.Background(), clientID, botID, "/faq"); err != nil {
		t.Fatalf("getCustomCommand(/faq) error = %v", err)
	}
	if _, err := redisClient.Get(context.Background(), newKey).Result(); err != nil {
		t.Errorf("new cache key should be present after read-through, got err = %v", err)
	}
}

func TestDeleteCustomCommand_InvalidatesCache(t *testing.T) {
	uc, redisClient, clientID, botID, cleanup := setupCustomCommandCacheTest(t)
	defer cleanup()

	webhookUC := &TelegramWebhookUseCase{
		db:          uc.db,
		commandRepo: repository.NewCustomCommandRepository(),
		redisClient: redisClient,
		log:         zap.NewNop(),
	}

	req := &model.CreateCustomCommandRequest{
		BotID:          botID,
		CommandTrigger: "rules",
		ResponseType:   "text",
		ResponseText:   "Aturan grup.",
	}
	created, err := uc.Create(context.Background(), clientID, req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	key := cacheKey(clientID, botID, "/rules")
	if _, err := webhookUC.getCustomCommand(context.Background(), clientID, botID, "/rules"); err != nil {
		t.Fatalf("getCustomCommand() error = %v", err)
	}
	if _, err := redisClient.Get(context.Background(), key).Result(); err != nil {
		t.Fatalf("expected key present after read-through: %v", err)
	}

	if err := uc.Delete(context.Background(), clientID, created.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if err := redisClient.Get(context.Background(), key).Err(); err != redis.Nil {
		t.Errorf("cache key should be invalidated after Delete, got err = %v", err)
	}
}
