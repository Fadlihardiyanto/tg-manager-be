package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/midtrans"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type IMemberOrderUseCase interface {
	Checkout(ctx context.Context, req *model.MemberCheckoutRequest) (*model.MemberCheckoutResponse, error)
	CheckActiveSubscriptions(ctx context.Context, tgUserID int64, packageID uuid.UUID) (*model.ActiveSubscriptionCheckResult, error)
	HandleWebhook(ctx context.Context, req *model.MidtransWebhookRequest) error
	GetCheckoutDetail(ctx context.Context, externalID string) (*model.MemberCheckoutDetailResponse, error)
	CancelPendingOrder(ctx context.Context, orderID string) error
}

type memberOrderUseCase struct {
	db               *entity.Database
	orderRepo        repository.IOrderRepository
	subRepo          repository.ISubscriptionRepository
	packageRepo      repository.IPackageRepository
	telegramUserRepo repository.ITelegramUserRepository
	clientRepo       repository.IClientRepository
	billingRepo      repository.IClientBillingRepository
	discountRepo     repository.IMemberDiscountRepository
	discountUC       IMemberDiscountUseCase
	outboxRepo       repository.IOutboxRepository
	botRepo          repository.ITelegramBotRepository
	redis            *redis.Client
	log              *zap.Logger
	encryptionKey    string
	midtransBaseURL  string
	midtransSnapURL  string
	appBaseURL       string
	appFrontendURL   string
	paymentLinkMode  string
}

func NewMemberOrderUseCase(
	db *entity.Database,
	orderRepo repository.IOrderRepository,
	subRepo repository.ISubscriptionRepository,
	packageRepo repository.IPackageRepository,
	telegramUserRepo repository.ITelegramUserRepository,
	clientRepo repository.IClientRepository,
	billingRepo repository.IClientBillingRepository,
	discountRepo repository.IMemberDiscountRepository,
	discountUC IMemberDiscountUseCase,
	outboxRepo repository.IOutboxRepository,
	botRepo repository.ITelegramBotRepository,
	redisClient *redis.Client,
	log *zap.Logger,
	encryptionKey string,
	midtransBaseURL string,
	midtransSnapURL string,
	appBaseURL string,
	appFrontendURL string,
	paymentLinkMode string,
) IMemberOrderUseCase {
	return &memberOrderUseCase{
		db:               db,
		orderRepo:        orderRepo,
		subRepo:          subRepo,
		packageRepo:      packageRepo,
		telegramUserRepo: telegramUserRepo,
		clientRepo:       clientRepo,
		billingRepo:      billingRepo,
		discountRepo:     discountRepo,
		discountUC:       discountUC,
		outboxRepo:       outboxRepo,
		botRepo:          botRepo,
		redis:            redisClient,
		log:              log,
		encryptionKey:    encryptionKey,
		midtransBaseURL:  midtransBaseURL,
		midtransSnapURL:  midtransSnapURL,
		appBaseURL:       appBaseURL,
		appFrontendURL:   appFrontendURL,
		paymentLinkMode:  paymentLinkMode,
	}
}

func (uc *memberOrderUseCase) CheckActiveSubscriptions(ctx context.Context, tgUserID int64, packageID uuid.UUID) (*model.ActiveSubscriptionCheckResult, error) {
	result := &model.ActiveSubscriptionCheckResult{}

	// First, find the TelegramUser UUID from tgUserID
	tgUser, err := uc.telegramUserRepo.FindByTelegramID(ctx, uc.db.Gorm, tgUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return result, nil // User doesn't exist yet, so no active subscription
		}
		return nil, err
	}

	// Also find the target package to know its clientID and if it's all-access
	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, packageID)
	if err != nil {
		return nil, err
	}

	// 1. Check for active subscription for this exact package
	samePackageSub, err := uc.subRepo.FindByUserAndPackage(ctx, uc.db.Gorm, tgUser.ID, packageID)
	if err != nil {
		return nil, err
	}
	if samePackageSub != nil {
		result.SamePackage = samePackageSub
	}

	// 2. Check for active ALL ACCESS subscription (only if the selected package is NOT all access)
	if pkg != nil && !pkg.IsAllAccess {
		allAccessSub, err := uc.subRepo.FindActiveAllAccessByUserAndClient(ctx, uc.db.Gorm, tgUser.ID, pkg.ClientID)
		if err != nil {
			return nil, err
		}
		if allAccessSub != nil {
			result.AllAccess = allAccessSub
		}
	}

	return result, nil
}

