-- ==========================================
-- Pastikan unique constraint pada kolom name ada di tabel yang di-seed.
-- Schema lama mungkin dibuat tanpa UNIQUE. Seed kini memakai
-- ON CONFLICT DO NOTHING (tanpa arbiter) jadi tidak error tanpa constraint,
-- tapi constraint ini tetap berguna agar seed idempotent (anti duplikat).
-- Idempotent — aman dijalankan berulang.
-- ==========================================
DO $$
BEGIN
    -- permissions.name
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        WHERE t.relname = 'permissions' AND c.conname = 'permissions_name_key'
    ) THEN
        ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);
    END IF;

    -- roles.name
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        WHERE t.relname = 'roles' AND c.conname = 'roles_name_key'
    ) THEN
        ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);
    END IF;
END $$;
