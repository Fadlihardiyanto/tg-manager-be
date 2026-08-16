package messaging

import (
	"bytes"
	"context"
	"errors"
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

type groupInvite struct {
	Name string
	URL  string
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
		// Package tidak ditemukan = anomali data permanen, terminal (jangan retry ke DLQ).
		// Error lain (DB down dll) tetap retryable.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Error("telegram action handler: package not found", append(logFields, zap.String("package_id", payload.PackageID))...)
			return nil
		}
		return fmt.Errorf("package lookup failed: %w", err)
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
	var inviteLinks []groupInvite
	var failedGroups []string
	var dmClient telegram.BotClient
	subIDPrefix := payload.SubscriptionID
	if len(subIDPrefix) > 8 {
		subIDPrefix = subIDPrefix[:8]
	}

	for botID, groups := range groupsByBot {
		bot, err := h.botRepo.FindByID(ctx, h.db, botID)
		if err != nil || bot == nil {
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
			inviteLinks = append(inviteLinks, groupInvite{Name: group.Name, URL: inviteLink})
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

	// 5. Fetch Subscription for Expiration Date
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

	// 6. Generate Receipt PDF and Upload to S3 (skip for resend link — no order data)
	var receiptPDFBytes []byte
	if !payload.IsResend {
		var receiptErr error
		receiptPDFBytes, _, receiptErr = h.generateReceipt(ctx, payload, pkg, sub.ExpiredAt, logFields)
		if receiptErr != nil {
			h.logger.Error("telegram action handler: failed to generate receipt", append(logFields, zap.Error(receiptErr))...)
		}
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// ponytail: container tanpa tzdata (alpine/distroless) → fallback UTC,
		// bukan panic di Time.In(nil) yang membuat pesan masuk DLQ.
		loc = time.UTC
	}
	expiredStr := sub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")

	// 7. Build DM message text (no links — links are inline buttons)
	var message string
	if payload.IsResend {
		if len(inviteLinks) == 0 {
			message = fmt.Sprintf("⚠️ Mohon maaf, kami mengalami kendala teknis saat membuat link akses untuk paket <b>%s</b>.\nSilakan hubungi Admin untuk bantuan lebih lanjut.", pkg.Name)
		} else {
			message = fmt.Sprintf("👋 Halo!\n\nBerikut adalah link akses ulang Anda untuk masuk ke grup paket <b>%s</b>.\n\n<i>Link ini hanya berlaku untuk 1 kali pakai.</i>", pkg.Name)
		}
	} else {
		if len(inviteLinks) == 0 {
			message = fmt.Sprintf("🎉 Pembayaran Berhasil!\n\nTerima kasih telah berlangganan paket <b>%s</b>.\nPaket Anda aktif sampai: <b>%s</b>\n\n⚠️ Mohon maaf, kami mengalami kendala teknis saat membuat link akses grup.\nSilakan hubungi Admin untuk bantuan lebih lanjut.", pkg.Name, expiredStr)
		} else {
			message = fmt.Sprintf("🎉 Pembayaran Berhasil!\n\nTerima kasih telah berlangganan paket <b>%s</b>.\nPaket Anda aktif sampai: <b>%s</b>\n\nSilakan klik tombol di bawah untuk masuk ke grup:", pkg.Name, expiredStr)
		}
	}

	if len(failedGroups) > 0 {
		message = fmt.Sprintf("%s\n\n⚠️ Gagal membuat link untuk: %s. Silakan hubungi admin.", message, strings.Join(failedGroups, ", "))
	}

	// 8. Build inline keyboard from invite links
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, invite := range inviteLinks {
		btn := tgbotapi.NewInlineKeyboardButtonURL("🔗 Gabung: "+invite.Name, invite.URL)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}

	// 9. Send message with inline keyboard
	if dmClient != nil {
		msg := tgbotapi.NewMessage(payload.TelegramUserID, message)
		msg.ParseMode = "HTML"
		if len(rows) > 0 {
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		}
		if _, err := dmClient.Send(ctx, msg); err != nil {
			h.logger.Error("telegram action handler: failed to send dm", append(logFields, zap.Int64("user_id", payload.TelegramUserID), zap.Error(err))...)
			// Error permanen (chat not found / bot diblokir / bot tidak bisa
			// memulai percakapan): retry tidak akan pernah sukses — drop event
			// (terminal) supaya tidak DLQ-loop selamanya. Subscription tetap
			// aktif; user bisa dapat link via admin/ulang.
			if telegram.IsPermanentError(err) {
				h.logger.Warn("telegram action handler: permanent telegram error, dropping event",
					append(logFields, zap.Int64("user_id", payload.TelegramUserID))...)
				return nil
			}
			return fmt.Errorf("failed to send dm: %w", err)
		}
	} else {
		h.logger.Error("telegram action handler: no bot available to send dm", logFields...)
	}

	// 8. Send PDF document AFTER text DM (so user reads the welcome message first)
	if receiptPDFBytes != nil && dmClient != nil {
		doc := tgbotapi.NewDocument(payload.TelegramUserID, tgbotapi.FileBytes{
			Name:  "bukti pembayaran.pdf",
			Bytes: receiptPDFBytes,
		})
		doc.Caption = fmt.Sprintf("Bukti Pembayaran Paket %s", pkg.Name)
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
	subExpiredAt time.Time,
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

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// ponytail: container tanpa tzdata → fallback UTC, jangan panic (lihat Handle).
		loc = time.UTC
	}
	paidAt := time.Now()
	if order.PaidAt != nil {
		paidAt = *order.PaidAt
	}

	durationText := fmt.Sprintf("%d Hari", pkg.DurationDays)

	oldExpiry := subExpiredAt.AddDate(0, 0, -pkg.DurationDays)
	remainingDays := int(time.Until(oldExpiry) / (24 * time.Hour))
	if remainingDays > 0 {
		durationText += fmt.Sprintf("\nSisa Langganan sebelumnya: %d Hari", remainingDays)
	}
	durationText += fmt.Sprintf("\nAktif s.d. %s", subExpiredAt.In(loc).Format("02 Jan 2006"))

	receiptData := &pdf.ReceiptData{
		MerchantName:    client.Name,
		OrderID:         order.ExternalID,
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
				Duration: durationText,
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
	if h.pdfClient == nil {
		h.logger.Warn("receipt: pdf client not configured, skipping", logFields...)
		return nil, "", fmt.Errorf("pdf client not configured")
	}
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