func (uc *memberOrderUseCase) Checkout(ctx context.Context, req *model.MemberCheckoutRequest) (*model.MemberCheckoutResponse, error) {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member order checkout start", zap.Int64("telegram_user_id", req.TelegramUserID), zap.String("package_id", req.PackageID.String()))

	// 1. Acquire Redis Lock per Telegram User ID to prevent duplicate checkouts
	unlockFn, err := uc.acquireMemberLock(ctx, req.TelegramUserID)
	if err != nil {
		return nil, err
	}
	defer unlockFn()

	// 2. Fetch and validate package
	pkg, err := uc.packageRepo.FindByID(ctx, uc.db.Gorm, req.PackageID)
	if err != nil || pkg == nil {
		return nil, helper.NewNotFound("Paket tidak ditemukan")
	}
	if !pkg.IsActive {
		return nil, helper.NewBadRequest("Paket ini sedang tidak aktif")
	}

	// 2.5. Check Client Quota (Max Members)
	billing, err := uc.billingRepo.FindActiveByClientID(ctx, uc.db.Gorm, pkg.ClientID)
	if err != nil {
		log.Error("member order checkout find client billing failed", zap.Error(err))
		return nil, fmt.Errorf("merchant sedang bermasalah dengan paket billing")
	}
	if billing != nil && billing.Plan.MaxMembers != -1 {
		currentCount, err := uc.subRepo.CountActiveUniqueUsersByClientID(ctx, uc.db.Gorm, pkg.ClientID)
		if err != nil {
			log.Error("member order checkout count members failed", zap.Error(err))
			return nil, fmt.Errorf("gagal menghitung batas member")
		}
		// Allow checkout IF the user already has an active sub for this client (not a new member, just upgrading/extending)
		// We'll check this below during user fetch. If they are a new member and count >= max, we block it.
		if currentCount >= int64(billing.Plan.MaxMembers) {
			// Needs to verify if this specific user already has an active sub to this client.
			// If they don't, it means they are adding a NEW member to the count, which is blocked.
			activeSubs, err := uc.subRepo.FindActiveByTelegramUserID(ctx, uc.db.Gorm, req.TelegramUserID, pkg.ClientID)
			if err != nil {
				log.Warn("member order checkout: failed to load active subs for member-limit check", zap.Int64("telegram_user_id", req.TelegramUserID), zap.Error(err))
			}
			if len(activeSubs) == 0 {
				return nil, helper.NewBadRequest(fmt.Sprintf("Mohon maaf, grup ini telah mencapai batas maksimum pendaftaran member (%d/%d).", currentCount, billing.Plan.MaxMembers))
			}
		}
	}

	// 3. Fetch/Create Telegram User
	tgUser, err := uc.telegramUserRepo.FindByTelegramID(ctx, uc.db.Gorm, req.TelegramUserID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("member order checkout find telegram user failed", zap.Error(err))
		return nil, fmt.Errorf("gagal memproses data user")
	}

	var userUUID uuid.UUID
	var customerName string
	var customerPhone string
	if tgUser == nil {
	userUUID = uuid.New()
	customerName = buildCustomerName(req.FirstName, req.LastName, req.Username, req.TelegramUserID)
	customerPhone = req.Phone

		newUser := &entity.TelegramUser{
			ID:             userUUID,
			TelegramUserID: req.TelegramUserID,
			Username:       req.Username,
			FirstName:      req.FirstName,
			LastName:       req.LastName,
			Phone:          req.Phone,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := uc.telegramUserRepo.Create(ctx, uc.db.Gorm, newUser); err != nil {
			log.Error("member order checkout create telegram user failed", zap.Error(err))
			return nil, fmt.Errorf("gagal mendaftarkan user baru")
		}
	} else {
	userUUID = tgUser.ID
	customerName = buildCustomerName(tgUser.FirstName, tgUser.LastName, tgUser.Username, tgUser.TelegramUserID)
	customerPhone = tgUser.Phone
	}

	// 4. Check for existing pending orders for this package to prevent duplicate links
	existingPending, err := uc.orderRepo.FindPendingOrderByUserAndPackage(ctx, uc.db.Gorm, userUUID, req.PackageID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("member order checkout find pending order failed", zap.Error(err))
		return nil, fmt.Errorf("gagal memverifikasi transaksi aktif")
	}
	if existingPending != nil {
		return nil, helper.NewConflict("Anda sudah memiliki transaksi pending untuk paket ini, silakan selesaikan pembayaran terlebih dahulu")
	}

	// 5. Load and decrypt client's Midtrans credentials
	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, pkg.ClientID)
	if err != nil || client == nil {
		return nil, helper.NewNotFound("Merchant tidak ditemukan")
	}

	midtransClient, err := uc.getMidtransClient(ctx, client)
	if err != nil {
		log.Warn("member order checkout midtrans client init failed", zap.String("client_id", client.ID.String()), zap.Error(err))
		return nil, helper.NewBadRequest(err.Error())
	}

	// 6. Calculate amounts and apply discount (Member Discount)
	originalAmount := pkg.Price
	discountAmount := decimal.Zero
	finalAmount := originalAmount
	var appliedDiscountID *uuid.UUID

	if req.DiscountCode != nil && *req.DiscountCode != "" {
		preview, err := uc.discountUC.ApplyByCode(ctx, &model.ApplyMemberDiscountRequest{
			ClientID:       pkg.ClientID,
			PackageID:      pkg.ID,
			TelegramUserID: userUUID,
			Code:           req.DiscountCode,
			Amount:         originalAmount,
		})
		if err != nil {
			return nil, err
		}
		appliedDiscountID = &preview.DiscountID
		discountAmount = preview.DiscountAmount
		finalAmount = preview.FinalAmount
	} else {
		preview, err := uc.discountUC.ApplyAuto(ctx, &model.ApplyMemberDiscountRequest{
			ClientID:       pkg.ClientID,
			PackageID:      pkg.ID,
			TelegramUserID: userUUID,
			Amount:         originalAmount,
		})
		if err != nil {
			log.Warn("member order checkout auto discount lookup failed", zap.Error(err))
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

	// 7. Generate order details
	externalID := fmt.Sprintf("ORDER-%d-%s-%d",
		req.TelegramUserID,
		pkg.ID.String()[:8],
		time.Now().UnixNano(),
	)

	// Expiry: standard 24 hours
	expiryDuration := 24
	now := time.Now()
	expiredAt := now.Add(time.Duration(expiryDuration) * time.Hour)

	// 8. Create Midtrans Snap request
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
			FirstName: customerName,
			Phone:     customerPhone,
			// Since email isn't mandatory for Telegram members, we default to placeholder if empty
			Email: fmt.Sprintf("member_%d@tgmanager.local", req.TelegramUserID),
		},
		ItemDetails: []midtrans.ItemDetail{
			{
				ID:       pkg.ID.String(),
				Name:     pkg.Name,
				Price:    itemPrice,
				Quantity: 1,
			},
		},
		Expiry: &midtrans.SnapExpiry{
			Duration: expiryDuration,
			Unit:     "hour",
		},
		Callbacks: &midtrans.SnapCallbacks{
			Finish: uc.buildFinishURL(ctx, client.Slug, externalID, pkg),
		},
		NotificationURL: fmt.Sprintf("%s/webhooks/midtrans", uc.appBaseURL),
	}

	if discountAmount.GreaterThan(decimal.Zero) && appliedDiscountID != nil {
		discountPrice, err := midtrans.AmountFromDecimal(discountAmount.Neg())
		if err != nil {
			return nil, helper.NewBadRequest("nilai diskon tidak valid")
		}
		snapReq.ItemDetails = append(snapReq.ItemDetails, midtrans.ItemDetail{
			ID:       "DISCOUNT",
			Name:     "Diskon Promo",
			Price:    discountPrice,
			Quantity: 1,
		})
	}

	snapResp, err := midtransClient.CreateSnapToken(ctx, snapReq)
	if err != nil {
		metrics.MidtransSnapTokenFailed.Inc()
		log.Error("member order checkout create snap token failed",
			zap.String("external_id", externalID),
			zap.String("client_id", pkg.ClientID.String()),
			zap.String("package_id", pkg.ID.String()),
			zap.String("amount", finalAmount.String()),
			zap.Error(err))
		return nil, fmt.Errorf("gagal membuat tautan pembayaran Midtrans")
	}
	metrics.MidtransSnapTokenCreated.Inc()

	// 9. Persist Order in DB
	order := &entity.Order{
		ID:             uuid.New(),
		TelegramUserID: userUUID,
		PackageID:      pkg.ID,
		ExternalID:     externalID,
		Amount:         finalAmount,
		OriginalAmount: originalAmount,
		DiscountID:     appliedDiscountID,
		DiscountAmount: discountAmount,
		Status:         "pending",
		ClientID:       pkg.ClientID,
		PaymentURL:     uc.buildPaymentURL(snapResp.RedirectURL, externalID),
		SnapToken:      snapResp.Token,
		ExpiredAt:      &expiredAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := uc.orderRepo.Create(ctx, uc.db.Gorm, order); err != nil {
		log.Error("member order checkout save failed",
			zap.String("external_id", externalID),
			zap.String("client_id", pkg.ClientID.String()),
			zap.Error(err))
		// Orphaned Snap token: DB save failed after Midtrans created the token.
		// Best-effort cancel so the payment link doesn't dangle; failure is only logged.
		if cancelErr := midtransClient.CancelTransaction(ctx, externalID); cancelErr != nil {
			log.Error("member order checkout failed to cancel orphaned snap token",
				zap.String("external_id", externalID),
				zap.Error(cancelErr))
		} else {
			log.Info("member order checkout cancelled orphaned snap token", zap.String("external_id", externalID))
		}
		return nil, fmt.Errorf("gagal membuat pesanan baru")
	}

	log.Info("member order checkout success", zap.String("order_id", order.ID.String()), zap.String("external_id", externalID))
	return &model.MemberCheckoutResponse{
		OrderID:        externalID,
		DBOrderID:      order.ID,
		ExternalID:     externalID,
		PaymentURL:     uc.buildPaymentURL(snapResp.RedirectURL, externalID),
		SnapToken:      snapResp.Token,
		ClientKey:      midtransClient.ClientKey(),
		PackageName:    pkg.Name,
		DurationDays:   pkg.DurationDays,
		OriginalAmount: originalAmount,
		DiscountAmount: discountAmount,
		Amount:         finalAmount,
	}, nil
}

func (uc *memberOrderUseCase) buildPaymentURL(midtransURL string, externalID string) string {
	if uc.paymentLinkMode == "direct" {
		return midtransURL
	}
	return fmt.Sprintf("%s/payment?order_id=%s", uc.appFrontendURL, externalID)
}

func (uc *memberOrderUseCase) HandleWebhook(ctx context.Context, req *model.MidtransWebhookRequest) error {
	log := logger.FromContext(ctx, uc.log)
	log.Info("member order webhook received", zap.String("order_id", req.OrderID), zap.String("status", req.TransactionStatus))

	// 1. Fetch Order from DB to identify Client
	log.Debug("member order webhook fetching order", zap.String("order_id", req.OrderID))
	order, err := uc.orderRepo.FindByExternalID(ctx, uc.db.Gorm, req.OrderID)
	if err != nil {
		log.Error("member order webhook fetch order failed", zap.Error(err))
		return fmt.Errorf("gagal mengambil data order")
	}
	if order == nil {
		log.Warn("member order webhook order not found", zap.String("order_id", req.OrderID))
		return ErrWebhookNotFound
	}
	log.Debug("member order webhook order found", zap.String("order_id", order.ID.String()), zap.String("client_id", order.ClientID.String()))

	// 2. Fetch Client settings for signature validation keys
	log.Debug("member order webhook fetching client", zap.String("client_id", order.ClientID.String()))
	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, order.ClientID)
	if err != nil || client == nil {
		log.Error("member order webhook client not found", zap.String("client_id", order.ClientID.String()))
		return ErrWebhookNotFound
	}
	log.Debug("member order webhook client found", zap.String("client_id", client.ID.String()), zap.String("client_name", client.Name))

	log.Debug("member order webhook initializing midtrans client", zap.String("client_id", client.ID.String()))
	midtransClient, err := uc.getMidtransClient(ctx, client)
	if err != nil {
		log.Error("member order webhook midtrans client init failed", zap.String("client_id", client.ID.String()), zap.Error(err))
		return err
	}
	log.Debug("member order webhook midtrans client initialized", zap.String("client_id", client.ID.String()))

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
	log.Debug("member order webhook notification parsed", zap.String("order_id", req.OrderID), zap.String("transaction_status", req.TransactionStatus))

	// 3. Verify Signature
	log.Debug("member order webhook verifying signature", zap.String("order_id", req.OrderID))
	if !midtransClient.VerifySignature(notification) {
		log.Warn("member order webhook invalid signature", zap.String("order_id", req.OrderID))
		return ErrWebhookInvalidSignature
	}

	// 4. Acquire Webhook Lock to prevent concurrent updates
	webhookLockKey := fmt.Sprintf("member:webhook:lock:%s", req.OrderID)
	if uc.redis != nil {
		log.Debug("member order webhook acquiring lock", zap.String("order_id", req.OrderID))
		ok, redisErr := uc.redis.SetNX(ctx, webhookLockKey, "1", 60*time.Second).Result()
		if redisErr != nil {
			log.Warn("member order webhook failed to acquire lock, fallback to DB lock", zap.Error(redisErr))
		} else if !ok {
			log.Info("member order webhook concurrent request rejected", zap.String("order_id", req.OrderID))
			return nil // returning 200 OK so Midtrans stops retrying
		}
		defer func() {
			delCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			uc.redis.Del(delCtx, webhookLockKey)
		}()
	}

	// 5. Early Idempotency Check
	if order.Status != "pending" {
		log.Info("member order webhook already processed, skipping", zap.String("order_id", req.OrderID), zap.String("status", order.Status))
		return nil
	}

	// 6. Translate Midtrans Status
	log.Debug("member order webhook translating status", zap.String("order_id", req.OrderID), zap.String("transaction_status", req.TransactionStatus))
	newStatus, handled := mapMemberOrderWebhookStatus(notification)
	if !handled {
		log.Info("member order webhook unhandled status, skipping", zap.String("order_id", req.OrderID), zap.String("transaction_status", req.TransactionStatus))
		return nil
	}

	// 7. Process Order Update & Subscription activation in DB transaction
	log.Debug("member order webhook processing order update", zap.String("order_id", req.OrderID), zap.String("new_status", newStatus))
	err = uc.db.Gorm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		log.Debug("member order webhook fetching order for update", zap.String("order_id", req.OrderID))
		now := time.Now()

		// Update order atomically ONLY IF status is 'pending'
		updates := map[string]any{
			"status":           newStatus,
			"updated_at":       now,
			"raw_notification": datatypes.JSON([]byte(req.RawNotification)),
		}
		if newStatus == "paid" {
			updates["paid_at"] = &now
			updates["payment_method"] = req.PaymentType
		}

		log.Debug("member order webhook updating order status", zap.String("order_id", order.ID.String()), zap.String("new_status", newStatus))
		rowsAffected, err := uc.orderRepo.AtomicUpdateStatus(ctx, tx, order.ID, "pending", updates)
		if err != nil {
			log.Error("member order webhook failed to update order status", zap.String("order_id", order.ID.String()), zap.Error(err))
			return err
		}

		order.Status = newStatus
		order.UpdatedAt = now
		if newStatus == "paid" {
			order.PaidAt = &now
			order.PaymentMethod = req.PaymentType
		}

		// Concurrent webhook processed it first
		if rowsAffected == 0 {
			log.Info("member order webhook concurrent update detected, skipping", zap.String("order_id", req.OrderID))
			return nil
		}

		if (newStatus == "expired" || newStatus == "failed") && order.DiscountID != nil {
			if err := uc.discountUC.RollbackUsage(ctx, tx, order.ID, *order.DiscountID); err != nil {
				log.Error("member order webhook rollback usage failed", zap.Error(err))
				return err
			}
			log.Info("member order webhook discount usage rolled back", zap.String("order_id", req.OrderID), zap.String("discount_id", order.DiscountID.String()))
		}

		if newStatus == "paid" && order.DiscountID != nil {
			log.Info("member order webhook recording discount usage", zap.String("order_id", req.OrderID), zap.String("discount_id", order.DiscountID.String()))
			usage := &entity.MemberDiscountUsage{
				ID:             uuid.New(),
				DiscountID:     *order.DiscountID,
				TelegramUserID: order.TelegramUserID,
				OrderID:        order.ID,
				DiscountAmount: order.DiscountAmount,
				UsedAt:         now,
			}
			if err := uc.discountRepo.CreateUsage(ctx, tx, usage); err != nil {
				log.Error("member order webhook record discount usage failed", zap.Error(err))
				return fmt.Errorf("gagal menyimpan usage diskon")
			}
			if err := uc.discountRepo.IncrementUsage(ctx, tx, *order.DiscountID); err != nil {
				log.Error("member order webhook increment discount usage failed", zap.Error(err))
				return err
			}
		}

		// Retrieve package to get duration details
		log.Debug("member order webhook fetching package for subscription details", zap.String("package_id", order.PackageID.String()))
		pkg, err := uc.packageRepo.FindByID(ctx, tx, order.PackageID)
		if err != nil || pkg == nil {
			return fmt.Errorf("paket tidak ditemukan")
		}

		log.Debug("member order webhook processing subscription update", zap.String("order_id", req.OrderID), zap.String("package_id", pkg.ID.String()), zap.Int("duration_days", pkg.DurationDays))
		if newStatus == "paid" {
			log.Info("member order webhook activating subscription", zap.String("order_id", req.OrderID), zap.String("package_id", pkg.ID.String()))
			// Find active subscription for user and package (for stacking perpanjangan)
			existingSub, err := uc.subRepo.FindByUserAndPackage(ctx, tx, order.TelegramUserID, order.PackageID)
			if err != nil {
				return err
			}

			var subID uuid.UUID
			var activatedAt time.Time
			var expiredAt time.Time

			if existingSub != nil {
				log.Debug("member order webhook found existing subscription, stacking", zap.String("sub_id", existingSub.ID.String()))
				subID, activatedAt, expiredAt = calculateMemberSubscriptionDates(existingSub, pkg.DurationDays, now)
				existingSub.ExpiredAt = expiredAt
				existingSub.OrderID = &order.ID
				existingSub.UpdatedAt = now
				if err := uc.subRepo.Update(ctx, tx, existingSub); err != nil {
					return err
				}
			} else {
				log.Debug("member order webhook no existing subscription, creating new one")
				subID, activatedAt, expiredAt = calculateMemberSubscriptionDates(nil, pkg.DurationDays, now)
				newSub := &entity.Subscription{
					ID:               subID,
					TelegramUserID:   order.TelegramUserID,
					PackageID:        order.PackageID,
					ClientID:         order.ClientID,
					OrderID:          &order.ID,
					Status:           "active",
					ActivatedAt:      activatedAt,
					ExpiredAt:        expiredAt,
					AutoRenew:        false,
					GracePeriodHours: 0,
					LastCheckedAt:    now,
					CreatedAt:        now,
					UpdatedAt:        now,
				}

				log.Info("member order webhook creating new subscription", zap.String("sub_id", subID.String()), zap.Time("expiry", expiredAt))
				if err := uc.subRepo.Create(ctx, tx, newSub); err != nil {
					return err
				}
				log.Info("member order webhook subscription activated", zap.String("sub_id", subID.String()), zap.String("telegram_user_id", order.TelegramUserID.String()), zap.String("package_id", order.PackageID.String()))
			}

			// Link order to subscription
			order.SubscriptionID = &subID
			log.Debug("member order webhook linking order to subscription", zap.String("order_id", order.ID.String()), zap.String("sub_id", subID.String()))
			if err := uc.orderRepo.Update(ctx, tx, order); err != nil {
				return err
			}

			// Outbox Pattern: Publish subscription activated event reliably
			// Log event to outbox table
			var tgUser entity.TelegramUser
			var tgID int64
			if err := uc.telegramUserRepo.FindById(ctx, tx, &tgUser, order.TelegramUserID); err == nil {
				tgID = tgUser.TelegramUserID
			}

			highPriority := false
			billing, err := uc.billingRepo.FindActiveByClientID(ctx, tx, order.ClientID)
			if err != nil {
				log.Warn("member order webhook: failed to load billing for high-priority flag", zap.String("client_id", order.ClientID.String()), zap.Error(err))
			} else if billing != nil {
				highPriority = billing.Plan.AllowHighPriority
			}

			eventPayload := map[string]any{
				"subscription_id":  subID.String(),
				"telegram_user_id": tgID,
				"user_uuid":        order.TelegramUserID.String(),
				"package_id":       order.PackageID.String(),
				"client_id":        order.ClientID.String(),
				"order_id":         order.ID.String(),
				"amount":           order.Amount,
				"activated_at":     activatedAt.Format(time.RFC3339),
				"expired_at":       expiredAt.Format(time.RFC3339),
				"high_priority":    highPriority,
			}

			payloadBytes, err := json.Marshal(eventPayload)
			if err != nil {
				return err
			}

			outbox := &entity.Outbox{
				ID:            uuid.New(),
				AggregateType: "subscription",
				AggregateID:   subID,
				EventType:     "subscription.activated",
				Payload:       datatypes.JSON(payloadBytes),
				Status:        "pending",
				RetryCount:    0,
				MaxRetries:    3,
				ProcessAfter:  now,
				CreatedAt:     now,
				UpdatedAt:     now,
			}

			log.Info("member order webhook writing outbox event", zap.String("outbox_id", outbox.ID.String()), zap.String("event_type", outbox.EventType))
			if err := uc.outboxRepo.Create(ctx, tx, outbox); err != nil {
				return err
			}
			log.Info("member order webhook outbox event written", zap.String("outbox_id", outbox.ID.String()))
		}

		return nil
	})
	if err != nil {
		log.Error("member order webhook transaction failed", zap.Error(err))
		return err
	}

	return nil
}

