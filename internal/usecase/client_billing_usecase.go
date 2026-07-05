package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model/converter"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/rbac"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IClientBillingUseCase interface {
	// Self-service: client checkout plan sendiri
	Checkout(ctx context.Context, req *model.ClientCheckoutPlanRequest) (*model.CheckoutResponse, error)

	// Webhook dari Midtrans
	HandleWebhook(ctx context.Context, req *model.MidtransWebhookRequest) error

	// Superadmin: assign plan manual
	AdminAssignPlan(ctx context.Context, req *model.AdminAssignPlanRequest) (*model.ClientBillingResponse, error)

	// List billing history
	ListBillings(ctx context.Context, req *model.AdminListBillingRequest) ([]model.ClientBillingResponse, int64, error)

	// Cancel billing
	CancelBilling(ctx context.Context, req *model.CancelBillingRequest) error

	// Get active billing milik client
	GetActiveBilling(ctx context.Context, clientID uuid.UUID) (*model.ClientBillingResponse, error)
}

type clientBillingUseCase struct {
	db                   *entity.Database
	billingRepo          repository.IClientBillingRepository
	planRepo             repository.IPlatformPlanRepository
	clientRepo           repository.IClientRepository
	platformDiscountRepo repository.IPlatformDiscountRepository

	platformDiscountUC IPlatformDiscountUseCase
	featureGateUC      IFeatureGateUseCase

	midtransClient *midtrans.Client
	redis          *redis.Client
	log            *zap.Logger
	appBaseURL     string
	appFrontendURL string
}

func NewClientBillingUseCase(
	db *entity.Database,
	billingRepo repository.IClientBillingRepository,
	planRepo repository.IPlatformPlanRepository,
	clientRepo repository.IClientRepository,
	platformDiscountRepo repository.IPlatformDiscountRepository,
	platformDiscountUC IPlatformDiscountUseCase,
	featureGateUC IFeatureGateUseCase,
	midtransClient *midtrans.Client,
	redisClient *redis.Client,
	log *zap.Logger,
	appBaseURL string,
	appFrontendURL string,
) IClientBillingUseCase {
	return &clientBillingUseCase{
		db:                   db,
		billingRepo:          billingRepo,
		planRepo:             planRepo,
		clientRepo:           clientRepo,
		platformDiscountRepo: platformDiscountRepo,
		platformDiscountUC:   platformDiscountUC,
		featureGateUC:        featureGateUC,
		midtransClient:       midtransClient,
		redis:                redisClient,
		log:                  log,
		appBaseURL:           appBaseURL,
		appFrontendURL:       appFrontendURL,
	}
}

