-- ==========================================
-- 13. BROADCASTS (Pesan Massal/Pengumuman)
-- ==========================================
CREATE TABLE broadcasts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    bot_id          UUID NOT NULL REFERENCES telegram_bots(id) ON DELETE CASCADE,
    target_type     VARCHAR(20) NOT NULL, -- 'group' atau 'member' (DM)
    message_type    VARCHAR(20) NOT NULL, -- 'text', 'photo', 'document'
    message_text    TEXT NOT NULL,
    file_url        VARCHAR(500),
    telegram_file_id VARCHAR(255), -- Menyimpan File ID unik dari Telegram jika message_type = 'photo' atau 'document'
    status          VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    total_targets   INT NOT NULL DEFAULT 0,
    sent_count      INT NOT NULL DEFAULT 0,
    failed_count    INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

CREATE INDEX idx_broadcasts_client_bot ON broadcasts(client_id, bot_id);
