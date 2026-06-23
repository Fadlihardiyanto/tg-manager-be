package usecase_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	repoMocks "github.com/Fadlihardiyanto/telegram-management-app/tests/mocks/repository"
	ucMocks "github.com/Fadlihardiyanto/telegram-management-app/tests/mocks/usecase"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/redis/go-redis/v9"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMiniredis() *redis.Client {
	mr, _ := miniredis.Run()
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

func setupSqlMock() *entity.Database {
	sqlDB, mock, _ := sqlmock.New()
	mock.ExpectBegin()
	mock.ExpectCommit()
	gormDB, _ := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{SkipDefaultTransaction: true})
	return &entity.Database{Gorm: gormDB}
}

func TestMemberOrderUseCase_Checkout_Success(t *testing.T) {
	// Setup Mocks
	db := setupSqlMock()
	orderRepo := new(repoMocks.IOrderRepository)
	subRepo := new(repoMocks.ISubscriptionRepository)
	pkgRepo := new(repoMocks.IPackageRepository)
	tgUserRepo := new(repoMocks.ITelegramUserRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IMemberDiscountRepository)
	memberDiscountUC := new(ucMocks.IMemberDiscountUseCase)
	outboxRepo := new(repoMocks.IOutboxRepository)
	redisClient := setupMiniredis()
	log := zaptest.NewLogger(t)

	uc := usecase.NewMemberOrderUseCase(
		db, orderRepo, subRepo, pkgRepo, tgUserRepo, clientRepo, discountRepo, memberDiscountUC, outboxRepo, redisClient, log, "12345678901234567890123456789012", "https://api.sandbox.midtrans.com", "https://app.sandbox.midtrans.com/snap/v1",
	)

	ctx := context.Background()
	req := &model.MemberCheckoutRequest{
		PackageID:      uuid.New(),
		TelegramUserID: 123456789,
		DiscountCode:   nil,
	}

	pkgID := req.PackageID
	clientID := uuid.New()
	mockPkg := entity.Package{
		ID:           pkgID,
		ClientID:     clientID,
		Name:         "VIP Monthly",
		Price:        decimal.NewFromInt(50000),
		DurationDays: 30,
		IsActive:     true,
	}

	mockTgUser := entity.TelegramUser{
		ID:             uuid.New(),
		TelegramUserID: req.TelegramUserID,
	}

	// Expectations
	pkgRepo.On("FindByID", mock.Anything, mock.Anything, pkgID).
		Return(&mockPkg, nil)

	memberDiscountUC.On("ApplyAuto", mock.Anything, mock.AnythingOfType("*model.ApplyMemberDiscountRequest")).
		Return(nil, nil)

	tgUserRepo.On("FindByTelegramID", mock.Anything, mock.Anything, req.TelegramUserID).
		Return(&mockTgUser, nil)

	orderRepo.On("FindPendingOrderByUserAndPackage", mock.Anything, mock.Anything, mockTgUser.ID, pkgID).
		Return(nil, gorm.ErrRecordNotFound)

	serverKey, err1 := crypto.Encrypt("SB-Mid-server-key", "12345678901234567890123456789012")
	if err1 != nil {
		panic(err1)
	}
	clientKey, err2 := crypto.Encrypt("SB-Mid-client-key", "12345678901234567890123456789012")
	if err2 != nil {
		panic(err2)
	}
	mockClient := entity.Client{
		ID:                       clientID,
		MidtransSandboxServerKey: &serverKey,
		MidtransSandboxClientKey: &clientKey,
		MidtransIsSandbox:        true,
	}
	clientRepo.On("FindByID", mock.Anything, mock.Anything, clientID).
		Return(&mockClient, nil)

	orderRepo.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*entity.Order")).
		Return(nil)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", "https://app.sandbox.midtrans.com/snap/v1/transactions",
		httpmock.NewStringResponder(200, `{"token":"test-snap-token","redirect_url":"https://test-payment-url.com"}`))

	// Action
	res, err := uc.Checkout(ctx, req)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "50000", res.Amount.String())
	assert.NotEmpty(t, res.PaymentURL)

	pkgRepo.AssertExpectations(t)
	tgUserRepo.AssertExpectations(t)
	orderRepo.AssertExpectations(t)
}

func TestMemberOrderUseCase_Checkout_PackageNotFound(t *testing.T) {
	// Setup Mocks
	db := setupSqlMock()
	orderRepo := new(repoMocks.IOrderRepository)
	subRepo := new(repoMocks.ISubscriptionRepository)
	pkgRepo := new(repoMocks.IPackageRepository)
	tgUserRepo := new(repoMocks.ITelegramUserRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IMemberDiscountRepository)
	memberDiscountUC := new(ucMocks.IMemberDiscountUseCase)
	outboxRepo := new(repoMocks.IOutboxRepository)
	redisClient := setupMiniredis()
	log := zaptest.NewLogger(t)

	uc := usecase.NewMemberOrderUseCase(
		db, orderRepo, subRepo, pkgRepo, tgUserRepo, clientRepo, discountRepo, memberDiscountUC, outboxRepo, redisClient, log, "12345678901234567890123456789012", "https://api.sandbox.midtrans.com", "https://app.sandbox.midtrans.com/snap/v1",
	)

	ctx := context.Background()
	req := &model.MemberCheckoutRequest{
		PackageID:      uuid.New(),
		TelegramUserID: 123456789,
	}

	pkgRepo.On("FindByID", mock.Anything, mock.Anything, req.PackageID).
		Return(nil, gorm.ErrRecordNotFound)

	res, err := uc.Checkout(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "Paket tidak ditemukan")
}