func (uc *clientBillingUseCase) Checkout(ctx context.Context, req *model.ClientCheckoutPlanRequest) (*model.CheckoutResponse, error) {
	unlockFn, err := uc.acquireBillingLock(ctx, req.ClientID)
	if err != nil {
		return nil, err
	}
	defer unlockFn()

	// 1. Fetch plan
	plan, err := uc.planRepo.FindByID(ctx, uc.db.Gorm, req.PlanID)
	if err != nil || plan == nil {
		return nil, helper.NewNotFound("plan tidak ditemukan")
	}
	if !plan.IsActive {
		return nil, helper.NewBadRequest("plan tidak tersedia")
	}

	// 2. Fetch client (dengan preload Owner untuk email)
	client, err := uc.clientRepo.FindByIDWithOwner(ctx, uc.db.Gorm, req.ClientID)
	if err != nil || client == nil {
		return nil, helper.NewBadRequest("client tidak ditemukan")
	}

	// 3. Cek apakah sudah ada billing aktif ATAU pending yang belum selesai
	// Cek pending: cegah double checkout untuk plan yang sama
	existing, err := uc.billingRepo.FindActiveOrPendingByClientID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil {
		return nil, helper.NewBadRequest("gagal memeriksa billing aktif")
	}
	if existing != nil {
		if existing.Status == "pending" {
			return nil, helper.NewBadRequest("Anda sudah memiliki billing pending, silakan selesaikan pembayaran atau batalkan terlebih dahulu")
		}
		return nil, helper.NewBadRequest(fmt.Sprintf("client masih memiliki billing aktif hingga %s, batalkan terlebih dahulu",
			existing.ExpiredAt.Format("2 Jan 2006")))
	}

	// 4. Hitung amount & expired_at berdasarkan billing cycle
	originalAmount, expiredAt := uc.calculateBilling(plan, req.BillingCycle)

	var appliedDiscountID *uuid.UUID
	var discountAmount decimal.Decimal
	finalAmount := originalAmount

	if req.DiscountCode != nil && *req.DiscountCode != "" {
		// User input kode promo — wajib valid, error jika tidak
		preview, err := uc.platformDiscountUC.ApplyByCode(ctx, &model.ApplyPlatformDiscountRequest{
			Code:     *req.DiscountCode,
			PlanID:   req.PlanID,
			ClientID: req.ClientID,
			Amount:   originalAmount,
		})
		if err != nil {
			// Kode tidak valid — langsung return error, jangan lanjut
			return nil, err
		}
		appliedDiscountID = &preview.DiscountID
		discountAmount = preview.DiscountAmount
		finalAmount = preview.FinalAmount
	} else {
		// Tidak ada kode — cari diskon otomatis, tidak error jika tidak ada
		preview, err := uc.platformDiscountUC.ApplyAuto(ctx, req.ClientID, req.PlanID, originalAmount)
		if err != nil {
			uc.log.Warn("checkout: gagal cek diskon otomatis", zap.Error(err))
			// Non-fatal: lanjut tanpa diskon
		}
		if preview != nil {
			appliedDiscountID = &preview.DiscountID
			discountAmount = preview.DiscountAmount
			finalAmount = preview.FinalAmount
		}
	}

	if finalAmount.LessThanOrEqual(decimal.Zero) {
		return nil, helper.NewBadRequest("nilai pembayaran tidak valid")
	}

	// 5. Buat external_id unik untuk idempotency
	externalID := fmt.Sprintf("BILLING-%s-%s-%d",
		req.ClientID.String()[:8],
		req.PlanID.String()[:8],
		time.Now().UnixNano(),
	)

	// 6. Create Snap token — pakai finalAmount (sudah dipotong diskon)
	grossAmount, err := midtrans.AmountFromDecimal(finalAmount)
	if err != nil {
		return nil, helper.NewBadRequest("nilai pembayaran tidak valid")
	}

	itemPrice, err := midtrans.AmountFromDecimal(originalAmount)
	if err != nil {
		return nil, helper.NewBadRequest("nilai pembayaran tidak valid")
	}

	snapReq := &midtrans.SnapRequest{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:     externalID,
			GrossAmount: grossAmount,
		},
		CustomerDetails: midtrans.CustomerDetails{
			FirstName: client.Name,
			Email:     client.Owner.Email,
		},
		ItemDetails: []midtrans.ItemDetail{
			{
				ID:       plan.ID.String(),
				Name:     fmt.Sprintf("%s - %s", plan.DisplayName, req.BillingCycle),
				Price:    itemPrice,
				Quantity: 1,
			},
			// ── TAMBAHAN: Tampilkan baris diskon di invoice Midtrans ──
			// Midtrans butuh sum(item_details) == gross_amount
			// Kalau ada diskon, tambahkan sebagai item negatif
			// Ini supaya invoice di halaman Midtrans lebih informatif
		},
		Expiry: &midtrans.SnapExpiry{
			Duration: 24,
			Unit:     "hour",
		},
		Callbacks: &midtrans.SnapCallbacks{
			Finish: fmt.Sprintf("%s/dashboard/billing?status=success", uc.appFrontendURL),
		},
		NotificationURL: fmt.Sprintf("%s/webhooks/midtrans", uc.appBaseURL),
	}

	// Append discount line item jika ada diskon
	if discountAmount.GreaterThan(decimal.Zero) && appliedDiscountID != nil {
		discountPrice, err := midtrans.AmountFromDecimal(discountAmount.Neg())
		if err != nil {
			return nil, helper.NewBadRequest("nilai diskon tidak valid")
		}
		snapReq.ItemDetails = append(snapReq.ItemDetails, midtrans.ItemDetail{
			ID:       "DISCOUNT",
			Name:     "Diskon",
			Price:    discountPrice,
			Quantity: 1,
		})
	}
	// ── END TAMBAHAN ──────────────────────────────────────────

	snapResp, err := uc.midtransClient.CreateSnapToken(ctx, snapReq)
	if err != nil {
		uc.log.Error("client billing: create snap token", zap.Error(err))
		return nil, fmt.Errorf("gagal membuat payment link")
	}

	// 7. Simpan billing dalam DB transaction
	// ── TAMBAHAN: Wrap dalam transaction karena ada 2 operasi (billing + increment diskon) ──
	now := time.Now()
	billing := &entity.ClientBilling{
		ID:             uuid.New(),
		ClientID:       req.ClientID,
		PlanID:         req.PlanID,
		Status:         "pending",
		BillingCycle:   req.BillingCycle,
		Amount:         finalAmount,
		OriginalAmount: originalAmount,
		DiscountID:     appliedDiscountID,
		DiscountAmount: discountAmount,
		ExternalID:     externalID,
		PaymentURL:     snapResp.RedirectURL,
		StartedAt:      now,
		ExpiredAt:      expiredAt,
		IsManual:       false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// ── TAMBAHAN: Gunakan DB transaction untuk atomicity ──────
	err = uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Simpan billing
		if err := uc.billingRepo.Create(ctx, tx, billing); err != nil {
			return err
		}

		// Increment usage diskon — dilakukan di sini (saat billing pending dibuat)
		// bukan saat paid, supaya kuota tidak oversold
		// Jika payment gagal, admin bisa manual reset atau pakai expired cleanup job
		if appliedDiscountID != nil {
			if err := uc.platformDiscountRepo.IncrementUsage(ctx, tx, *appliedDiscountID); err != nil {
				uc.log.Warn("checkout: gagal increment diskon usage", zap.Error(err))
				// Non-fatal untuk sekarang, tapi log — pertimbangkan jadikan fatal
				// jika quota enforcement sangat ketat
			}
		}

		return nil
	})
	if err != nil {
		uc.log.Error("client billing: save billing transaction", zap.Error(err))
		return nil, fmt.Errorf("gagal menyimpan billing")
	}
	// ── END TAMBAHAN ──────────────────────────────────────────

	return &model.CheckoutResponse{
		BillingID:      billing.ID,
		ExternalID:     externalID,
		PaymentURL:     snapResp.RedirectURL,
		SnapToken:      snapResp.Token,
		Amount:         finalAmount,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		ExpiredAt:      expiredAt,
	}, nil
}

