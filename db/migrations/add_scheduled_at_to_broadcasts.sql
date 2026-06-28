-- =======================================================
-- ADD scheduled_at COLUMN TO broadcasts
-- =======================================================
ALTER TABLE broadcasts ADD COLUMN scheduled_at TIMESTAMP WITH TIME ZONE;

-- Create partial index to speed up scheduler queries
CREATE INDEX idx_broadcasts_scheduled_status ON broadcasts (status, scheduled_at) 
WHERE deleted_at IS NULL AND status = 'scheduled';
