package usecase_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
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

func setupBillingSqlMock() (*entity.Database, sqlmock.Sqlmock) {
	sqlDB, mockObj, _ := sqlmock.New()
	mockObj.ExpectBegin()
	mockObj.ExpectCommit()
	gormDB, _ := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{SkipDefaultTransaction: true})
	return &entity.Database{Gorm: gormDB}, mockObj
}

func setupBillingMiniredis() *redis.Client {
	mr, _ := miniredis.Run()
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

func TestClientBillingUseCase_CancelBilling_Success(t *testing.T) {
	db, _ := setupBillingSqlMock()
	billingRepo := new(repoMocks.IClientBillingRepository)
	planRepo := new(repoMocks.IPlatformPlanRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IPlatformDiscountRepository)
	discountUC := new(ucMocks.IPlatformDiscountUseCase)
	redisClient := setupBillingMiniredis()
	log := zaptest.NewLogger(t)

	midtransClient := midtrans.NewClient(midtrans.Config{})

	uc := usecase.NewClientBillingUseCase(
		db, billingRepo, planRepo, clientRepo, discountRepo, discountUC, midtransClient, redisClient, log,
	)

	ctx := context.Background()
	adminID := uuid.New()
	billingID := uuid.New()

	req := &model.CancelBillingRequest{
		BillingID:         billingID,
		Reason:            "Pelanggaran TOS",
		CallerPermissions: []string{"billing.manage"},
		AdminID:           adminID,
	}

	mockBilling := entity.ClientBilling{
		ID:     billingID,
		Status: "active",
	}

	billingRepo.On("FindByID", mock.Anything, mock.Anything, billingID).Return(&mockBilling, nil)
	billingRepo.On("Update", mock.Anything, mock.Anything, mock.AnythingOfType("*entity.ClientBilling")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*entity.ClientBilling)
		assert.Equal(t, "cancelled", arg.Status)
		assert.Equal(t, adminID, *arg.CancelledBy)
		assert.Equal(t, adminID, *arg.UpdatedBy)
		assert.Equal(t, "Pelanggaran TOS", arg.Note)
	})

	err := uc.CancelBilling(ctx, req)

	assert.NoError(t, err)
	billingRepo.AssertExpectations(t)
}

func TestClientBillingUseCase_CancelBilling_Forbidden(t *testing.T) {
	db, _ := setupBillingSqlMock()
	billingRepo := new(repoMocks.IClientBillingRepository)
	planRepo := new(repoMocks.IPlatformPlanRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IPlatformDiscountRepository)
	discountUC := new(ucMocks.IPlatformDiscountUseCase)
	redisClient := setupBillingMiniredis()
	log := zaptest.NewLogger(t)
	midtransClient := midtrans.NewClient(midtrans.Config{})

	uc := usecase.NewClientBillingUseCase(
		db, billingRepo, planRepo, clientRepo, discountRepo, discountUC, midtransClient, redisClient, log,
	)

	ctx := context.Background()
	req := &model.CancelBillingRequest{
		BillingID:         uuid.New(),
		CallerPermissions: []string{"billing.read"}, // missing manage
		AdminID:           uuid.New(),
	}

	err := uc.CancelBilling(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "izin")
}

func TestClientBillingUseCase_Checkout_Success(t *testing.T) {
	db, _ := setupBillingSqlMock()
	billingRepo := new(repoMocks.IClientBillingRepository)
	planRepo := new(repoMocks.IPlatformPlanRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IPlatformDiscountRepository)
	discountUC := new(ucMocks.IPlatformDiscountUseCase)
	redisClient := setupBillingMiniredis()
	log := zaptest.NewLogger(t)
	midtransClient := midtrans.NewClient(midtrans.Config{
		ServerKey: "dummy-key",
		BaseURL:   "https://api.sandbox.midtrans.com",
		SnapURL:   "https://app.sandbox.midtrans.com/snap/v1",
	})

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("POST", "https://app.sandbox.midtrans.com/snap/v1/transactions",
		httpmock.NewStringResponder(200, `{"token":"test-snap-token","redirect_url":"https://test-redirect.com"}`))

	uc := usecase.NewClientBillingUseCase(
		db, billingRepo, planRepo, clientRepo, discountRepo, discountUC, midtransClient, redisClient, log,
	)

	ctx := context.Background()
	clientID := uuid.New()
	planID := uuid.New()

	req := &model.ClientCheckoutPlanRequest{
		ClientID:     clientID,
		PlanID:       planID,
		BillingCycle: "monthly",
	}

	// Mock existing active or pending billing (none)
	billingRepo.On("FindActiveOrPendingByClientID", mock.Anything, mock.Anything, clientID).Return((*entity.ClientBilling)(nil), nil)

	// Mock fetch plan
	plan := &entity.PlatformPlan{
		ID:           planID,
		PriceMonthly: decimal.NewFromInt(100000),
		IsActive:     true,
	}
	planRepo.On("FindByID", mock.Anything, mock.Anything, planID).Return(plan, nil)

	client := &entity.Client{
		ID:    clientID,
		Name:  "Test Client",
		Owner: &entity.User{Email: "admin@test.com"},
	}
	clientRepo.On("FindByIDWithOwner", mock.Anything, mock.Anything, clientID).Return(client, nil)

	// Mock apply auto discount (none)
	discountUC.On("ApplyAuto", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return((*model.DiscountPreviewResponse)(nil), nil)

	// Mock create billing
	billingRepo.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*entity.ClientBilling")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*entity.ClientBilling)
		assert.Equal(t, "pending", arg.Status)
		assert.Equal(t, decimal.NewFromInt(100000), arg.Amount)
	})

	resp, err := uc.Checkout(ctx, req)
	assert.NoError(t, err)
	if resp != nil {
		assert.NotEmpty(t, resp.ExternalID)
	}

	billingRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	clientRepo.AssertExpectations(t)
}

