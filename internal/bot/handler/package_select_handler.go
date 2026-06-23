package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// IMemberOrderUseCase local interface to avoid import cycle
type IMemberOrderUseCase interface {
	Checkout(ctx context.Context, req *model.MemberCheckoutRequest) (*model.MemberCheckoutResponse, error)
	CheckActiveSubscriptions(ctx context.Context, tgUserID int64, packageID uuid.UUID) (*model.ActiveSubscriptionCheckResult, error)
}

type PackageSelectHandler struct {
	memberOrderUC   IMemberOrderUseCase
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewPackageSelectHandler(
	memberOrderUC IMemberOrderUseCase,
	factory telegram.BotFactory,
	encKey string,
	log *zap.Logger,
) *PackageSelectHandler {
	return &PackageSelectHandler{
		memberOrderUC:   memberOrderUC,
		telegramFactory: factory,
		encryptionKey:   encKey,
		log:             log,
	}
}

func (h *PackageSelectHandler) Prefix() string {
	return "pkg_sel:"
}

func (h *PackageSelectHandler) AllowedRoles() []string {
	return []string{"all_in_one", "sales_only"}
}

func (h *PackageSelectHandler) Execute(ctx context.Context, bot *entity.TelegramBot, query *tgbotapi.CallbackQuery) error {
	h.log.Info("executing package select callback", zap.Int64("user_id", query.From.ID), zap.String("bot_id", bot.ID.String()))

	// Initialize Telegram Client
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.log.Error("failed to decrypt token", zap.Error(err))
		return err
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.log.Error("failed to init bot client", zap.Error(err))
		return err
	}

	// 1. Answer Callback Query to stop loading animation
	callbackCfg := tgbotapi.NewCallback(query.ID, "")
	if _, err := botClient.Request(ctx, callbackCfg); err != nil {
		h.log.Warn("failed to answer callback query", zap.Error(err))
	}

	// 2. Check for cancel action
	if query.Data == h.Prefix()+"cancel" {
		if query.Message != nil {
			delMsg := tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
			if _, err := botClient.Request(ctx, delMsg); err != nil {
				h.log.Warn("failed to delete message on cancel", zap.Error(err))
			}
		}
		return nil
	}

	// 3. Extract Package ID
	// Data format: "pkg_sel:uuid" or "pkg_sel:uuid:1" (forced)
	parts := strings.Split(query.Data, ":")
	if len(parts) < 2 {
		h.log.Error("invalid callback data format", zap.String("data", query.Data))
		return fmt.Errorf("invalid callback data format")
	}

	packageIDStr := parts[1]
	packageID, err := uuid.Parse(packageIDStr)
	if err != nil {
		h.log.Error("invalid package ID in callback data", zap.String("package_id", packageIDStr))
		return fmt.Errorf("invalid package id")
	}

	isForced := false
	if len(parts) == 3 && parts[2] == "1" {
		isForced = true
	}

	// 2.5. Check for active subscription
	if !isForced {
		checkResult, err := h.memberOrderUC.CheckActiveSubscriptions(ctx, query.From.ID, packageID)
		if err != nil {
			h.log.Error("failed to check active subscriptions", zap.Error(err))
			// fallback: continue to checkout if checking fails
		} else if checkResult != nil && (checkResult.SamePackage != nil || checkResult.AllAccess != nil) {
			h.log.Info("active subscription found, asking for confirmation")

			loc, _ := time.LoadLocation("Asia/Jakarta")
			var replyText string

			if checkResult.SamePackage != nil {
				activeSub := checkResult.SamePackage
				expiredStr := activeSub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")
				pkgName := activeSub.Package.Name
				if pkgName == "" {
					pkgName = "ini" // fallback if preload failed
				}
				replyText = fmt.Sprintf("⚠️ <b>Perhatian!</b>\n\nAnda saat ini sudah memiliki paket <b>%s</b> yang masih aktif dan baru akan kedaluwarsa pada <b>%s</b>.\n\nApakah Anda tetap ingin melanjutkan pembelian? (Masa aktif akan otomatis diakumulasikan/diperpanjang).", pkgName, expiredStr)
			} else if checkResult.AllAccess != nil {
				activeSub := checkResult.AllAccess
				expiredStr := activeSub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")
				pkgName := activeSub.Package.Name
				if pkgName == "" {
					pkgName = "All Access"
				}
				replyText = fmt.Sprintf("⚠️ <b>Perhatian!</b>\n\nAnda saat ini sudah memiliki paket <b>%s</b> (Akses Semua Grup) yang masih aktif sampai <b>%s</b>.\n\nPaket yang sedang Anda pilih saat ini mungkin tidak berguna karena Anda sudah memiliki akses ke semua grup.\n\nApakah Anda YAKIN tetap ingin melanjutkan pembelian?", pkgName, expiredStr)
			}

			var chatID int64
			if query.Message != nil {
				chatID = query.Message.Chat.ID
			} else {
				chatID = query.From.ID
			}

			replyMsg := tgbotapi.NewMessage(chatID, replyText)
			replyMsg.ParseMode = tgbotapi.ModeHTML

			confirmData := fmt.Sprintf("%s%s:1", h.Prefix(), packageID.String())
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Tetap Lanjut", confirmData),
				),
				tgbotapi.NewInlineKeyboardRow(
					// Menghapus pesan konfirmasi jika batal
					tgbotapi.NewInlineKeyboardButtonData("❌ Batal", h.Prefix()+"cancel"),
				),
			)
			replyMsg.ReplyMarkup = keyboard

			if _, err := botClient.Send(ctx, replyMsg); err != nil {
				h.log.Error("failed to send confirmation message", zap.Error(err))
				return err
			}
			return nil
		}
	}

	// 3. Prepare Checkout Request
	checkoutReq := &model.MemberCheckoutRequest{
		PackageID:      packageID,
		TelegramUserID: query.From.ID,
		Username:       query.From.UserName,
		FirstName:      query.From.FirstName,
		LastName:       query.From.LastName,
	}

	// 4. Call Checkout UseCase
	h.log.Info("initiating checkout", zap.String("package_id", packageID.String()), zap.Bool("is_forced", isForced))
	checkoutResp, err := h.memberOrderUC.Checkout(ctx, checkoutReq)

	var replyText string
	if err != nil {
		h.log.Error("failed to checkout package", zap.Error(err))
		replyText = userFacingError(err)
	} else {
		replyText = buildCheckoutSuccessMessage(checkoutResp)
	}

	// 5. Send new message with payment link (Reply to the original message chat)
	var chatID int64
	if query.Message != nil {
		chatID = query.Message.Chat.ID
	} else {
		// Fallback if message is somehow missing (e.g., inline bot), though rare for our flow
		chatID = query.From.ID
	}

	replyMsg := tgbotapi.NewMessage(chatID, replyText)
	replyMsg.ParseMode = tgbotapi.ModeHTML

	if _, err := botClient.Send(ctx, replyMsg); err != nil {
		h.log.Error("failed to send payment reply", zap.Error(err))
		return err
	}

	h.log.Info("successfully replied with payment link")
	return nil
}

