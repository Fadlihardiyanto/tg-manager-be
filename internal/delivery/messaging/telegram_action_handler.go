package messaging

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/pdf"
	pkg_s3 "github.com/Fadlihardiyanto/telegram-management-app/pkg/s3"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TelegramActionHandler struct {
	db              *gorm.DB
	packageRepo     repository.IPackageRepository
	botRepo         repository.ITelegramBotRepository
	groupRepo       repository.ITelegramGroupRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	logger          *zap.Logger
	orderRepo       repository.IOrderRepository
	clientRepo      repository.IClientRepository
	tgUserRepo      repository.ITelegramUserRepository
	pdfClient       *pdf.Client
	s3Client        *pkg_s3.Client
}

func NewTelegramActionHandler(
	db *gorm.DB,
	packageRepo repository.IPackageRepository,
	botRepo repository.ITelegramBotRepository,
	groupRepo repository.ITelegramGroupRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	logger *zap.Logger,
	orderRepo repository.IOrderRepository,
	clientRepo repository.IClientRepository,
	tgUserRepo repository.ITelegramUserRepository,
	pdfClient *pdf.Client,
	s3Client *pkg_s3.Client,
) *TelegramActionHandler {
	return &TelegramActionHandler{
		db:              db,
		packageRepo:     packageRepo,
		botRepo:         botRepo,
		groupRepo:       groupRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		logger:          logger,
		orderRepo:       orderRepo,
		clientRepo:      clientRepo,
		tgUserRepo:      tgUserRepo,
		pdfClient:       pdfClient,
		s3Client:        s3Client,
	}
}

type SubscriptionActivatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	TelegramUserID int64  `json:"telegram_user_id"`
	PackageID      string `json:"package_id"`
	ClientID       string `json:"client_id"`
	OrderID        string `json:"order_id"`
}

