package messaging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/metrics"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/reporting"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/repository"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/crypto"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/telegram"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DailyReportWorker mengirim laporan harian (kick, reminder, DM, DLQ) per
// tenant sesuai report_settings. Interval ticker cukup 1 menit — dedup via
// last_sent_at (restart-safe), kirim ulang menit berikutnya kalau gagal.
type DailyReportWorker struct {
	db              *gorm.DB
	settingRepo     repository.IReportSettingRepository
	botRepo         repository.ITelegramBotRepository
	telegramFactory telegram.BotFactory
	encryptionKey   string
	log             *zap.Logger
}

func NewDailyReportWorker(
	db *gorm.DB,
	settingRepo repository.IReportSettingRepository,
	botRepo repository.ITelegramBotRepository,
	telegramFactory telegram.BotFactory,
	encryptionKey string,
	log *zap.Logger,
) *DailyReportWorker {
	return &DailyReportWorker{
		db:              db,
		settingRepo:     settingRepo,
		botRepo:         botRepo,
		telegramFactory: telegramFactory,
		encryptionKey:   encryptionKey,
		log:             log,
	}
}

func (w *DailyReportWorker) Start(ctx context.Context, interval time.Duration) {
	w.log.Info("daily report worker: starting", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("daily report worker: stopping")
			return
		case <-ticker.C:
			w.Process(ctx)
		}
	}
}

func (w *DailyReportWorker) Process(ctx context.Context) {
	start := time.Now()
	defer metrics.WorkerCycleDuration.WithLabelValues("daily_report").Observe(time.Since(start).Seconds())

	settings, err := w.settingRepo.FindEnabled(ctx, w.db)
	if err != nil {
		w.log.Error("daily report worker: failed to fetch enabled settings", zap.Error(err))
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	reportDate := today.AddDate(0, 0, -1)

	for _, setting := range settings {
		if now.Format("15:04") != setting.ReportTime {
			continue
		}
		if setting.LastSentAt != nil && !setting.LastSentAt.Before(today) {
			continue // sudah terkirim hari ini
		}

		if err := w.sendReport(ctx, setting, reportDate); err != nil {
			w.log.Error("daily report worker: failed to send report",
				zap.String("client_id", setting.ClientID.String()), zap.Error(err))
			continue // last_sent_at tidak di-update → retry tick berikutnya
		}

		if err := w.settingRepo.UpdateLastSentAt(ctx, w.db, setting.ClientID, time.Now()); err != nil {
			w.log.Error("daily report worker: failed to update last_sent_at",
				zap.String("client_id", setting.ClientID.String()), zap.Error(err))
		}
	}

	// Retensi 30 hari — jalankan setiap kali worker cycle lewat
	reporting.CleanupOlderThan(ctx, w.db, 30, w.log)
}

type dailyReportCounts struct {
	KickSuccess, KickFailed     int64
	ReminderSuccess, ReminderFailed int64
	DMSuccess, DMFailed         int64
	DLQ                         int64
}

func (w *DailyReportWorker) sendReport(ctx context.Context, setting entity.ReportSetting, reportDate time.Time) error {
	end := reportDate.AddDate(0, 0, 1)

	var c dailyReportCounts
	err := w.db.WithContext(ctx).Raw(
		`SELECT
			COALESCE(SUM((event_type='enforcer.kick' AND status='success')::int), 0) AS kick_success,
			COALESCE(SUM((event_type='enforcer.kick' AND status='failed')::int), 0)  AS kick_failed,
			COALESCE(SUM((event_type='expiry.reminder' AND status='success')::int), 0) AS reminder_success,
			COALESCE(SUM((event_type='expiry.reminder' AND status='failed')::int), 0)  AS reminder_failed,
			COALESCE(SUM((event_type='telegram.dm' AND status='success')::int), 0)     AS dm_success,
			COALESCE(SUM((event_type='telegram.dm' AND status='failed')::int), 0)      AS dm_failed
		 FROM job_events
		 WHERE client_id = ? AND created_at >= ? AND created_at < ?`,
		setting.ClientID, reportDate, end,
	).Scan(&c).Error
	if err != nil {
		return fmt.Errorf("daily report: aggregate job_events: %w", err)
	}

	// Outbox DLQ: event gagal permanen (status='failed') — JOIN ke subscription
	// untuk scope per-tenant. ponytail: DLQ broadcast/notification tanpa
	// subscription tidak terhitung per tenant.
	err = w.db.WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM outbox o
		 LEFT JOIN subscriptions s ON s.id = o.aggregate_id
		 LEFT JOIN packages p ON p.id = s.package_id
		 WHERE o.status = 'failed' AND p.client_id = ?
		   AND o.updated_at >= ? AND o.updated_at < ?`,
		setting.ClientID, reportDate, end,
	).Scan(&c.DLQ).Error
	if err != nil {
		return fmt.Errorf("daily report: aggregate outbox dlq: %w", err)
	}

	totalFailed := c.KickFailed + c.ReminderFailed + c.DMFailed + c.DLQ
	message := w.buildMessage(reportDate, c, totalFailed)

	// Kirim via bot tenant (decrypt token — pola sama seperti handler lain)
	bot, err := w.botRepo.FindByID(ctx, w.db, setting.BotID)
	if err != nil || bot == nil {
		return fmt.Errorf("daily report: bot not found: %v", err)
	}
	token, err := crypto.Decrypt(bot.Token, w.encryptionKey)
	if err != nil {
		return fmt.Errorf("daily report: decrypt bot token: %w", err)
	}
	botClient, err := w.telegramFactory.NewClient(token)
	if err != nil {
		return fmt.Errorf("daily report: init telegram client: %w", err)
	}
	if err := botClient.SendMessage(ctx, setting.TargetChatID, message); err != nil {
		return fmt.Errorf("daily report: send message: %w", err)
	}
	w.log.Info("daily report worker: report sent",
		zap.String("client_id", setting.ClientID.String()),
		zap.String("date", reportDate.Format("2006-01-02")),
		zap.Int64("total_failed", totalFailed),
	)
	return nil
}

func (w *DailyReportWorker) buildMessage(reportDate time.Time, c dailyReportCounts, totalFailed int64) string {
	var b strings.Builder
	b.WriteString("📊 Laporan Harian — ")
	b.WriteString(reportDate.Format("02/01/2006"))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "🔨 Kick member: %d ✅ · %d ❌\n", c.KickSuccess, c.KickFailed)
	fmt.Fprintf(&b, "⏰ Pengingat: %d ✅ · %d ❌\n", c.ReminderSuccess, c.ReminderFailed)
	fmt.Fprintf(&b, "✉️ DM aktivasi: %d ✅ · %d ❌\n", c.DMSuccess, c.DMFailed)
	fmt.Fprintf(&b, "📥 Outbox DLQ: %d\n", c.DLQ)
	b.WriteString("\nTotal gagal: ")
	if totalFailed > 0 {
		b.WriteString(fmt.Sprintf("<b>%d</b> — cek detail di dashboard > Laporan > Kegagalan", totalFailed))
	} else {
		b.WriteString("0 🎉")
	}
	return b.String()
}
