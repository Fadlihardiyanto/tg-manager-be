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
	IsResend       bool   `json:"is_resend"`
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

	// 3. Group target groups by bot_id — each group uses its own bot.
	groupsByBot := make(map[uuid.UUID][]entity.Group)
	for _, group := range targetGroups {
		groupsByBot[group.BotUUID] = append(groupsByBot[group.BotUUID], group)
	}

	// 4. Generate Invite Links for each group using the correct bot.
	var inviteLinks []string
	var failedGroups []string
	var dmClient telegram.BotClient
	subIDPrefix := payload.SubscriptionID
	if len(subIDPrefix) > 8 {
		subIDPrefix = subIDPrefix[:8]
	}

	for botID, groups := range groupsByBot {
		bot, err := h.botRepo.FindByID(ctx, h.db, botID)
		if err != nil {
			h.logger.Error("telegram action handler: failed to find bot",
				append(logFields, zap.String("bot_id", botID.String()), zap.Error(err))...)
			for _, group := range groups {
				failedGroups = append(failedGroups, group.Name)
			}
			continue
		}

		token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
		if err != nil {
			h.logger.Error("telegram action handler: failed to decrypt bot token",
				append(logFields, zap.String("bot_id", botID.String()), zap.Error(err))...)
			for _, group := range groups {
				failedGroups = append(failedGroups, group.Name)
			}
			continue
		}

		botClient, err := h.telegramFactory.NewClient(token)
		if err != nil {
			h.logger.Error("telegram action handler: failed to init telegram client",
				append(logFields, zap.String("bot_id", botID.String()), zap.Error(err))...)
			for _, group := range groups {
				failedGroups = append(failedGroups, group.Name)
			}
			continue
		}

		if dmClient == nil {
			dmClient = botClient
		}

		for _, group := range groups {
			linkName := fmt.Sprintf("Sub-%s", subIDPrefix)
			inviteLink, err := botClient.CreateChatInviteLink(ctx, group.TelegramChatID, linkName, true)
			if err != nil {
				h.logger.Error("telegram action handler: failed to create invite link",
					append(logFields, zap.String("group", group.Name), zap.Error(err))...)
				failedGroups = append(failedGroups, group.Name)
				continue
			}
			inviteLinks = append(inviteLinks, fmt.Sprintf("• %s: %s", group.Name, inviteLink))
		}
	}

	if len(inviteLinks) == 0 {
		h.logger.Warn("telegram action handler: no invite links generated, user will be notified via DM",
			append(logFields,
				zap.String("subscription_id", payload.SubscriptionID),
				zap.String("package_id", payload.PackageID),
			)...,
		)
		// Continue to send DM — user gets a message explaining the issue (no DLQ)
	}

	// 5. Generate Receipt PDF and Upload to S3 (skip for resend link — no order data)
	var receiptPDFBytes []byte
	var receiptURL string
	if !payload.IsResend {
		receiptPDFBytes, receiptURL, _ = h.generateReceipt(ctx, payload, pkg, logFields)
	}

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

	// 7. Build DM message
	var message string
	if payload.IsResend {
		if len(inviteLinks) == 0 {
			message = fmt.Sprintf("⚠️ Mohon maaf, kami mengalami kendala teknis saat membuat link akses untuk paket <b>%s</b>.\nSilakan hubungi Admin untuk bantuan lebih lanjut.", pkg.Name)
		} else {
			message = fmt.Sprintf("👋 Halo!\n\nBerikut adalah link akses ulang Anda untuk masuk ke grup paket <b>%s</b>.\n\n%s\n\n<i>Link ini hanya berlaku untuk 1 kali pakai.</i>", pkg.Name, strings.Join(inviteLinks, "\n"))
		}
	} else {
		if len(inviteLinks) == 0 {
			message = fmt.Sprintf("🎉 Pembayaran Berhasil!\n\nTerima kasih telah berlangganan paket <b>%s</b>.\nPaket Anda aktif sampai: <b>%s</b>\n\n⚠️ Mohon maaf, kami mengalami kendala teknis saat membuat link akses grup.\nSilakan hubungi Admin untuk bantuan lebih lanjut.", pkg.Name, expiredStr)
		} else {
			message = fmt.Sprintf("🎉 Pembayaran Berhasil!\n\nTerima kasih telah berlangganan paket <b>%s</b>.\nPaket Anda aktif sampai: <b>%s</b>\n\nBerikut adalah link khusus untuk masuk ke grup:\n%s\n\n<i>Link ini hanya berlaku untuk 1 kali pakai.</i>", pkg.Name, expiredStr, strings.Join(inviteLinks, "\n"))
		}
	}

	if receiptURL != "" {
		message = fmt.Sprintf("%s\n\n📄 <a href=\"%s\">Download Kwitansi Pembayaran</a>", message, receiptURL)
	}
	if len(failedGroups) > 0 {
		message = fmt.Sprintf("%s\n\n⚠️ Gagal membuat link untuk: %s. Silakan hubungi admin.", message, strings.Join(failedGroups, ", "))
	}

	if dmClient != nil {
		if err := dmClient.SendMessage(ctx, payload.TelegramUserID, message); err != nil {
			h.logger.Error("telegram action handler: failed to send dm", append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Error(err))...)
			return fmt.Errorf("failed to send dm: %w", err)
		}
	} else {
		h.logger.Error("telegram action handler: no bot available to send dm", logFields...)
	}

	// 8. Send PDF document AFTER text DM (so user reads the welcome message first)
	if receiptPDFBytes != nil && dmClient != nil {
		doc := tgbotapi.NewDocument(payload.TelegramUserID, tgbotapi.FileBytes{
			Name:  "kwitansi.pdf",
			Bytes: receiptPDFBytes,
		})
		doc.Caption = fmt.Sprintf("Kwitansi pembayaran paket %s", pkg.Name)
		if _, err := dmClient.SendDocument(ctx, doc); err != nil {
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
	// 1. Fetch Order via repository
	var order *entity.Order
	if payload.OrderID != "" {
		orderID, err := uuid.Parse(payload.OrderID)
		if err != nil {
			h.logger.Warn("receipt: invalid order_id", append(logFields, zap.String("order_id", payload.OrderID), zap.Error(err))...)
			return nil, "", fmt.Errorf("invalid order_id: %w", err)
		}
		order, err = h.orderRepo.FindByID(ctx, h.db, orderID)
		if err != nil {
			h.logger.Warn("receipt: order lookup failed", append(logFields, zap.String("order_id", payload.OrderID), zap.Error(err))...)
			return nil, "", fmt.Errorf("order lookup failed: %w", err)
		}
	} else {
		// Fallback: older events may not carry order_id — look up by subscription_id
		subID, err := uuid.Parse(payload.SubscriptionID)
		if err != nil {
			h.logger.Warn("receipt: invalid subscription_id", append(logFields, zap.String("subscription_id", payload.SubscriptionID), zap.Error(err))...)
			return nil, "", fmt.Errorf("invalid subscription_id: %w", err)
		}
		order, err = h.orderRepo.FindBySubscriptionID(ctx, h.db, subID)
		if err != nil {
			h.logger.Warn("receipt: order lookup by subscription failed", append(logFields, zap.String("subscription_id", payload.SubscriptionID), zap.Error(err))...)
			return nil, "", fmt.Errorf("order lookup by subscription failed: %w", err)
		}
	}
	if order == nil {
		h.logger.Warn("receipt: order not found", append(logFields, zap.String("order_id", payload.OrderID), zap.String("subscription_id", payload.SubscriptionID))...)
		return nil, "", fmt.Errorf("order not found")
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
	if err := h.orderRepo.Update(ctx, h.db, order); err != nil {
		h.logger.Warn("receipt: failed to save receipt_url on order", append(logFields, zap.Error(err))...)
		// non-fatal, continue
	}

	h.logger.Info("receipt: generated and uploaded", append(logFields, zap.String("receipt_url", receiptURL))...)
	return pdfBytes, receiptURL, nil
}