func (uc *memberOrderUseCase) buildFinishURL(ctx context.Context, slug, externalID string, pkg *entity.Package) string {
	base := fmt.Sprintf("%s/checkout/success?slug=%s&order_id=%s", uc.appFrontendURL, slug, externalID)
	if len(pkg.Groups) > 0 {
		bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, pkg.Groups[0].BotUUID)
		if err != nil {
			uc.log.Warn("member order: failed to load bot for finish URL", zap.String("bot_id", pkg.Groups[0].BotUUID.String()), zap.Error(err))
		} else if bot != nil && bot.Username != "" {
			base += "&bot=" + bot.Username
		}
	}
	return base
}

func (uc *memberOrderUseCase) GetCheckoutDetail(ctx context.Context, externalID string) (*model.MemberCheckoutDetailResponse, error) {
	order, err := uc.orderRepo.FindByExternalIDWithPackage(ctx, uc.db.Gorm, externalID)
	if err != nil {
		return nil, fmt.Errorf("gagal mencari order: %w", err)
	}
	if order == nil {
		return nil, helper.NewNotFound("Order tidak ditemukan")
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, order.ClientID)
	if err != nil || client == nil {
		return nil, helper.NewNotFound("Merchant tidak ditemukan")
	}

	midtransClient, err := uc.getMidtransClient(ctx, client)
	if err != nil {
		uc.log.Warn("GetCheckoutDetail: midtrans client init failed", zap.String("client_id", client.ID.String()), zap.Error(err))
		return nil, helper.NewBadRequest(err.Error())
	}

	var botUsername string
	if len(order.Package.Groups) > 0 {
		bot, err := uc.botRepo.FindByID(ctx, uc.db.Gorm, order.Package.Groups[0].BotUUID)
		if err != nil {
			uc.log.Warn("GetCheckoutDetail: failed to load bot", zap.String("bot_id", order.Package.Groups[0].BotUUID.String()), zap.Error(err))
		} else if bot != nil && bot.Username != "" {
			botUsername = bot.Username
		}
	}
	if botUsername == "" {
		fallback, err := uc.botRepo.FindFirstByClientID(ctx, uc.db.Gorm, order.ClientID)
		if err != nil {
			uc.log.Warn("GetCheckoutDetail: failed to load fallback bot", zap.String("client_id", order.ClientID.String()), zap.Error(err))
		} else if fallback != nil && fallback.Username != "" {
			botUsername = fallback.Username
		}
	}

	return &model.MemberCheckoutDetailResponse{
		OrderID:     order.ExternalID,
		SnapToken:   order.SnapToken,
		PaymentURL:  order.PaymentURL,
		ClientKey:   midtransClient.ClientKey(),
		Bot:         botUsername,
		Slug:        client.Slug,
		Amount:      order.Amount,
		PackageName: order.Package.Name,
		Status:      order.Status,
	}, nil
}

