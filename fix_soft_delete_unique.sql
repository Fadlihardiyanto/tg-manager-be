-- ============================================================
-- FIX: Soft Delete + Unique Constraint Conflict
-- ============================================================
-- Problem: When a row is soft-deleted (deleted_at IS NOT NULL),
--          the plain UNIQUE constraint still counts it, so
--          re-creating with the same value causes a conflict.
-- Solution: Replace plain UNIQUE with a partial unique index
--           that only applies to non-deleted rows.
-- ============================================================

BEGIN;

-- 1. platform_plans.name
ALTER TABLE platform_plans DROP CONSTRAINT IF EXISTS platform_plans_name_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_plans_name_active
    ON platform_plans(name) WHERE deleted_at IS NULL;

-- 2. clients.slug
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_slug_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_clients_slug_active
    ON clients(slug) WHERE deleted_at IS NULL;

-- 3. roles.name
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_roles_name_active
    ON roles(name) WHERE deleted_at IS NULL;

-- 4. permissions.name
ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_name_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_permissions_name_active
    ON permissions(name) WHERE deleted_at IS NULL;

-- 5. admin_roles.name
ALTER TABLE admin_roles DROP CONSTRAINT IF EXISTS admin_roles_name_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_admin_roles_name_active
    ON admin_roles(name) WHERE deleted_at IS NULL;

-- 6. admin_permissions.name
ALTER TABLE admin_permissions DROP CONSTRAINT IF EXISTS admin_permissions_name_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_admin_permissions_name_active
    ON admin_permissions(name) WHERE deleted_at IS NULL;

-- 7. admin_users.email
ALTER TABLE admin_users DROP CONSTRAINT IF EXISTS admin_users_email_key;
DROP INDEX IF EXISTS idx_admin_users_email_key;  -- GORM may create this
CREATE UNIQUE INDEX IF NOT EXISTS uq_admin_users_email_active
    ON admin_users(email) WHERE deleted_at IS NULL;

-- 8. users.email
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_active
    ON users(email) WHERE deleted_at IS NULL;

-- 9. telegram_users.telegram_user_id
ALTER TABLE telegram_users DROP CONSTRAINT IF EXISTS telegram_users_telegram_user_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_telegram_users_tg_id_active
    ON telegram_users(telegram_user_id) WHERE deleted_at IS NULL;

-- 10. groups.telegram_chat_id
ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_telegram_chat_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_groups_telegram_chat_id_active
    ON groups(telegram_chat_id) WHERE deleted_at IS NULL;

-- 11. orders.external_id  (biasanya tidak di-soft-delete, tapi jaga-jaga)
ALTER TABLE orders DROP CONSTRAINT IF EXISTS orders_external_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_external_id_active
    ON orders(external_id) WHERE deleted_at IS NULL;

-- 12. platform_discounts.code
ALTER TABLE platform_discounts DROP CONSTRAINT IF EXISTS platform_discounts_code_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_platform_discounts_code_active
    ON platform_discounts(code) WHERE deleted_at IS NULL;

COMMIT;
