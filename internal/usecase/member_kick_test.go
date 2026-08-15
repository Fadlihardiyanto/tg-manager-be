package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMemberKickTest(t *testing.T) (*memberUseCase, *entity.Database, uuid.UUID, func()) {
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
	createTable(`CREATE TABLE packages (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL,
		name TEXT NOT NULL, description TEXT, price REAL NOT NULL,
		duration_days INTEGER NOT NULL, is_all_access INTEGER DEFAULT 0,
		is_active INTEGER DEFAULT 1, max_purchases_per_member INTEGER DEFAULT 0,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE groups (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL, bot_id TEXT NOT NULL,
		telegram_chat_id INTEGER NOT NULL, name TEXT, description TEXT,
		is_active INTEGER DEFAULT 1, inactive_reason TEXT, member_count INTEGER DEFAULT 0,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE package_groups (
		package_id TEXT NOT NULL, group_id TEXT NOT NULL,
		PRIMARY KEY (package_id, group_id)
	)`)
	createTable(`CREATE TABLE outbox (
		id TEXT PRIMARY KEY, aggregate_type TEXT NOT NULL, aggregate_id TEXT NOT NULL,
		event_type TEXT NOT NULL, payload BLOB NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending', retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3, last_error TEXT,
		process_after DATETIME, processed_at DATETIME,
		created_at DATETIME, updated_at DATETIME
	)`)
	createTable(`CREATE TABLE audit_logs (
		id TEXT PRIMARY KEY, client_id TEXT, entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL, action TEXT NOT NULL,
		actor_type TEXT, actor_id TEXT, metadata BLOB,
		created_at DATETIME
	)`)

	database := &entity.Database{Gorm: db}
	uc := NewMemberUseCase(
		database,
		repository.NewTelegramUserRepository(),
		repository.NewSubscriptionRepository(),
		repository.NewOutboxRepository(),
		repository.NewAuditLogRepository(),
		zap.NewNop(),
	).(*memberUseCase)

	clientID := uuid.New()
	now := time.Now()

	db.Exec(`INSERT INTO telegram_users (id, telegram_user_id, username, first_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New(), 0, "seed", "Seed", now, now)

	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}

	return uc, database, clientID, cleanup
}

func insertKickFixture(t *testing.T, db *entity.Database, clientID, userID, subID uuid.UUID, expiredAt time.Time, status string) (packageID, groupID uuid.UUID) {
	t.Helper()
	now := time.Now()
	packageID = uuid.New()
	groupID = uuid.New()

	exec := func(sql string, args ...any) {
		if err := db.Gorm.Exec(sql, args...).Error; err != nil {
			t.Fatalf("fixture exec error: %v\nSQL: %s", err, sql)
		}
	}

	exec(`INSERT INTO telegram_users (id, telegram_user_id, username, first_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, 123456, "member", "Member", now, now)
	exec(`INSERT INTO packages (id, client_id, name, price, duration_days, is_active, created_at, updated_at) VALUES (?, ?, ?, 50000, 30, 1, ?, ?)`,
		packageID, clientID, "Basic", now, now)
	exec(`INSERT INTO groups (id, client_id, bot_id, telegram_chat_id, name, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		groupID, clientID, uuid.New(), -100111, "Group 1", now, now)
	exec(`INSERT INTO package_groups (package_id, group_id) VALUES (?, ?)`, packageID, groupID)
	exec(`INSERT INTO subscriptions (id, telegram_user_id, package_id, client_id, status, activated_at, expired_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		subID, userID, packageID, clientID, status, now, expiredAt, now, now)
	return packageID, groupID
}

func TestBulkKickMembers_Success(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	insertKickFixture(t, db, clientID, userID, subID, time.Now().Add(30*24*time.Hour), "active")

	result := uc.BulkKickMembers(context.Background(), clientID, []model.BulkMemberKickItem{
		{MemberID: userID, SubscriptionID: subID},
	})

	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none", result.Failed)
	}

	var sub entity.Subscription
	if err := db.Gorm.Where("id = ?", subID).First(&sub).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	if sub.Status != "cancelled" {
		t.Errorf("subscription status = %q, want cancelled", sub.Status)
	}
	if sub.KickedAt == nil {
		t.Error("expected KickedAt to be set")
	}

	var outboxCount int64
	db.Gorm.Model(&entity.Outbox{}).Where("event_type = ?", "enforcer.kick").Count(&outboxCount)
	if outboxCount != 1 {
		t.Errorf("outbox kick events = %d, want 1", outboxCount)
	}

	var auditCount int64
	db.Gorm.Model(&entity.AuditLog{}).Where("action = ?", "kick_member").Count(&auditCount)
	if auditCount != 1 {
		t.Errorf("audit logs = %d, want 1", auditCount)
	}
}

func TestBulkKickMembers_SkipsNotFoundSubscription(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	insertKickFixture(t, db, clientID, userID, subID, time.Now().Add(30*24*time.Hour), "active")

	// subscription belongs to member but a different client → should skip
	otherClient := uuid.New()
	result := uc.BulkKickMembers(context.Background(), otherClient, []model.BulkMemberKickItem{
		{MemberID: userID, SubscriptionID: subID},
	})

	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0 (skipped)", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none (silent skip)", result.Failed)
	}
}

