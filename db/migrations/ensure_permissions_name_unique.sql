-- ==========================================
-- Pastikan unique constraint pada permissions.name ada.
-- Schema lama mungkin dibuat tanpa UNIQUE — seed_tenant_rbac.sql
-- (ON CONFLICT (name)) butuh constraint ini. Idempotent.
-- ==========================================
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        WHERE t.relname = 'permissions'
          AND c.conname = 'permissions_name_key'
    ) THEN
        ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);
    END IF;
END $$;