func (h *TelegramActionHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	// 1. Parse Payload
	var payload SubscriptionActivatedPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("telegram action handler: failed to unmarshal payload", append(logFields, zap.Error(err))...)
		return nil
	}

	h.logger.Info("telegram action handler: processing subscription.activated", append(logFields, zap.String("subscription_id", payload.SubscriptionID))...)

	// 2. Fetch Package & associated Groups
	pkgID, err := uuid.Parse(payload.PackageID)
	if err != nil {
		h.logger.Error("telegram action handler: invalid package id", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	pkg, err := h.packageRepo.FindByID(ctx, h.db, pkgID)
	if err != nil {
		return fmt.Errorf("package lookup failed: %w", err)
	}
	if pkg == nil {
		h.logger.Error("telegram action handler: package not found", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	var targetGroups []entity.Group
	if pkg.IsAllAccess {
		groups, err := h.groupRepo.FindByClientID(ctx, h.db, pkg.ClientID, 1, 10000000000000000)
		if err != nil {
			return fmt.Errorf("group lookup failed: %w", err)
		}
		targetGroups = groups
	} else {
		targetGroups = pkg.Groups
	}

	if len(targetGroups) == 0 {
		h.logger.Info("telegram action handler: no groups to invite, skipping", append(logFields, zap.String("package_id", payload.PackageID))...)
		return nil
	}

	// 3. Fetch Bot for this Client
	clientID, err := uuid.Parse(payload.ClientID)
	if err != nil {
		h.logger.Error("telegram action handler: invalid client id", append(logFields, zap.String("client_id", payload.ClientID))...)
		return nil
	}

	bot, err := h.botRepo.FindFirstByClientID(ctx, h.db, clientID)
	if err != nil {
		return fmt.Errorf("bot lookup failed: %w", err)
	}
	if bot == nil {
		h.logger.Error("telegram action handler: no bot found for client", append(logFields, zap.String("client_id", payload.ClientID))...)
		return nil
	}
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt bot token: %w", err)
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		return fmt.Errorf("failed to init telegram client: %w", err)
	}

	// 4. Generate Invite Links for each group
	var inviteLinks []string
	var failedGroups []string
	for _, group := range targetGroups {
		// Limit to 1 use, so it can't be shared
		subIDPrefix := payload.SubscriptionID
		if len(subIDPrefix) > 8 {
			subIDPrefix = subIDPrefix[:8]
		}
		linkName := fmt.Sprintf("Sub-%s", subIDPrefix)
		inviteLink, err := botClient.CreateChatInviteLink(ctx, group.TelegramChatID, linkName, true)
		if err != nil {
			h.logger.Error("telegram action handler: failed to create invite link", append(logFields, zap.String("group", group.Name), zap.Error(err))...)
			failedGroups = append(failedGroups, group.Name)
			continue
		}
		inviteLinks = append(inviteLinks, fmt.Sprintf("• %s: %s", group.Name, inviteLink))
	}

	if len(inviteLinks) == 0 {
		err := fmt.Errorf("telegram action handler: failed to generate any invite links")
		h.logger.Error("telegram action handler: failed to generate any invite links",
			append(logFields,
				zap.String("subscription_id", payload.SubscriptionID),
				zap.String("package_id", payload.PackageID),
				zap.Error(err),
			)...,
		)
		return err
	}

	// 5. Generate Receipt PDF and Upload to S3 (non-blocking)
	var receiptPDFBytes []byte
	var receiptURL string
	receiptPDFBytes, receiptURL, _ = h.generateReceipt(ctx, payload, pkg, logFields)

	// 6. Fetch Subscription for Expiration Date
	subID, err := uuid.Parse(payload.SubscriptionID)
	if err != nil {
		h.logger.Error("telegram action handler: invalid subscription id", append(logFields, zap.String("subscription_id", payload.SubscriptionID))...)
		return nil
	}

	var sub entity.Subscription
	if err := h.db.WithContext(ctx).First(&sub, "id = ?", subID).Error; err != nil {
		h.logger.Error("telegram action handler: failed to fetch subscription", append(logFields, zap.Error(err))...)
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	expiredStr := sub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")

	// 7. Send DM to User FIRST (welcome text + invite links + receipt URL link)
	message := fmt.Sprintf("🎉 Pembayaran Berhasil!\n\nTerima kasih telah berlangganan paket <b>%s</b>.\nPaket Anda aktif sampai: <b>%s</b>\n\nBerikut adalah link khusus untuk masuk ke grup:\n%s\n\n<i>Link ini hanya berlaku untuk 1 kali pakai.</i>", pkg.Name, expiredStr, strings.Join(inviteLinks, "\n"))
	if receiptURL != "" {
		message = fmt.Sprintf("%s\n\n📄 <a href=\"%s\">Download Kwitansi Pembayaran</a>", message, receiptURL)
	}
	if len(failedGroups) > 0 {
		message = fmt.Sprintf("%s\n\n⚠️ Gagal membuat link untuk: %s. Silakan hubungi admin.", message, strings.Join(failedGroups, ", "))
	}

	if err := botClient.SendMessage(ctx, payload.TelegramUserID, message); err != nil {
		h.logger.Error("telegram action handler: failed to send dm", append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Error(err))...)
		return fmt.Errorf("failed to send dm: %w", err)
	}

	// 8. Send PDF document AFTER text DM (so user reads the welcome message first)
	if receiptPDFBytes != nil {
		doc := tgbotapi.NewDocument(payload.TelegramUserID, tgbotapi.FileBytes{
			Name:  "kwitansi.pdf",
			Bytes: receiptPDFBytes,
		})
		doc.Caption = fmt.Sprintf("Kwitansi pembayaran paket %s", pkg.Name)
		if _, err := botClient.SendDocument(ctx, doc); err != nil {
			h.logger.Warn("telegram action handler: failed to send receipt document", append(logFields, zap.Error(err))...)
			// non-fatal — text DM with receipt URL link already delivered
		}
	}

	h.logger.Info("telegram action handler: successfully processed subscription.activated", append(logFields, zap.String("subscription_id", payload.SubscriptionID))...)
	return nil
}

// generateReceipt builds a PDF receipt, uploads it to S3, and saves the URL on the order.
// Returns (pdfBytes, receiptURL, error). pdfBytes is nil when skipped (idempotent retry).
// Errors are logged as warnings and do not block the overall flow.
func (h *TelegramActionHandler) generateReceipt(
	ctx context.Context,
	payload SubscriptionActivatedPayload,
	pkg *entity.Package,
	logFields []zap.Field,
) ([]byte, string, error) {
	// 1. Fetch Order by order_id from payload (not subscription_id, to handle renewals correctly)
	var order entity.Order
	if err := h.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", payload.OrderID).
		First(&order).Error; err != nil {
		h.logger.Warn("receipt: order not found", append(logFields, zap.String("order_id", payload.OrderID), zap.Error(err))...)
		return nil, "", fmt.Errorf("order not found: %w", err)
	}

	// 2. Idempotency check — if receipt already generated on a previous attempt, skip regeneration.
	if order.ReceiptURL != "" {
		h.logger.Info("receipt: already generated, skipping regeneration",
			append(logFields, zap.String("url", order.ReceiptURL))...)
		return nil, order.ReceiptURL, nil
	}

	// 3. Fetch Client
	client, err := h.clientRepo.FindByID(ctx, h.db, order.ClientID)
	if err != nil {
		h.logger.Warn("receipt: client lookup failed", append(logFields, zap.Error(err))...)
		return nil, "", fmt.Errorf("client lookup failed: %w", err)
	}

	// 4. Fetch TelegramUser
	tgUser, err := h.tgUserRepo.FindByTelegramID(ctx, h.db, payload.TelegramUserID)
	if err != nil {
		h.logger.Warn("receipt: telegram user lookup failed", append(logFields, zap.Error(err))...)
		return nil, "", fmt.Errorf("telegram user lookup failed: %w", err)
	}

	// 5. Build ReceiptData
	customerName := tgUser.FirstName
	if tgUser.LastName != "" {
		customerName = customerName + " " + tgUser.LastName
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	paidAt := time.Now()
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}

	receiptData := &pdf.ReceiptData{
		MerchantName:    client.Name,
		OrderID:         order.ID.String(),
		TransactionID:   order.ExternalID,
		TransactionTime: paidAt.In(loc).Format("02 Januari 2006, 15:04 WIB"),
		PaymentMethod:   order.PaymentMethod,
		Status:          "paid",
		PaidAt:          paidAt,
		CustomerName:    customerName,
		CustomerTelegram: func() string {
			if tgUser.Username != "" {
				return "@" + tgUser.Username
			}
			return ""
		}(),
		CustomerPhone: tgUser.Phone,
		Items: []pdf.ReceiptItem{
			{
				Name:     pkg.Name,
				Duration: fmt.Sprintf("%d Hari", pkg.DurationDays),
				Qty:      1,
				Price:    pkg.Price,
				Subtotal: pkg.Price,
			},
		},
		Subtotal:       order.OriginalAmount,
		DiscountAmount: order.DiscountAmount,
		TotalPaid:      order.Amount,
		CurrencyCode:   "IDR",
		ReceiptNumber:  order.ExternalID,
		Notes:          fmt.Sprintf("Paket berlangganan %s — %d hari akses.", pkg.Name, pkg.DurationDays),
	}

	// 6. Generate PDF
	pdfBytes, err := h.pdfClient.GenerateReceipt(receiptData)
	if err != nil {
		h.logger.Warn("receipt: pdf generation failed", append(logFields, zap.Error(err))...)
		return nil, "", fmt.Errorf("pdf generation failed: %w", err)
	}

	// 7. Upload to S3
	if h.s3Client == nil {
		h.logger.Warn("receipt: s3 client not configured, skipping upload", logFields...)
		return nil, "", fmt.Errorf("s3 client not configured")
	}

	s3Key := fmt.Sprintf("receipts/%s/%s.pdf", order.ClientID.String(), order.ID.String())
	uploadOut, err := h.s3Client.Upload(ctx, &pkg_s3.UploadInput{
		Key:         s3Key,
		Body:        bytes.NewReader(pdfBytes),
		ContentType: "application/pdf",
		Size:        int64(len(pdfBytes)),
	})
	if err != nil {
		h.logger.Warn("receipt: s3 upload failed", append(logFields, zap.Error(err))...)
		return nil, "", fmt.Errorf("s3 upload failed: %w", err)
	}

	receiptURL := uploadOut.PublicURL

	// 8. Save receipt_url on Order
	order.ReceiptURL = receiptURL
	if err := h.orderRepo.Update(ctx, h.db, &order); err != nil {
		h.logger.Warn("receipt: failed to save receipt_url on order", append(logFields, zap.Error(err))...)
		// non-fatal, continue
	}

	h.logger.Info("receipt: generated and uploaded", append(logFields, zap.String("receipt_url", receiptURL))...)
	return pdfBytes, receiptURL, nil
}
