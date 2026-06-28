-- =======================================================
-- ADD max_broadcasts column to platform_plans
-- =======================================================
ALTER TABLE platform_plans ADD COLUMN max_broadcasts INT NOT NULL DEFAULT 3;
