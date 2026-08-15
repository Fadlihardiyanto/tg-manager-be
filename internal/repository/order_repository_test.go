package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOrderCountTest(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID, func()) {
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

	userID := uuid.New()
	packageID := uuid.New()
	now := time.Now()

	insertOrder := func(status string, packageID uuid.UUID, deleted bool) {
		t.Helper()
		var deletedAt any
		if deleted {
			deletedAt = now
		}
		if err := db.Exec(`INSERT INTO orders (id, telegram_user_id, package_id, client_id, external_id, amount, status, created_at, updated_at, deleted_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New(), userID, packageID, uuid.New(), "ORDER-X", 50000, status, now, now, deletedAt).Error; err != nil {
			t.Fatalf("insert order error = %v", err)
		}
	}

	insertOrder("paid", packageID, false)
	insertOrder("paid", packageID, false)
	insertOrder("pending", packageID, false)
	insertOrder("paid", packageID, true)
	insertOrder("paid", uuid.New(), false)

	return db, userID, packageID, cleanup
}

func TestCountPaidByUserAndPackage(t *testing.T) {
	db, userID, packageID, cleanup := setupOrderCountTest(t)
	defer cleanup()

	repo := NewOrderRepository()
	count, err := repo.CountPaidByUserAndPackage(context.Background(), db, userID, packageID)
	if err != nil {
		t.Fatalf("CountPaidByUserAndPackage error = %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (hanya paid, deleted diabaikan)", count)
	}
}

func TestCountPaidByUserAndPackages(t *testing.T) {
	db, userID, packageID, cleanup := setupOrderCountTest(t)
	defer cleanup()

	otherPackage := uuid.New()
	repo := NewOrderRepository()

	counts, err := repo.CountPaidByUserAndPackages(context.Background(), db, userID, []uuid.UUID{packageID, otherPackage})
	if err != nil {
		t.Fatalf("CountPaidByUserAndPackages error = %v", err)
	}
	if counts[packageID] != 2 {
		t.Errorf("counts[package] = %d, want 2", counts[packageID])
	}
	if _, ok := counts[otherPackage]; ok {
		t.Errorf("counts[otherPackage] should be absent, got %d", counts[otherPackage])
	}

	empty, err := repo.CountPaidByUserAndPackages(context.Background(), db, userID, nil)
	if err != nil {
		t.Fatalf("CountPaidByUserAndPackages(empty) error = %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty result len = %d, want 0", len(empty))
	}
}
