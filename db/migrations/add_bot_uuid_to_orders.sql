-- ==========================================
-- Simpan bot asal checkout di orders — DM aktivasi/invite dikirim dari bot
-- ini (user pasti pernah chat bot tempat checkout, sedangkan bot pemilik grup
-- belum tentu — Telegram menolak DM dari bot yang belum pernah di-chat user)
-- ==========================================
ALTER TABLE orders ADD COLUMN IF NOT EXISTS bot_uuid UUID;

CREATE INDEX IF NOT EXISTS idx_orders_bot_uuid ON orders (bot_uuid);
