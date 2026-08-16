-- ==========================================
-- Daily report: per-tenant settings + job event sink
-- job_events: 1 baris per aksi worker (kick, reminder, DM) — laporan harian
-- dan halaman kegagalan (GET /admin/v1/report-failures) membaca dari sini
-- ==========================================

CREATE TABLE IF NOT EXISTS report_settings (
    client_id      UUID PRIMARY KEY REFERENCES clients(id),
    enabled        BOOLEAN NOT NULL DEFAULT FALSE,
    target_chat_id BIGINT NOT NULL,
    bot_id         UUID NOT NULL REFERENCES telegram_bots(id),
    report_time    VARCHAR(5) NOT NULL DEFAULT '08:00',
    last_sent_at   TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS job_events (
    id               BIGSERIAL PRIMARY KEY,
    client_id        UUID NOT NULL,
    event_type       VARCHAR(50) NOT NULL,
    status           VARCHAR(10) NOT NULL,
    bot_id           UUID,
    telegram_chat_id BIGINT,
    detail           TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_job_events_created ON job_events (created_at);
CREATE INDEX IF NOT EXISTS idx_job_events_client ON job_events (client_id, created_at);
