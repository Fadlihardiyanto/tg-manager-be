package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// MyOrdersHandler handles the /myorders command.
// It shows the member their recent order statuses for this tenant's bot.
type MyOrdersHandler struct {
	db              *entity.Database
	orderRepo       repository.IOrderRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewMyOrdersHandler(
	db *entity.Database,
	orderRepo repository.IOrderRepository,
	factory telegram.BotFactory,
	encKey string,
	log *zap.Logger,
) *MyOrdersHandler {
	return &MyOrdersHandler{
		db:              db,
		orderRepo:       orderRepo,
		telegramFactory: factory,
		encryptionKey:   encKey,
		log:             log,
	}
}

func (h *MyOrdersHandler) Name() string {
	return "myorders"
}

func (h *MyOrdersHandler) AllowedRoles() []string {
	return []string{"all_in_one", "sales_only"}
}

func (h *MyOrdersHandler) Execute(ctx context.Context, bot *entity.TelegramBot, msg *tgbotapi.Message) error {
	h.log.Info("executing /myorders command",
		zap.Int64("user_id", msg.From.ID),
		zap.String("bot_id", bot.ID.String()),
	)

	// 1. Init Telegram client
	token, err := crypto.Decrypt(bot.Token, h.encryptionKey)
	if err != nil {
		h.log.Error("myorders handler: failed to decrypt token", zap.Error(err))
		return err
	}

	botClient, err := h.telegramFactory.NewClient(token)
	if err != nil {
		h.log.Error("myorders handler: failed to init bot client", zap.Error(err))
		return err
	}

	// 2. Fetch recent orders scoped to this bot's client (tenant)
	// Fetch last 5 orders
	orders, err := h.orderRepo.FindRecentByTelegramUserID(ctx, h.db.Gorm, msg.From.ID, bot.ClientID, 5)
	if err != nil {
		h.log.Error("myorders handler: failed to fetch orders", zap.Error(err))
		replyText := "❌ Gagal mengambil riwayat pembayaran. Silakan coba beberapa saat lagi."
		_ = botClient.SendMessage(ctx, msg.Chat.ID, replyText)
		return err
	}

	// 3. Build response
	replyText := h.buildReply(orders)

	if err := botClient.SendMessage(ctx, msg.Chat.ID, replyText); err != nil {
		h.log.Error("myorders handler: failed to send reply", zap.Error(err))
		return err
	}

	h.log.Info("myorders handler: successfully replied", zap.Int64("user_id", msg.From.ID))
	return nil
}

func (h *MyOrdersHandler) buildReply(orders []entity.Order) string {
	if len(orders) == 0 {
		return "📭 Anda tidak memiliki riwayat pembayaran.\n\nKetik /packages untuk melihat dan membeli paket yang tersedia."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💳 <b>Riwayat Pembayaran Anda (5 Terakhir)</b>\n"))

	loc, _ := time.LoadLocation("Asia/Jakarta")

	for i, order := range orders {
		sb.WriteString("\n─────────────────────\n")
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n", i+1, order.Package.Name))

		createdStr := order.CreatedAt.In(loc).Format("02 Jan 2006 15:04 WIB")
		
		// Status badge based on order status
		var statusBadge string
		switch order.Status {
		case "pending":
			statusBadge = "⏳ Pending (Menunggu Pembayaran)"
		case "settlement", "capture":
			statusBadge = "✅ Berhasil"
		case "expire":
			statusBadge = "❌ Kedaluwarsa"
		case "cancel", "deny":
			statusBadge = "🚫 Dibatalkan"
		default:
			statusBadge = fmt.Sprintf("ℹ️ %s", strings.ToTitle(order.Status))
		}

		sb.WriteString(fmt.Sprintf("Status  : %s\n", statusBadge))
		sb.WriteString(fmt.Sprintf("Tanggal : <b>%s</b>\n", createdStr))
		
		formattedAmount := h.formatRupiah(order.Amount.InexactFloat64())
		sb.WriteString(fmt.Sprintf("Total   : %s\n", formattedAmount))
		
		if order.Status == "pending" && order.PaymentURL != "" {
			sb.WriteString(fmt.Sprintf("Tautan  : <a href=\"%s\">Bayar Sekarang</a>\n", order.PaymentURL))
		}
	}

	sb.WriteString("\n─────────────────────\n")
	sb.WriteString("Untuk pertanyaan lebih lanjut, silakan hubungi admin.")

	return sb.String()
}

func (h *MyOrdersHandler) formatRupiah(amount float64) string {
	return fmt.Sprintf("Rp %s", h.thousandSeparator(int64(amount)))
}

func (h *MyOrdersHandler) thousandSeparator(n int64) string {
	in := fmt.Sprintf("%d", n)
	numOfDigits := len(in)
	if numOfDigits <= 3 {
		return in
	}

	out := make([]byte, 0, numOfDigits+(numOfDigits-1)/3)
	for i, c := range in {
		if i > 0 && (numOfDigits-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
