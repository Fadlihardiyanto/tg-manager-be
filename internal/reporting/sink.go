package reporting

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Record menulis 1 baris job event (fire-and-forget). Gagal hanya di-log —
// sink tidak pernah mengganggu alur utama worker.
func Record(ctx context.Context, db *gorm.DB, clientID any, eventType, status string, chatID any, detail string, log *zap.Logger) {
	result := db.WithContext(ctx).Exec(
		`INSERT INTO job_events (client_id, event_type, status, telegram_chat_id, detail, created_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		clientID, eventType, status, chatID, detail,
	)
	if result.Error != nil {
		log.Warn("reporting sink: failed to record job event", zap.String("event_type", eventType), zap.String("status", status), zap.Error(result.Error))
	}
}

// CleanupOlderThan menghapus job_events lebih lama dari n hari (retensi).
func CleanupOlderThan(ctx context.Context, db *gorm.DB, days int, log *zap.Logger) {
	// ponytail: pass days as text — pgx can't encode Go int for `? || ' days'`.
	result := db.WithContext(ctx).Exec(
		`DELETE FROM job_events WHERE created_at < CURRENT_TIMESTAMP - (?::text || ' days')::interval`,
		fmt.Sprintf("%d", days),
	)
	if result.Error != nil {
		log.Warn("reporting sink: failed to cleanup job events", zap.Error(result.Error))
	}
}