func (uc *memberOrderUseCase) CancelPendingOrder(ctx context.Context, externalID string) error {
	order, err := uc.orderRepo.FindByExternalID(ctx, uc.db.Gorm, externalID)
	if err != nil {
		return fmt.Errorf("gagal mencari order: %w", err)
	}
	if order == nil {
		return helper.NewNotFound("Order tidak ditemukan")
	}
	if order.Status != "pending" {
		return helper.NewBadRequest("Order tidak dalam status pending")
	}

	client, err := uc.clientRepo.FindByID(ctx, uc.db.Gorm, order.ClientID)
	if err != nil || client == nil {
		return helper.NewNotFound("Merchant tidak ditemukan")
	}

	midtransClient, err := uc.getMidtransClient(ctx, client)
	if err != nil {
		uc.log.Warn("CancelPendingOrder: midtrans client init failed", zap.Error(err))
		return helper.NewBadRequest(err.Error())
	}

	if err := midtransClient.CancelTransaction(ctx, order.ExternalID); err != nil {
		uc.log.Warn("CancelPendingOrder: midtrans cancel failed", zap.Error(err), zap.String("external_id", order.ExternalID))
	}

	now := time.Now()
	order.Status = "failed"
	order.UpdatedAt = now

	if err := uc.orderRepo.Update(ctx, uc.db.Gorm, order); err != nil {
		return fmt.Errorf("gagal update status order: %w", err)
	}

	if order.DiscountID != nil {
		if err := uc.discountUC.RollbackUsage(ctx, uc.db.Gorm, order.ID, *order.DiscountID); err != nil {
			uc.log.Error("CancelPendingOrder: failed to rollback discount", zap.Error(err), zap.String("discount_id", order.DiscountID.String()))
		}
	}

	return nil
}

