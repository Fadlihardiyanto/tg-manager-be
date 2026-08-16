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
	"github.com/redis/go-redis/v9"
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
	redisClient     *redis.Client
	log             *zap.Logger
}

func NewPackageSelectHandler(
	memberOrderUC IMemberOrderUseCase,
	factory telegram.BotFactory,
	encKey string,
	redisClient *redis.Client,
	log *zap.Logger,
) *PackageSelectHandler {
	return &PackageSelectHandler{
		memberOrderUC:   memberOrderUC,
		telegramFactory: factory,
		encryptionKey:   encKey,
		redisClient:     redisClient,
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
	userID := query.From.ID
	h.log.Info("executing package select callback", zap.Int64("user_id", userID), zap.String("bot_id", bot.ID.String()))

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

	chatID := int64(0)
	messageID := 0
	if query.Message != nil {
		chatID = query.Message.Chat.ID
		messageID = query.Message.MessageID
	}

	// 1. Cancel action — no lock needed
	if query.Data == h.Prefix()+"cancel" {
		if query.Message != nil {
			delMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
			if _, err := botClient.Request(ctx, delMsg); err != nil {
				h.log.Warn("failed to delete message on cancel", zap.Error(err))
			}
		}
		return nil
	}

	// 2. Redis lock — prevent spam clicks
	lockKey := fmt.Sprintf("lock:tg_checkout:%d", userID)
	lockTTL := 10 * time.Second

	var acquired bool
	if h.redisClient != nil {
		acquired, err = h.redisClient.SetNX(ctx, lockKey, "1", lockTTL).Result()
		if err != nil {
			h.log.Warn("redis lock error, proceeding without lock", zap.Error(err))
			acquired = true
		}
	} else {
		acquired = true
	}

	if !acquired {
		alert := tgbotapi.NewCallback(query.ID, "Mohon tunggu, pesanan Anda sedang diproses...")
		alert.ShowAlert = true
		if _, err := botClient.Request(ctx, alert); err != nil {
			h.log.Warn("failed to answer callback with alert", zap.Error(err))
		}
		return nil
	}
	defer func() {
		if h.redisClient != nil {
			h.redisClient.Del(ctx, lockKey)
		}
	}()

	// 3. Answer callback — stop loading spinner
	callbackCfg := tgbotapi.NewCallback(query.ID, "")
	if _, err := botClient.Request(ctx, callbackCfg); err != nil {
		h.log.Warn("failed to answer callback query", zap.Error(err))
	}

	// 4. Extract package ID
	parts := strings.Split(query.Data, ":")
	if len(parts) < 2 {
		h.log.Error("invalid callback data format", zap.String("data", query.Data))
		return nil
	}

	packageID, err := uuid.Parse(parts[1])
	if err != nil {
		h.log.Error("invalid package ID in callback data", zap.String("package_id", parts[1]))
		return nil
	}

	isForced := false
	if len(parts) == 3 && parts[2] == "1" {
		isForced = true
	}

	// 5. Check active subscription — edit original message with confirmation
	if !isForced {
		checkResult, err := h.memberOrderUC.CheckActiveSubscriptions(ctx, userID, packageID)
		if err != nil {
			h.log.Error("failed to check active subscriptions", zap.Error(err))
		} else if checkResult != nil && (checkResult.SamePackage != nil || checkResult.AllAccess != nil) {
			loc, _ := time.LoadLocation("Asia/Jakarta")
			var replyText string

			if checkResult.SamePackage != nil {
				activeSub := checkResult.SamePackage
				expiredStr := activeSub.ExpiredAt.In(loc).Format("02 Jan 2006 15:04 WIB")
				pkgName := activeSub.Package.Name
				if pkgName == "" {
					pkgName = "ini"
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

			confirmData := fmt.Sprintf("%s%s:1", h.Prefix(), packageID.String())
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Ya, Tetap Lanjut", confirmData),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("❌ Batal", h.Prefix()+"cancel"),
				),
			)

			if chatID != 0 && messageID != 0 {
				editMsg := tgbotapi.NewEditMessageText(chatID, messageID, replyText)
				editMsg.ParseMode = tgbotapi.ModeHTML
				editMsg.ReplyMarkup = &keyboard
				if _, err := botClient.Send(ctx, editMsg); err != nil {
					h.log.Error("failed to edit message for confirmation", zap.Error(err))
				}
			}
			return nil
		}
	}

	// 6. Show processing message — edit original message in-place
	if chatID != 0 && messageID != 0 {
		processingMsg := tgbotapi.NewEditMessageText(chatID, messageID, "⏳ <b>Sedang memproses pesanan...</b>\n\nMohon tunggu sebentar, kami sedang membuat link pembayaran Anda.")
		processingMsg.ParseMode = tgbotapi.ModeHTML
		if _, err := botClient.Send(ctx, processingMsg); err != nil {
			h.log.Warn("failed to show processing message", zap.Error(err))
		}
	}

	// 7. Call Checkout
	checkoutReq := &model.MemberCheckoutRequest{
		PackageID:      packageID,
		TelegramUserID: userID,
		Username:       query.From.UserName,
		FirstName:      query.From.FirstName,
		LastName:       query.From.LastName,
		BotID:          bot.ID, // bot asal checkout — DM aktivasi dikirim dari bot ini
	}

	h.log.Info("initiating checkout", zap.String("package_id", packageID.String()), zap.Bool("is_forced", isForced))
	checkoutResp, err := h.memberOrderUC.Checkout(ctx, checkoutReq)

	var replyText string
	if err != nil {
		h.log.Error("failed to checkout package", zap.Error(err))
		replyText = userFacingError(err)
	} else {
		replyText = buildCheckoutSuccessMessage(checkoutResp)
	}

	// 8. Edit original message with result
	if chatID != 0 && messageID != 0 {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, replyText)
		editMsg.ParseMode = tgbotapi.ModeHTML
		if _, err := botClient.Send(ctx, editMsg); err != nil {
			h.log.Warn("failed to edit message with result", zap.Error(err))
		}
	}

	h.log.Info("successfully processed package selection")
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
	sb.WriteString(fmt.Sprintf("🔗 <b>Link Pembayaran:</b> <a href=\"%s\">klik di sini untuk membayar</a>\n\n", resp.PaymentURL))
	sb.WriteString("<i>⏳ Link pembayaran akan kedaluwarsa dalam 24 jam.</i>")

	return sb.String()
}

// formatRupiah memformat decimal.Decimal menjadi format mata uang Indonesia.
// Contoh: 150000 → "Rp 150.000"
func formatRupiah(amount decimal.Decimal) string {
	p := message.NewPrinter(language.Indonesian)
	return p.Sprintf("Rp %d", amount.IntPart())
}