func (uc *clientBillingUseCase) HandleWebhook(ctx context.Context, req *model.MidtransWebhookRequest) error {
	notification := &midtrans.WebhookNotification{
		TransactionTime:   req.TransactionTime,
		TransactionStatus: req.TransactionStatus,
		TransactionID:     req.TransactionID,
		OrderID:           req.OrderID,
		GrossAmount:       req.GrossAmount,
		PaymentType:       req.PaymentType,
		SignatureKey:      req.SignatureKey,
		StatusCode:        req.StatusCode,
		FraudStatus:       req.FraudStatus,
	}

	// 1. Verifikasi signature — tolak jika tidak valid
	if !uc.midtransClient.VerifySignature(notification) {
		uc.log.Warn("client billing webhook: invalid signature",
			zap.String("order_id", req.OrderID))
		return fmt.Errorf("invalid signature")
	}

	// 2. Distributed lock per order_id — cegah concurrent webhook processing
	// Midtrans AKAN retry webhook jika tidak dapat response 2xx tepat waktu.
	// Tanpa lock ini, dua webhook untuk order yang sama bisa diproses bersamaan:
	//   Webhook A & B keduanya baca status='pending' → keduanya update → double process.
	// TTL 60 detik: cukup untuk satu webhook selesai diproses.
	// Kunci menggunakan order_id (bukan client_id) agar berbeda per transaksi.
	webhookLockKey := fmt.Sprintf("billing:webhook:lock:%s", req.OrderID)
	if uc.redis != nil {
		ok, redisErr := uc.redis.SetNX(ctx, webhookLockKey, "1", 60*time.Second).Result()
		if redisErr != nil {
			// Redis error — lanjut tanpa lock, atomic DB update jadi safety net
			uc.log.Warn("client billing webhook: gagal acquire webhook lock",
				zap.String("order_id", req.OrderID),
				zap.Error(redisErr))
		} else if !ok {
			// Lock sudah dipegang webhook lain — tolak dengan aman
			uc.log.Info("client billing webhook: duplicate webhook rejected via lock",
				zap.String("order_id", req.OrderID))
			return nil // return nil agar controller tetap kirim 200 ke Midtrans
		}
		// Release lock setelah selesai (gunakan context baru karena ctx mungkin cancelled)
		defer func() {
			delCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			uc.redis.Del(delCtx, webhookLockKey)
		}()
	}

	// 3. Cari billing berdasarkan external_id
	billing, err := uc.billingRepo.FindByExternalID(ctx, uc.db.Gorm, req.OrderID)
	if err != nil {
		return fmt.Errorf("gagal mencari billing")
	}
	if billing == nil {
		uc.log.Warn("client billing webhook: billing not found",
			zap.String("order_id", req.OrderID))
		return fmt.Errorf("billing tidak ditemukan")
	}

	// 4. Early idempotency check (optimistic — bisa race, dikuatkan oleh atomic UPDATE di bawah)
	if billing.Status != "pending" {
		uc.log.Info("client billing webhook: already processed, skipping",
			zap.String("order_id", req.OrderID),
			zap.String("status", billing.Status))
		return nil
	}

	// 5. Tentukan status baru berdasarkan notifikasi Midtrans
	var newStatus string
	switch {
	case notification.IsSuccess():
		newStatus = "active"
	case notification.IsExpired():
		newStatus = "past_due"
	case notification.IsFailed():
		newStatus = "cancelled"
	default:
		// Status tidak dikenal (e.g. 'pending', 'authorize') — skip, tunggu webhook berikutnya
		uc.log.Info("client billing webhook: unhandled transaction status, skipping",
			zap.String("order_id", req.OrderID),
			zap.String("transaction_status", req.TransactionStatus))
		return nil
	}

	// 6. DB Transaction dengan ATOMIC conditional UPDATE
	// Kunci utama idempotency: UPDATE ... WHERE id=? AND status='pending'
	// Jika concurrent webhook sudah memproses → RowsAffected=0 → skip.
	// Ini mengunci di DB level tanpa SELECT FOR UPDATE yang lebih expensive.
	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// Atomic conditional UPDATE: hanya update jika masih 'pending'
		updates := map[string]any{
			"status":     newStatus,
			"updated_at": now,
		}
		if newStatus == "active" {
			updates["paid_at"] = now
		}

		result := tx.WithContext(ctx).
			Model(&entity.ClientBilling{}).
			Where("id = ? AND status = 'pending'", billing.ID).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}

		// RowsAffected = 0: billing sudah diupdate oleh concurrent webhook
		// Ini adalah lapisan keamanan terakhir — tidak perlu retry
		if result.RowsAffected == 0 {
			uc.log.Info("client billing webhook: concurrent duplicate detected via atomic update, skipping",
				zap.String("order_id", req.OrderID),
				zap.String("billing_id", billing.ID.String()))
			return nil
		}

		// Update subscription_tier di client jika payment sukses
		if newStatus == "active" {
			if err := uc.activateClientPlan(ctx, tx, billing); err != nil {
				return err
			}
			uc.log.Info("client billing webhook: payment success, plan activated",
				zap.String("order_id", req.OrderID),
				zap.String("client_id", billing.ClientID.String()),
				zap.String("new_status", newStatus))
		} else if (newStatus == "past_due" || newStatus == "cancelled") && billing.DiscountID != nil {
			if err := uc.platformDiscountRepo.DecrementUsage(ctx, tx, *billing.DiscountID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (uc *clientBillingUseCase) AdminAssignPlan(ctx context.Context, req *model.AdminAssignPlanRequest) (*model.ClientBillingResponse, error) {
	// Layer 2 permission check
	if !rbac.HasPermission(req.CallerPermissions, "billing.manage") {
		return nil, helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	// ── Distributed Lock: cegah concurrent admin assign ke client yang sama ──
	unlockFn, err := uc.acquireBillingLock(ctx, req.ClientID)
	if err != nil {
		return nil, err
	}
	defer unlockFn()

	// Fetch plan & client
	plan, err := uc.planRepo.FindByID(ctx, uc.db.Gorm, req.PlanID)
	if err != nil || plan == nil {
		return nil, helper.NewNotFound("plan tidak ditemukan")
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, req.ClientID)
	if err != nil || client == nil {
		return nil, helper.NewNotFound("client tidak ditemukan")
	}

	// Cancel billing aktif atau pending jika ada
	existing, _ := uc.billingRepo.FindActiveOrPendingByClientID(ctx, uc.db.Gorm, req.ClientID)

	// Hitung amount & expired_at
	amount, expiredAt := uc.calculateBilling(plan, req.BillingCycle)

	startedAt := time.Now()
	if req.StartedAt != nil {
		startedAt = *req.StartedAt
	}

	status := "active"
	if req.IsTrialing {
		status = "trialing"
	}

	now := time.Now()
	billing := &entity.ClientBilling{
		ID:           uuid.New(),
		ClientID:     req.ClientID,
		PlanID:       req.PlanID,
		Status:       status,
		BillingCycle: req.BillingCycle,
		Amount:       amount,
		ExternalID:   fmt.Sprintf("MANUAL-%s-%d", req.ClientID.String()[:8], now.Unix()),
		StartedAt:    startedAt,
		ExpiredAt:    expiredAt,
		PaidAt:       &now,
		IsManual:     true,
		Note:         req.Note,
		CreatedBy:    &req.AdminID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Simpan billing & update client plan dalam satu transaction
	err = uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing != nil {
			wasPending := existing.Status == "pending"
			now := time.Now()
			existing.Status = "cancelled"
			existing.CancelledAt = &now
			existing.UpdatedAt = now
			if err := uc.billingRepo.Update(ctx, tx, existing); err != nil {
				return fmt.Errorf("gagal cancel billing lama")
			}
			if wasPending && existing.DiscountID != nil {
				if err := uc.platformDiscountRepo.DecrementUsage(ctx, tx, *existing.DiscountID); err != nil {
					return err
				}
			}
		}

		if err := uc.billingRepo.Create(ctx, tx, billing); err != nil {
			return err
		}
		return uc.activateClientPlan(ctx, tx, billing)
	})
	if err != nil {
		uc.log.Error("client billing: admin assign plan", zap.Error(err))
		return nil, fmt.Errorf("gagal assign plan")
	}

	// Fetch ulang dengan relasi
	created, _ := uc.billingRepo.FindByID(ctx, uc.db.Gorm, billing.ID)
	resp := toBillingResponse(created)
	return &resp, nil
}

func (uc *clientBillingUseCase) ListBillings(ctx context.Context, req *model.AdminListBillingRequest) ([]model.ClientBillingResponse, int64, error) {
	if !rbac.HasPermission(req.CallerPermissions, "billing.read") {
		return nil, 0, helper.NewForbidden("forbidden: requires 'billing.read' permission")
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 {
		req.Limit = 20
	}
	offset := (req.Page - 1) * req.Limit

	billings, total, err := uc.billingRepo.FindAllPaginated(ctx, uc.db.Gorm, req.ClientID, req.Status, offset, req.Limit)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal mengambil data billing")
	}

	result := make([]model.ClientBillingResponse, len(billings))
	for i, b := range billings {
		result[i] = toBillingResponse(&b)
	}
	return result, total, nil
}

func (uc *clientBillingUseCase) CancelBilling(ctx context.Context, req *model.CancelBillingRequest) error {
	if !rbac.HasPermission(req.CallerPermissions, "billing.manage") {
		return helper.NewForbidden("forbidden: requires 'billing.manage' permission")
	}

	billing, err := uc.billingRepo.FindByID(ctx, uc.db.Gorm, req.BillingID)
	if err != nil || billing == nil {
		return helper.NewNotFound("billing tidak ditemukan")
	}

	if billing.Status == "cancelled" {
		return helper.NewBadRequest("billing sudah dibatalkan")
	}

	return uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wasPending := billing.Status == "pending"
		now := time.Now()
		billing.Status = "cancelled"
		billing.CancelledAt = &now
		billing.CancelledBy = &req.AdminID
		billing.UpdatedBy = &req.AdminID
		billing.Note = req.Reason
		billing.UpdatedAt = now

		if err := uc.billingRepo.Update(ctx, tx, billing); err != nil {
			return err
		}
		if wasPending && billing.DiscountID != nil {
			if err := uc.platformDiscountRepo.DecrementUsage(ctx, tx, *billing.DiscountID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (uc *clientBillingUseCase) GetActiveBilling(ctx context.Context, clientID uuid.UUID) (*model.ClientBillingResponse, error) {
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, clientID)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil billing aktif")
	}
	if billing == nil {
		return nil, nil
	}
	resp := toBillingResponse(billing)
	if uc.featureGateUC != nil {
		usage, err := uc.featureGateUC.GetUsage(ctx, clientID)
		if err != nil {
			uc.log.Warn("failed to get usage for active billing", zap.Error(err))
		} else {
			resp.Usage = usage
		}
	}
	return &resp, nil
}

// ── Private helpers ───────────────────────────────────────────

// calculateBilling menghitung amount dan expired_at berdasarkan billing cycle
func (uc *clientBillingUseCase) calculateBilling(plan *entity.PlatformPlan, cycle string) (decimal.Decimal, time.Time) {
	now := time.Now()
	if cycle == "yearly" {
		return plan.PriceYearly, now.AddDate(1, 0, 0)
	}
	return plan.PriceMonthly, now.AddDate(0, 1, 0)
}

// activateClientPlan update subscription_tier di tabel clients
func (uc *clientBillingUseCase) activateClientPlan(ctx context.Context, tx *gorm.DB, billing *entity.ClientBilling) error {
	// Fetch plan untuk dapat nama tier
	plan, err := uc.planRepo.FindByID(ctx, tx, billing.PlanID)
	if err != nil || plan == nil {
		return fmt.Errorf("plan tidak ditemukan saat aktivasi")
	}

	return tx.WithContext(ctx).
		Model(&entity.Client{}).
		Where("id = ?", billing.ClientID).
		Updates(map[string]any{
			"subscription_tier": plan.Name,
			"updated_at":        time.Now(),
		}).Error
}

func toBillingResponse(b *entity.ClientBilling) model.ClientBillingResponse {
	resp := model.ClientBillingResponse{
		ID:             b.ID,
		Status:         b.Status,
		BillingCycle:   b.BillingCycle,
		OriginalAmount: b.OriginalAmount,
		DiscountAmount: b.DiscountAmount,
		Amount:         b.Amount,
		StartedAt:      b.StartedAt,
		ExpiredAt:      b.ExpiredAt,
		CancelledAt:    b.CancelledAt,
		PaidAt:         b.PaidAt,
		PaymentURL:     b.PaymentURL,
		IsManual:       b.IsManual,
		Note:           b.Note,
		CreatedAt:      b.CreatedAt,
		Plan:           *converter.PlatformPlanToResponse(&b.Plan),
		Client: model.ClientBriefResponse{
			ID:   b.Client.ID,
			Name: b.Client.Name,
			Slug: b.Client.Slug,
		},
	}

	if b.CancelledByAdmin != nil {
		resp.CancelledBy = &model.AdminBriefResponse{
			ID:   b.CancelledByAdmin.ID,
			Name: b.CancelledByAdmin.Name,
		}
	}
	if b.UpdatedByAdmin != nil {
		resp.UpdatedBy = &model.AdminBriefResponse{
			ID:   b.UpdatedByAdmin.ID,
			Name: b.UpdatedByAdmin.Name,
		}
	}

	return resp
}

const billingLockTTL = 30 * time.Second

func (uc *clientBillingUseCase) acquireBillingLock(ctx context.Context, clientID uuid.UUID) (func(), error) {
	lockKey := fmt.Sprintf("billing:lock:%s", clientID.String())
	noop := func() {} // fungsi kosong untuk kasus Redis tidak tersedia

	if uc.redis == nil {
		// Redis tidak diinisialisasi — skip lock, fallback ke DB constraint
		uc.log.Warn("client billing: redis nil, skipping distributed lock",
			zap.String("client_id", clientID.String()))
		return noop, nil
	}

	ok, err := uc.redis.SetNX(ctx, lockKey, "1", billingLockTTL).Result()
	if err != nil {
		uc.log.Warn("client billing: gagal acquire redis lock, melanjutkan tanpa lock",
			zap.String("client_id", clientID.String()),
			zap.Error(err))
		return noop, nil
	}

	if !ok {
		uc.log.Warn("client billing: duplicate checkout detected via redis lock",
			zap.String("client_id", clientID.String()))
		return noop, helper.NewConflict("operasi billing sedang diproses, silakan tunggu dan coba lagi")
	}

	// Berhasil akuisisi lock — kembalikan fungsi untuk release lock
	unlockFn := func() {
		// Gunakan context baru karena context original mungkin sudah cancelled
		delCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if delErr := uc.redis.Del(delCtx, lockKey).Err(); delErr != nil {
			uc.log.Warn("client billing: gagal release redis lock",
				zap.String("lock_key", lockKey),
				zap.Error(delErr))
		}
	}

	return unlockFn, nil
}
