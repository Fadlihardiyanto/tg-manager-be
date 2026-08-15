package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPurchaseLimitTest(t *testing.T) (*memberOrderUseCase, uuid.UUID, uuid.UUID, func()) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}
	cleanup := func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
	if err := db.Exec(`CREATE TABLE orders (
		id TEXT PRIMARY KEY, telegram_user_id TEXT NOT NULL,
		package_id TEXT NOT NULL, client_id TEXT NOT NULL,
		external_id TEXT NOT NULL, amount REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create orders table error = %v", err)
	}

	uc := &memberOrderUseCase{
		db:        &entity.Database{Gorm: db},
		orderRepo: repository.NewOrderRepository(),
		log:       zap.NewNop(),
	}

	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	insertPaid := func() {
		t.Helper()
		if err := db.Exec(`INSERT INTO orders (id, telegram_user_id, package_id, client_id, external_id, amount, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'paid', ?, ?)`,
			uuid.New(), userID, packageID, uuid.New(), "ORDER-X", 50000, now, now).Error; err != nil {
			t.Fatalf("insert paid order error = %v", err)
		}
	}

	insertPaid()
	insertPaid()

	return uc, userID, packageID, cleanup
}

func TestCheckPurchaseLimit(t *testing.T) {
	uc, userID, packageID, cleanup := setupPurchaseLimitTest(t)
	defer cleanup()
	ctx := context.Background()

	// sub-case: limit 0 → unlimited
	if err := uc.checkPurchaseLimit(ctx, uc.db.Gorm, userID, &entity.Package{ID: packageID, MaxPurchasesPerMember: 0}); err != nil {
		t.Errorf("limit 0: got error %v, want nil", err)
	}

	// sub-case: paid count 2, limit 5 → lolos
	if err := uc.checkPurchaseLimit(ctx, uc.db.Gorm, userID, &entity.Package{ID: packageID, MaxPurchasesPerMember: 5}); err != nil {
		t.Errorf("limit 5 (count 2): got error %v, want nil", err)
	}

	// sub-case: paid count 2, limit 2 → diblokir
	err := uc.checkPurchaseLimit(ctx, uc.db.Gorm, userID, &entity.Package{ID: packageID, MaxPurchasesPerMember: 2})
	if err == nil {
		t.Fatal("limit 2 (count 2): got nil, want ErrBadRequest")
	}
	var errBadRequest *helper.ErrBadRequest
	if !errors.As(err, &errBadRequest) {
		t.Errorf("limit 2 (count 2): got %T, want *helper.ErrBadRequest", err)
	}
}
