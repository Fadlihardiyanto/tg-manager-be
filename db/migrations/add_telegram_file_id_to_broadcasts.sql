-- =======================================================
-- ADD telegram_file_id column to broadcasts
-- =======================================================
ALTER TABLE broadcasts ADD COLUMN telegram_file_id VARCHAR(255);
