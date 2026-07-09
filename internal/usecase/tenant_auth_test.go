package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTenantAuthTest(t *testing.T) (*TenantAuthUseCase, *gorm.DB, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	createTable := func(ddl string) {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table error: %v\nDDL: %s", err, ddl)
		}
	}

	createTable(`CREATE TABLE users (
		id TEXT PRIMARY KEY, email TEXT NOT NULL, name TEXT NOT NULL,
		password_hash TEXT NOT NULL, phone TEXT, avatar_url TEXT,
		is_email_verified INTEGER DEFAULT 0, last_login_at DATETIME,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE outbox (
		id TEXT PRIMARY KEY, aggregate_type TEXT NOT NULL, aggregate_id TEXT NOT NULL,
		event_type TEXT NOT NULL, payload BLOB NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending', retry_count INTEGER NOT NULL DEFAULT 0,
		max_retries INTEGER NOT NULL DEFAULT 3, last_error TEXT,
		process_after DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, processed_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	createTable(`CREATE TABLE clients (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE,
		owner_id TEXT, logo_url TEXT, bot_token TEXT, bot_username TEXT,
		is_active INTEGER DEFAULT 1, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE client_users (
		id TEXT PRIMARY KEY, client_id TEXT NOT NULL, user_id TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'owner', is_active INTEGER DEFAULT 1,
		created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE roles (
		id TEXT PRIMARY KEY, client_id TEXT, name TEXT NOT NULL,
		description TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)
	createTable(`CREATE TABLE permissions (
		id TEXT PRIMARY KEY, role_id TEXT, client_id TEXT,
		permission TEXT NOT NULL, resource TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
	)`)

	log := zap.NewNop()

	uc := &TenantAuthUseCase{
		db:             &entity.Database{Gorm: db},
		userRepo:       repository.NewUserRepository(log),
		clientRepo:     repository.NewClientRepository(log),
		clientUserRepo: repository.NewClientUserRepository(log),
		permissionRepo: repository.NewTenantPermissionRepository(log),
		outboxRepo:     repository.NewOutboxRepository(),
		log:            log,
		redis:          redis.NewClient(&redis.Options{Addr: mr.Addr(), DB: 0}),
		jwtConfig: &pkg_jwt.JWTConfig{
			AdminSecretKey:      "admin-secret-key-for-testing-purposes-123",
			AdminAccessExpiry:   time.Hour,
			AdminRefreshExpiry:  24 * time.Hour,
			TenantSecretKey:     "tenant-secret-key-for-testing-purposes-456",
			TenantAccessExpiry:  time.Hour,
			TenantRefreshExpiry: 24 * time.Hour,
			Issuer:              "test-issuer",
		},
		frontendURL: "http://localhost:3000",
		bcryptCost:  4,
	}

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		mr.Close()
	}

	return uc, db, cleanup
}

func TestTenantAuthRegister_Success(t *testing.T) {
	uc, db, cleanup := setupTenantAuthTest(t)
	defer cleanup()

	ctx := context.Background()
	req := &model.TenantRegisterRequest{
		Email:    "test@example.com",
		Password: "Password123!",
		Name:     "Test User",
		Phone:    "08123456789",
	}

	resp, err := uc.Register(ctx, req)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if resp.User.Email != req.Email {
		t.Errorf("Email = %q, want %q", resp.User.Email, req.Email)
	}
	if resp.User.Name != req.Name {
		t.Errorf("Name = %q, want %q", resp.User.Name, req.Name)
	}
	if resp.User.ID == uuid.Nil {
		t.Error("expected non-nil user ID")
	}

	// Verify DB has the user
	var user entity.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		t.Fatalf("user not found in DB: %v", err)
	}
	if user.Email != req.Email {
		t.Errorf("DB email = %q, want %q", user.Email, req.Email)
	}
	if user.PasswordHash == "" {
		t.Error("password hash should not be empty")
	}
	if user.PasswordHash == req.Password {
		t.Error("password stored in plaintext!")
	}

	// Verify outbox event created
	var outboxCount int64
	db.Model(&entity.Outbox{}).Where("aggregate_id = ?", user.ID).Count(&outboxCount)
	if outboxCount == 0 {
		t.Error("expected outbox event for verification email")
	}
}

