-- ==========================================
-- Outbox indexes — query worker (FindPending, expiry-reminder dedup)
-- tanpa index = full scan tiap 5 detik / SLOW SQL >= 200ms
-- ==========================================

-- Dedup subquery di expiry reminder: WHERE event_type = ... AND status != 'failed' AND aggregate_id NOT IN (...)
CREATE INDEX IF NOT EXISTS idx_outbox_event_type_status ON outbox (event_type, status, aggregate_id);

-- Outbox worker polling: WHERE status = 'pending' AND process_after <= NOW()
CREATE INDEX IF NOT EXISTS idx_outbox_status_process_after ON outbox (status, process_after);
