package controller

import (
	"strconv"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/delivery/http/middleware"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/usecase"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type TenantTransactionController struct {
	txUC usecase.ITenantTransactionUseCase
	log  *zap.Logger
}

func NewTenantTransactionController(uc usecase.ITenantTransactionUseCase, log *zap.Logger) *TenantTransactionController {
	return &TenantTransactionController{
		txUC: uc,
		log:  log,
	}
}

// List godoc
// GET /api/v1/tenant/transactions
func (c *TenantTransactionController) List(ctx fiber.Ctx) error {
	log := logger.FromContext(ctx.Context(), c.log)
	log.Info("tenant transaction controller list request")

	clientID := middleware.GetTenantClientID(ctx)

	// Parse query params
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	limit, _ := strconv.Atoi(ctx.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	status := ctx.Query("status", "all")
	search := ctx.Query("search", "")
	paymentMethod := ctx.Query("payment_method", "")

	var packageID uuid.UUID
	if pid := ctx.Query("package_id", ""); pid != "" {
		parsed, err := uuid.Parse(pid)
		if err != nil {
			log.Warn("tenant transaction list invalid package_id", zap.String("package_id", pid))
			return helper.BadRequest(ctx, "package_id tidak valid")
		}
		packageID = parsed
	}

	// Date range filters
	var dateFrom, dateTo *time.Time
	if df := ctx.Query("date_from", ""); df != "" {
		t, err := time.Parse("2006-01-02", df)
		if err != nil {
			log.Warn("tenant transaction list invalid date_from", zap.String("date_from", df))
			return helper.BadRequest(ctx, "date_from tidak valid, format: YYYY-MM-DD")
		}
		dateFrom = &t
	}
	if dt := ctx.Query("date_to", ""); dt != "" {
		t, err := time.Parse("2006-01-02", dt)
		if err != nil {
			log.Warn("tenant transaction list invalid date_to", zap.String("date_to", dt))
			return helper.BadRequest(ctx, "date_to tidak valid, format: YYYY-MM-DD")
		}
		// Set to end of day
		endOfDay := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		dateTo = &endOfDay
	}

	filter := model.TransactionFilterRequest{
		Page:          page,
		Limit:         limit,
		Status:        status,
		Search:        search,
		PackageID:     packageID,
		PaymentMethod: paymentMethod,
		DateFrom:      dateFrom,
		DateTo:        dateTo,
	}

	result, total, err := c.txUC.FindAll(ctx.Context(), clientID, filter)
	if err != nil {
		log.Error("tenant transaction controller list failed", zap.Error(err))
		return err
	}

	meta := helper.NewMeta(page, limit, total)
	return helper.SuccessWithMeta(ctx, "Berhasil mengambil daftar transaksi", result, meta)
}