func TestTenantAuthRegister_DuplicateEmail(t *testing.T) {
	uc, _, cleanup := setupTenantAuthTest(t)
	defer cleanup()

	ctx := context.Background()
	req := &model.TenantRegisterRequest{
		Email:    "duplicate@example.com",
		Password: "Password123!",
		Name:     "First User",
	}

	_, err := uc.Register(ctx, req)
	if err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	_, err = uc.Register(ctx, req)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}

	conflict, ok := err.(*helper.ErrConflict)
	if !ok {
		t.Fatalf("expected ErrConflict, got %T: %v", err, err)
	}
	if conflict.Error() != "Email sudah terdaftar" {
		t.Errorf("error = %q, want %q", conflict.Error(), "Email sudah terdaftar")
	}
}

func TestTenantAuthLogin_Success(t *testing.T) {
	uc, _, cleanup := setupTenantAuthTest(t)
	defer cleanup()

	ctx := context.Background()
	password := "StrongPass1!"

	// Register first
	_, err := uc.Register(ctx, &model.TenantRegisterRequest{
		Email:    "login-test@example.com",
		Password: password,
		Name:     "Login Test",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify email (set is_email_verified = true directly since it's stored in Redis)
	var user entity.User
	if err := uc.db.Gorm.Where("email = ?", "login-test@example.com").First(&user).Error; err != nil {
		t.Fatalf("user not found: %v", err)
	}
	user.IsEmailVerified = true
	if err := uc.db.Gorm.Save(&user).Error; err != nil {
		t.Fatalf("update user error = %v", err)
	}

	// Login
	resp, err := uc.Login(ctx, &model.TenantLoginRequest{
		Email:    "login-test@example.com",
		Password: password,
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if resp.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
}

func TestTenantAuthLogin_WrongPassword(t *testing.T) {
	uc, _, cleanup := setupTenantAuthTest(t)
	defer cleanup()

	ctx := context.Background()

	// Register
	_, err := uc.Register(ctx, &model.TenantRegisterRequest{
		Email:    "wrong-pass@example.com",
		Password: "CorrectPass1!",
		Name:     "Wrong Pass",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify email
	var user entity.User
	uc.db.Gorm.Where("email = ?", "wrong-pass@example.com").First(&user)
	user.IsEmailVerified = true
	uc.db.Gorm.Save(&user)

	// Try wrong password
	_, err = uc.Login(ctx, &model.TenantLoginRequest{
		Email:    "wrong-pass@example.com",
		Password: "WrongPass1!",
	})
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestTenantAuthLogin_EmailNotVerified(t *testing.T) {
	uc, _, cleanup := setupTenantAuthTest(t)
	defer cleanup()

	ctx := context.Background()

	// Register but don't verify email
	_, err := uc.Register(ctx, &model.TenantRegisterRequest{
		Email:    "unverified@example.com",
		Password: "Password123!",
		Name:     "Unverified",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err = uc.Login(ctx, &model.TenantLoginRequest{
		Email:    "unverified@example.com",
		Password: "Password123!",
	})
	if err == nil {
		t.Fatal("expected error for unverified email, got nil")
	}
}

func TestTenantAuthLogin_EmailNotFound(t *testing.T) {
	uc, _, cleanup := setupTenantAuthTest(t)
	defer cleanup()

	ctx := context.Background()
	_, err := uc.Login(ctx, &model.TenantLoginRequest{
		Email:    "nonexistent@example.com",
		Password: "Password123!",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent email, got nil")
	}
}
