-- =======================================================
-- ADD max_purchases_per_member column to packages
-- 0 = unlimited
-- =======================================================
ALTER TABLE packages ADD COLUMN max_purchases_per_member INT NOT NULL DEFAULT 0;