func TestClientBillingUseCase_Checkout_AlreadyActive(t *testing.T) {
	db, _ := setupBillingSqlMock()
	billingRepo := new(repoMocks.IClientBillingRepository)
	planRepo := new(repoMocks.IPlatformPlanRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IPlatformDiscountRepository)
	discountUC := new(ucMocks.IPlatformDiscountUseCase)
	redisClient := setupBillingMiniredis()
	log := zaptest.NewLogger(t)
	midtransClient := midtrans.NewClient(midtrans.Config{})

	uc := usecase.NewClientBillingUseCase(
		db, billingRepo, planRepo, clientRepo, discountRepo, discountUC, midtransClient, redisClient, log,
	)

	ctx := context.Background()
	clientID := uuid.New()
	req := &model.ClientCheckoutPlanRequest{
		ClientID:     clientID,
		PlanID:       uuid.New(),
		BillingCycle: "monthly",
	}

	// Mock fetch plan
	plan := &entity.PlatformPlan{
		ID:           req.PlanID,
		PriceMonthly: decimal.NewFromInt(100000),
		IsActive:     true,
	}
	planRepo.On("FindByID", mock.Anything, mock.Anything, req.PlanID).Return(plan, nil)

	client := &entity.Client{
		ID:    clientID,
		Name:  "Test Client",
		Owner: &entity.User{Email: "test@example.com"},
	}
	clientRepo.On("FindByIDWithOwner", mock.Anything, mock.Anything, clientID).Return(client, nil)

	// Mock existing active
	existing := &entity.ClientBilling{
		Status: "active",
	}
	billingRepo.On("FindActiveOrPendingByClientID", mock.Anything, mock.Anything, clientID).Return(existing, nil)

	resp, err := uc.Checkout(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "masih memiliki billing aktif")
}

func TestClientBillingUseCase_AdminAssignPlan_Success(t *testing.T) {
	db, sqlMock := setupBillingSqlMock()
	billingRepo := new(repoMocks.IClientBillingRepository)
	planRepo := new(repoMocks.IPlatformPlanRepository)
	clientRepo := new(repoMocks.IClientRepository)
	discountRepo := new(repoMocks.IPlatformDiscountRepository)
	discountUC := new(ucMocks.IPlatformDiscountUseCase)
	redisClient := setupBillingMiniredis()
	log := zaptest.NewLogger(t)
	midtransClient := midtrans.NewClient(midtrans.Config{})

	uc := usecase.NewClientBillingUseCase(
		db, billingRepo, planRepo, clientRepo, discountRepo, discountUC, midtransClient, redisClient, log,
	)

	ctx := context.Background()
	adminID := uuid.New()
	clientID := uuid.New()
	planID := uuid.New()

	req := &model.AdminAssignPlanRequest{
		AdminID:           adminID,
		ClientID:          clientID,
		PlanID:            planID,
		BillingCycle:      "yearly",
		CallerPermissions: []string{"billing.manage"},
		Note:              "Manual assignment",
	}

	plan := &entity.PlatformPlan{
		ID:          planID,
		PriceYearly: decimal.NewFromInt(1000000),
		IsActive:    true,
	}
	planRepo.On("FindByID", mock.Anything, mock.Anything, planID).Return(plan, nil)

	client := &entity.Client{
		ID:   clientID,
		Name: "Test Client",
	}
	clientRepo.On("FindByID", mock.Anything, mock.Anything, clientID).Return(client, nil)

	billingRepo.On("FindActiveOrPendingByClientID", mock.Anything, mock.Anything, clientID).Return((*entity.ClientBilling)(nil), nil)

	// Since activateClientPlan executes a raw GORM Updates on the db object, we need to mock it
	sqlMock.ExpectBegin()
	sqlMock.ExpectExec(regexp.QuoteMeta(`UPDATE "clients" SET "subscription_tier"=$1,"updated_at"=$2 WHERE id = $3`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	sqlMock.ExpectCommit()

	billingRepo.On("Create", mock.Anything, mock.Anything, mock.AnythingOfType("*entity.ClientBilling")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*entity.ClientBilling)
		assert.Equal(t, "active", arg.Status)
		assert.Equal(t, decimal.NewFromInt(1000000), arg.Amount)
		assert.True(t, arg.IsManual)
		assert.Equal(t, "Manual assignment", arg.Note)
		assert.Equal(t, adminID, *arg.CreatedBy)
	})

	resp, err := uc.AdminAssignPlan(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "active", resp.Status)

	billingRepo.AssertExpectations(t)
	planRepo.AssertExpectations(t)
	clientRepo.AssertExpectations(t)
}