// userFacingError menentukan pesan error yang aman ditampilkan ke user Telegram.
// Error dari helper (ErrBadRequest, ErrConflict, dll) dianggap user-friendly
// karena pesannya sudah ditulis dalam bahasa yang bisa dipahami user.
// Error lainnya (internal/sistem) ditampilkan sebagai pesan generik.
func userFacingError(err error) string {
	var (
		errBadRequest *helper.ErrBadRequest
		errConflict   *helper.ErrConflict
		errNotFound   *helper.ErrNotFound
		errForbidden  *helper.ErrForbidden
	)

	switch {
	case errors.As(err, &errBadRequest):
		return fmt.Sprintf("⚠️ %s", errBadRequest.Message)
	case errors.As(err, &errConflict):
		return fmt.Sprintf("⚠️ %s", errConflict.Message)
	case errors.As(err, &errNotFound):
		return fmt.Sprintf("⚠️ %s tidak ditemukan.", errNotFound.Resource)
	case errors.As(err, &errForbidden):
		return fmt.Sprintf("🚫 %s", errForbidden.Message)
	default:
		return "❌ Gagal memproses pesanan Anda. Silakan coba beberapa saat lagi atau hubungi admin."
	}
}

// buildCheckoutSuccessMessage membuat pesan sukses checkout yang informatif
// dengan detail paket, harga, diskon, dan link pembayaran.
func buildCheckoutSuccessMessage(resp *model.MemberCheckoutResponse) string {
	var sb strings.Builder

	sb.WriteString("✅ <b>Pesanan Berhasil Dibuat!</b>\n\n")
	sb.WriteString(fmt.Sprintf("📦 <b>Paket:</b> %s\n", resp.PackageName))
	sb.WriteString(fmt.Sprintf("📅 <b>Durasi:</b> %d hari\n", resp.DurationDays))
	sb.WriteString("\n─────────────────────\n")

	// Price breakdown
	hasDiscount := resp.DiscountAmount.GreaterThan(decimal.Zero)
	if hasDiscount {
		sb.WriteString(fmt.Sprintf("💰 Harga     : <s>%s</s>\n", formatRupiah(resp.OriginalAmount)))
		sb.WriteString(fmt.Sprintf("🎁 Diskon    : -%s\n", formatRupiah(resp.DiscountAmount)))
		sb.WriteString(fmt.Sprintf("💳 <b>Total     : %s</b>\n", formatRupiah(resp.Amount)))
	} else {
		sb.WriteString(fmt.Sprintf("💳 <b>Total     : %s</b>\n", formatRupiah(resp.Amount)))
	}

	sb.WriteString("─────────────────────\n\n")
	sb.WriteString(fmt.Sprintf("🔗 <b>Link Pembayaran:</b>\n%s\n\n", resp.PaymentURL))
	sb.WriteString("<i>⏳ Link pembayaran akan kedaluwarsa dalam 24 jam.</i>")

	return sb.String()
}

// formatRupiah memformat decimal.Decimal menjadi format mata uang Indonesia.
// Contoh: 150000 → "Rp 150.000"
func formatRupiah(amount decimal.Decimal) string {
	p := message.NewPrinter(language.Indonesian)
	return p.Sprintf("Rp %d", amount.IntPart())
}