func TestBulkKickMembers_SkipsExpired(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	insertKickFixture(t, db, clientID, userID, subID, time.Now().Add(-24*time.Hour), "active")

	result := uc.BulkKickMembers(context.Background(), clientID, []model.BulkMemberKickItem{
		{MemberID: userID, SubscriptionID: subID},
	})

	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0 (expired skipped)", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none", result.Failed)
	}
}

func TestBulkKickMembers_BestEffortMix(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	validUser := uuid.New()
	validSub := uuid.New()
	insertKickFixture(t, db, clientID, validUser, validSub, time.Now().Add(30*24*time.Hour), "active")

	otherClient := uuid.New()
	foreignUser := uuid.New()
	foreignSub := uuid.New()
	insertKickFixture(t, db, otherClient, foreignUser, foreignSub, time.Now().Add(30*24*time.Hour), "active")

	result := uc.BulkKickMembers(context.Background(), clientID, []model.BulkMemberKickItem{
		{MemberID: validUser, SubscriptionID: validSub},
		{MemberID: foreignUser, SubscriptionID: foreignSub}, // foreign → skip
	})

	if result.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Errorf("Failed = %v, want none (skip, not fail)", result.Failed)
	}
}

func TestBulkKickMembers_AuditLogFailureFailsItem(t *testing.T) {
	uc, db, clientID, cleanup := setupMemberKickTest(t)
	defer cleanup()

	userID := uuid.New()
	subID := uuid.New()
	insertKickFixture(t, db, clientID, userID, subID, time.Now().Add(30*24*time.Hour), "active")

	// simulate audit-log storage failure: item must fail and roll back the kick
	if err := db.Gorm.Exec("DROP TABLE audit_logs").Error; err != nil {
		t.Fatalf("drop audit_logs error = %v", err)
	}

	result := uc.BulkKickMembers(context.Background(), clientID, []model.BulkMemberKickItem{
		{MemberID: userID, SubscriptionID: subID},
	})

	if result.Deleted != 0 {
		t.Errorf("Deleted = %d, want 0 (audit failure must fail the item)", result.Deleted)
	}
	if len(result.Failed) != 1 {
		t.Fatalf("Failed = %v, want 1", result.Failed)
	}
	if result.Failed[0].ID != userID {
		t.Errorf("Failed[0].ID = %v, want %v", result.Failed[0].ID, userID)
	}

	var sub entity.Subscription
	if err := db.Gorm.Where("id = ?", subID).First(&sub).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	if sub.Status != "active" {
		t.Errorf("subscription status = %q, want active (kick rolled back)", sub.Status)
	}

	var outboxCount int64
	db.Gorm.Model(&entity.Outbox{}).Where("event_type = ?", "enforcer.kick").Count(&outboxCount)
	if outboxCount != 0 {
		t.Errorf("outbox kick events = %d, want 0 (rolled back)", outboxCount)
	}
}