// ── Private Helpers ──────────────────────────────────────────

func (uc *memberOrderUseCase) getMidtransClient(ctx context.Context, client *entity.Client) (*midtrans.Client, error) {
	var serverKeyEnc, clientKeyEnc *string
	if client.MidtransIsSandbox {
		serverKeyEnc = client.MidtransSandboxServerKey
		clientKeyEnc = client.MidtransSandboxClientKey
	} else {
		serverKeyEnc = client.MidtransProductionServerKey
		clientKeyEnc = client.MidtransProductionClientKey
	}

	if serverKeyEnc == nil || *serverKeyEnc == "" ||
		clientKeyEnc == nil || *clientKeyEnc == "" {
		return nil, fmt.Errorf("merchant ini belum mengaktifkan pembayaran Midtrans")
	}

	serverKey, err := crypto.Decrypt(*serverKeyEnc, uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca server key Midtrans client: %v", err)
	}

	clientKey, err := crypto.Decrypt(*clientKeyEnc, uc.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca client key Midtrans client")
	}

	baseURL, snapURL := midtrans.EnvironmentURLs(client.MidtransIsSandbox)

	return midtrans.NewClient(midtrans.Config{
		ServerKey: serverKey,
		ClientKey: clientKey,
		BaseURL:   baseURL,
		SnapURL:   snapURL,
	}, uc.log), nil
}

func (uc *memberOrderUseCase) acquireMemberLock(ctx context.Context, telegramUserID int64) (func(), error) {
	lockKey := fmt.Sprintf("member:lock:%d", telegramUserID)
	noop := func() {}

	if uc.redis == nil {
		uc.log.Warn("member order usecase: redis nil, skipping distributed lock", zap.Int64("telegram_user_id", telegramUserID))
		return noop, nil
	}

	ok, err := uc.redis.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
	if err != nil {
		uc.log.Warn("member order usecase: failed to acquire redis lock, skipping", zap.Int64("telegram_user_id", telegramUserID), zap.Error(err))
		return noop, nil
	}

	if !ok {
		return noop, helper.NewConflict("Transaksi Anda sedang diproses, silakan tunggu beberapa saat")
	}

	unlockFn := func() {
		delCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = uc.redis.Del(delCtx, lockKey)
	}

	return unlockFn, nil
}

func buildCustomerName(firstName, lastName, username string, telegramUserID int64) string {
	name := firstName
	if lastName != "" {
		if firstName != "" {
			name = firstName + " " + lastName
		} else {
			name = lastName
		}
	}
	if name == "" {
		name = username
	}
	if name == "" {
		name = fmt.Sprintf("User-%d", telegramUserID)
	}
	return name
}

func mapMemberOrderWebhookStatus(n *midtrans.WebhookNotification) (string, bool) {
	switch {
	case n.IsSuccess():
		return "paid", true
	case n.IsExpired():
		return "expired", true
	case n.IsFailed():
		return "failed", true
	default:
		return "", false
	}
}

func calculateMemberSubscriptionDates(existingSub *entity.Subscription, durationDays int, now time.Time) (uuid.UUID, time.Time, time.Time) {
	if existingSub != nil {
		return existingSub.ID, existingSub.ActivatedAt, existingSub.ExpiredAt.AddDate(0, 0, durationDays)
	}
	subID := uuid.New()
	return subID, now, now.AddDate(0, 0, durationDays)
}
