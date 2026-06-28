-- PostgreSQL 16 Schema for TG-Manager SaaS
-- Revisi: Fix forward reference, tambah outbox, perkuat multi-tenancy

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ==========================================
-- 1. USERS (dibuat duluan, direferensi oleh clients)
-- ==========================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    name            VARCHAR(255) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    phone           VARCHAR(50),
    avatar_url      VARCHAR(500),
    is_email_verified BOOLEAN DEFAULT false,
    last_login_at   TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

-- ==========================================
-- 2. CLIENTS (Pemilik Bisnis / Tenant)
-- ==========================================
CREATE TABLE clients (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL,
    slug                VARCHAR(255) UNIQUE NOT NULL,
    description         TEXT,
    logo_url            VARCHAR(500),
    is_active           BOOLEAN DEFAULT true,
    owner_user_id       UUID NOT NULL REFERENCES users(id),
    subscription_tier   VARCHAR(50) NOT NULL DEFAULT 'free',
    -- 'free', 'basic', 'pro', 'enterprise'
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMP
);

-- ==========================================
-- 3. CLIENT USERS (Multi-user per Tenant)
-- ==========================================
CREATE TABLE client_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(50) NOT NULL,
    -- 'owner', 'admin', 'manager', 'viewer'
    is_active   BOOLEAN DEFAULT true,
    invited_by  UUID REFERENCES users(id),
    invited_at  TIMESTAMP,
    accepted_at TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMP,

    UNIQUE(client_id, user_id)
);

-- ==========================================
-- 4. RBAC: ROLES & PERMISSIONS
-- ==========================================
CREATE TABLE roles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    description  TEXT,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) UNIQUE NOT NULL,
    -- e.g. 'packages.create'
    module      VARCHAR(50) NOT NULL,
    -- e.g. 'packages', 'groups', 'bots'
    action      VARCHAR(50) NOT NULL,
    -- e.g. 'create', 'read', 'update', 'delete'
    description TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMP
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
    -- Hapus deleted_at: junction table, gunakan hard delete
);

-- ==========================================
-- 5. TELEGRAM BOTS (White-label)
-- ==========================================
CREATE TABLE telegram_bots (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    token      TEXT NOT NULL,
    -- Encrypted AES-256, JANGAN simpan plaintext
    username   VARCHAR(255),
    bot_id     BIGINT,
    -- Telegram bot's numeric ID (dari getMe)
    is_active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- ==========================================
-- 6. GROUPS (Grup Telegram yang Dikelola)
-- ==========================================
CREATE TABLE groups (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id        UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    bot_id           UUID NOT NULL REFERENCES telegram_bots(id),
    telegram_chat_id BIGINT NOT NULL,
    name             VARCHAR(255),
    description      TEXT,
    member_count     INT DEFAULT 0,
    -- Di-update periodik, bukan realtime
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMP,

    -- Satu chat ID unik per client (bisa satu grup dikelola banyak client? Tidak.)
    UNIQUE(telegram_chat_id)
);

-- ==========================================
-- 7. PACKAGES (Paket Langganan)
-- ==========================================
CREATE TABLE packages (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id      UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    price          DECIMAL(12, 2) NOT NULL,
    duration_days  INT NOT NULL,
    is_all_access  BOOLEAN DEFAULT FALSE,
    -- true = akses semua grup milik client ini
    is_active      BOOLEAN DEFAULT TRUE,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMP
);

-- ==========================================
-- 8. PACKAGE GROUPS (Many-to-Many: Packages ↔ Groups)
-- ==========================================
CREATE TABLE package_groups (
    package_id UUID NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (package_id, group_id)
    -- Hard delete: kalau relasi berubah, hapus & buat ulang
);

-- ==========================================
-- 9. TELEGRAM USERS (Member)
-- ==========================================
CREATE TABLE telegram_users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_user_id BIGINT UNIQUE NOT NULL,
    username         VARCHAR(255),
    first_name       VARCHAR(255),
    last_name        VARCHAR(255),
    phone            VARCHAR(50),
    -- Bisa didapat jika user pernah contact bot
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMP
);

-- ==========================================
-- 10. SUBSCRIPTIONS
-- ==========================================
CREATE TABLE subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Relasi ke Member & Paket
    telegram_user_id    UUID NOT NULL REFERENCES telegram_users(id),
    package_id          UUID NOT NULL REFERENCES packages(id),
    
    -- TAMBAHAN: client_id untuk isolasi multi-tenant di worker
    client_id           UUID NOT NULL REFERENCES clients(id),
    
    -- TAMBAHAN: order_id untuk traceability (payment -> subscription)
    order_id            UUID,
    -- FK ke orders ditambah setelah tabel orders dibuat

    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    -- 'active', 'expired', 'cancelled'
    
    activated_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    expired_at          TIMESTAMP NOT NULL,
    
    -- Stacking: kalau beli lagi, expired_at += duration_days
    -- Parallel: bisa ada multiple rows aktif untuk package berbeda
    
    auto_renew          BOOLEAN DEFAULT FALSE,
    grace_period_hours  INT NOT NULL DEFAULT 0,
    kicked_at           TIMESTAMP,
    -- Kapan bot mengeksekusi kick
    last_checked_at     TIMESTAMP NOT NULL DEFAULT NOW(),

    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMP
);

-- ==========================================
-- 11. ORDERS (Transaksi Pembayaran)
-- ==========================================
CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Relasi
    telegram_user_id UUID NOT NULL REFERENCES telegram_users(id),
    package_id      UUID NOT NULL REFERENCES packages(id),
    client_id       UUID NOT NULL REFERENCES clients(id),
    -- TAMBAHAN: untuk multi-tenant isolation
    subscription_id UUID REFERENCES subscriptions(id),
    -- NULL saat pending, diisi setelah payment sukses
    
    -- Payment Gateway
    external_id     VARCHAR(255) UNIQUE NOT NULL,
    -- ID dari Midtrans/Xendit, untuk idempotency
    payment_url     VARCHAR(500),
    -- URL redirect ke payment page
    amount          DECIMAL(12, 2) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 'pending', 'paid', 'failed', 'expired'
    payment_method  VARCHAR(50),
    -- 'bca_va', 'gopay', 'qris', etc
    
    paid_at         TIMESTAMP,
    expired_at      TIMESTAMP,
    -- Kapan payment link expired
    
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

-- Sekarang bisa tambah FK dari subscriptions ke orders
ALTER TABLE subscriptions
    ADD CONSTRAINT fk_subscriptions_order
    FOREIGN KEY (order_id) REFERENCES orders(id);

-- Receipt PDF URL (generated after payment)
ALTER TABLE orders ADD COLUMN IF NOT EXISTS receipt_url VARCHAR(500);

-- ==========================================
-- 12. OUTBOX (Atomic Event Publishing)
-- ==========================================
-- Pola Outbox untuk menjamin atomicity antara:
-- DB Transaction (update subscription) dan Event Publishing (RabbitMQ)
CREATE TABLE outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type  VARCHAR(100) NOT NULL,
    -- e.g. 'subscription', 'order'
    aggregate_id    UUID NOT NULL,
    -- ID entity terkait
    event_type      VARCHAR(100) NOT NULL,
    -- e.g. 'subscription.activated', 'member.kick'
    payload         JSONB NOT NULL,
    -- Data yang akan dipublish ke RabbitMQ
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 'pending', 'processed', 'failed'
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    last_error      TEXT,
    -- Error message terakhir jika gagal
    process_after   TIMESTAMP NOT NULL DEFAULT NOW(),
    -- Untuk delay/backoff
    processed_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
    -- Tidak perlu deleted_at, outbox di-cleanup setelah processed
);

-- ==========================================
-- 13. AUDIT LOGS
-- ==========================================
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   UUID REFERENCES clients(id),
    -- TAMBAHAN: untuk filter audit per tenant
    entity_type VARCHAR(50) NOT NULL,
    -- 'subscription', 'order', 'join_request', 'member'
    entity_id   UUID NOT NULL,
    action      VARCHAR(50) NOT NULL,
    -- 'approved', 'declined', 'kicked', 'payment_received'
    actor_type  VARCHAR(50),
    -- 'bot', 'system', 'admin', 'user'
    actor_id    VARCHAR(255),
    -- UUID atau Telegram ID tergantung actor_type
    metadata    JSONB,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
    -- Audit log tidak perlu updated_at/deleted_at: immutable record
);

-- ==========================================
-- INDEXES
-- ==========================================

-- [Client Users]
CREATE INDEX idx_client_users_client ON client_users(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_client_users_user ON client_users(user_id) WHERE deleted_at IS NULL;

-- [Telegram Bots]
CREATE INDEX idx_telegram_bots_client ON telegram_bots(client_id) WHERE deleted_at IS NULL AND is_active = TRUE;

-- [Groups]
CREATE INDEX idx_groups_client ON groups(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_groups_bot ON groups(bot_id) WHERE deleted_at IS NULL;

-- [Packages]
CREATE INDEX idx_packages_client ON packages(client_id) WHERE deleted_at IS NULL AND is_active = TRUE;

-- [Telegram Users]
CREATE INDEX idx_telegram_users_tg_id 
ON telegram_users 
USING HASH (telegram_user_id);

-- [Subscriptions] — CRITICAL untuk Worker & Gatekeeping
-- Partial index: hanya row aktif yang masuk index (hemat storage)
CREATE INDEX idx_subscriptions_expired_worker
    ON subscriptions(expired_at, client_id)
    WHERE status = 'active' AND deleted_at IS NULL;

-- Untuk Gatekeeping: cek apakah user punya sub aktif di package tertentu
CREATE INDEX idx_subscriptions_user_package
    ON subscriptions(telegram_user_id, package_id)
    WHERE status = 'active' AND deleted_at IS NULL;

-- Untuk dashboard: list semua sub per client
CREATE INDEX idx_subscriptions_client
    ON subscriptions(client_id, status)
    WHERE deleted_at IS NULL;

-- [Orders]
CREATE INDEX idx_orders_external_id ON orders(external_id);
-- Sudah UNIQUE, tapi eksplisit untuk clarity
CREATE INDEX idx_orders_client_status ON orders(client_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_telegram_user ON orders(telegram_user_id) WHERE deleted_at IS NULL;

-- [Outbox] — CRITICAL untuk reliabilitas event publishing
CREATE INDEX idx_outbox_pending
    ON outbox(process_after, retry_count)
    WHERE status = 'pending';
-- Worker poll: "ambil outbox yang belum diproses dan sudah waktunya"

-- [Audit Logs]
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_client ON audit_logs(client_id, created_at DESC);-- PostgreSQL 16 Schema for TG-Manager SaaS
-- Revisi: Fix forward reference, tambah outbox, perkuat multi-tenancy

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ==========================================
-- 1. USERS (dibuat duluan, direferensi oleh clients)
-- ==========================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    name            VARCHAR(255) NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    phone           VARCHAR(50),
    avatar_url      VARCHAR(500),
    is_email_verified BOOLEAN DEFAULT false,
    last_login_at   TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

-- ==========================================
-- 2. CLIENTS (Pemilik Bisnis / Tenant)
-- ==========================================
CREATE TABLE clients (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL,
    slug                VARCHAR(255) UNIQUE NOT NULL,
    description         TEXT,
    logo_url            VARCHAR(500),
    is_active           BOOLEAN DEFAULT true,
    owner_user_id       UUID NOT NULL REFERENCES users(id),
    subscription_tier   VARCHAR(50) NOT NULL DEFAULT 'free',
    -- 'free', 'basic', 'pro', 'enterprise'
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMP
);

-- ==========================================
-- 3. CLIENT USERS (Multi-user per Tenant)
-- ==========================================
CREATE TABLE client_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(50) NOT NULL,
    -- 'owner', 'admin', 'manager', 'viewer'
    is_active   BOOLEAN DEFAULT true,
    invited_by  UUID REFERENCES users(id),
    invited_at  TIMESTAMP,
    accepted_at TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMP,

    UNIQUE(client_id, user_id)
);

-- ==========================================
-- 4. RBAC: ROLES & PERMISSIONS
-- ==========================================
CREATE TABLE roles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    description  TEXT,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) UNIQUE NOT NULL,
    -- e.g. 'packages.create'
    module      VARCHAR(50) NOT NULL,
    -- e.g. 'packages', 'groups', 'bots'
    action      VARCHAR(50) NOT NULL,
    -- e.g. 'create', 'read', 'update', 'delete'
    description TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMP
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
    -- Hapus deleted_at: junction table, gunakan hard delete
);

-- ==========================================
-- 5. TELEGRAM BOTS (White-label)
-- ==========================================
CREATE TABLE telegram_bots (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    token      TEXT NOT NULL,
    -- Encrypted AES-256, JANGAN simpan plaintext
    username   VARCHAR(255),
    bot_id     BIGINT,
    -- Telegram bot's numeric ID (dari getMe)
    is_active  BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- ==========================================
-- 6. GROUPS (Grup Telegram yang Dikelola)
-- ==========================================
CREATE TABLE groups (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id        UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    bot_id           UUID NOT NULL REFERENCES telegram_bots(id),
    telegram_chat_id BIGINT NOT NULL,
    name             VARCHAR(255),
    description      TEXT,
    member_count     INT DEFAULT 0,
    -- Di-update periodik, bukan realtime
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMP,

    -- Satu chat ID unik per client (bisa satu grup dikelola banyak client? Tidak.)
    UNIQUE(telegram_chat_id)
);

-- ==========================================
-- 7. PACKAGES (Paket Langganan)
-- ==========================================
CREATE TABLE packages (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id      UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    price          DECIMAL(12, 2) NOT NULL,
    duration_days  INT NOT NULL,
    is_all_access  BOOLEAN DEFAULT FALSE,
    -- true = akses semua grup milik client ini
    is_active      BOOLEAN DEFAULT TRUE,
    created_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMP
);

-- ==========================================
-- 8. PACKAGE GROUPS (Many-to-Many: Packages ↔ Groups)
-- ==========================================
CREATE TABLE package_groups (
    package_id UUID NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (package_id, group_id)
    -- Hard delete: kalau relasi berubah, hapus & buat ulang
);

-- ==========================================
-- 9. TELEGRAM USERS (Member)
-- ==========================================
CREATE TABLE telegram_users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_user_id BIGINT UNIQUE NOT NULL,
    username         VARCHAR(255),
    first_name       VARCHAR(255),
    last_name        VARCHAR(255),
    phone            VARCHAR(50),
    -- Bisa didapat jika user pernah contact bot
    created_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMP
);

-- ==========================================
-- 10. SUBSCRIPTIONS
-- ==========================================
CREATE TABLE subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Relasi ke Member & Paket
    telegram_user_id    UUID NOT NULL REFERENCES telegram_users(id),
    package_id          UUID NOT NULL REFERENCES packages(id),
    
    -- TAMBAHAN: client_id untuk isolasi multi-tenant di worker
    client_id           UUID NOT NULL REFERENCES clients(id),
    
    -- TAMBAHAN: order_id untuk traceability (payment -> subscription)
    order_id            UUID,
    -- FK ke orders ditambah setelah tabel orders dibuat

    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    -- 'active', 'expired', 'cancelled'
    
    activated_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    expired_at          TIMESTAMP NOT NULL,
    
    -- Stacking: kalau beli lagi, expired_at += duration_days
    -- Parallel: bisa ada multiple rows aktif untuk package berbeda
    
    auto_renew          BOOLEAN DEFAULT FALSE,
    grace_period_hours  INT NOT NULL DEFAULT 0,
    kicked_at           TIMESTAMP,
    -- Kapan bot mengeksekusi kick
    last_checked_at     TIMESTAMP NOT NULL DEFAULT NOW(),

    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMP
);

-- ==========================================
-- 11. ORDERS (Transaksi Pembayaran)
-- ==========================================
CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Relasi
    telegram_user_id UUID NOT NULL REFERENCES telegram_users(id),
    package_id      UUID NOT NULL REFERENCES packages(id),
    client_id       UUID NOT NULL REFERENCES clients(id),
    -- TAMBAHAN: untuk multi-tenant isolation
    subscription_id UUID REFERENCES subscriptions(id),
    -- NULL saat pending, diisi setelah payment sukses
    
    -- Payment Gateway
    external_id     VARCHAR(255) UNIQUE NOT NULL,
    -- ID dari Midtrans/Xendit, untuk idempotency
    payment_url     VARCHAR(500),
    -- URL redirect ke payment page
    amount          DECIMAL(12, 2) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 'pending', 'paid', 'failed', 'expired'
    payment_method  VARCHAR(50),
    -- 'bca_va', 'gopay', 'qris', etc
    
    paid_at         TIMESTAMP,
    expired_at      TIMESTAMP,
    -- Kapan payment link expired
    
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

-- Sekarang bisa tambah FK dari subscriptions ke orders
ALTER TABLE subscriptions
    ADD CONSTRAINT fk_subscriptions_order
    FOREIGN KEY (order_id) REFERENCES orders(id);

-- ==========================================
-- 12. OUTBOX (Atomic Event Publishing)
-- ==========================================
-- Pola Outbox untuk menjamin atomicity antara:
-- DB Transaction (update subscription) dan Event Publishing (RabbitMQ)
CREATE TABLE outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type  VARCHAR(100) NOT NULL,
    -- e.g. 'subscription', 'order'
    aggregate_id    UUID NOT NULL,
    -- ID entity terkait
    event_type      VARCHAR(100) NOT NULL,
    -- e.g. 'subscription.activated', 'member.kick'
    payload         JSONB NOT NULL,
    -- Data yang akan dipublish ke RabbitMQ
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    -- 'pending', 'processed', 'failed'
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    last_error      TEXT,
    -- Error message terakhir jika gagal
    process_after   TIMESTAMP NOT NULL DEFAULT NOW(),
    -- Untuk delay/backoff
    processed_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
    -- Tidak perlu deleted_at, outbox di-cleanup setelah processed
);

-- ==========================================
-- 13. AUDIT LOGS
-- ==========================================
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   UUID REFERENCES clients(id),
    -- TAMBAHAN: untuk filter audit per tenant
    entity_type VARCHAR(50) NOT NULL,
    -- 'subscription', 'order', 'join_request', 'member'
    entity_id   UUID NOT NULL,
    action      VARCHAR(50) NOT NULL,
    -- 'approved', 'declined', 'kicked', 'payment_received'
    actor_type  VARCHAR(50),
    -- 'bot', 'system', 'admin', 'user'
    actor_id    VARCHAR(255),
    -- UUID atau Telegram ID tergantung actor_type
    metadata    JSONB,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
    -- Audit log tidak perlu updated_at/deleted_at: immutable record
);

-- ==========================================
-- INDEXES
-- ==========================================

-- [Client Users]
CREATE INDEX idx_client_users_client ON client_users(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_client_users_user ON client_users(user_id) WHERE deleted_at IS NULL;

-- [Telegram Bots]
CREATE INDEX idx_telegram_bots_client ON telegram_bots(client_id) WHERE deleted_at IS NULL AND is_active = TRUE;

-- [Groups]
CREATE INDEX idx_groups_client ON groups(client_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_groups_bot ON groups(bot_id) WHERE deleted_at IS NULL;

-- [Packages]
CREATE INDEX idx_packages_client ON packages(client_id) WHERE deleted_at IS NULL AND is_active = TRUE;

-- [Telegram Users]
CREATE INDEX idx_telegram_users_tg_id ON telegram_users(telegram_user_id) USING HASH;

-- [Subscriptions] — CRITICAL untuk Worker & Gatekeeping
-- Partial index: hanya row aktif yang masuk index (hemat storage)
CREATE INDEX idx_subscriptions_expired_worker
    ON subscriptions(expired_at, client_id)
    WHERE status = 'active' AND deleted_at IS NULL;

-- Untuk Gatekeeping: cek apakah user punya sub aktif di package tertentu
CREATE INDEX idx_subscriptions_user_package
    ON subscriptions(telegram_user_id, package_id)
    WHERE status = 'active' AND deleted_at IS NULL;

-- Untuk dashboard: list semua sub per client
CREATE INDEX idx_subscriptions_client
    ON subscriptions(client_id, status)
    WHERE deleted_at IS NULL;

-- [Orders]
CREATE INDEX idx_orders_external_id ON orders(external_id);
-- Sudah UNIQUE, tapi eksplisit untuk clarity
CREATE INDEX idx_orders_client_status ON orders(client_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_orders_telegram_user ON orders(telegram_user_id) WHERE deleted_at IS NULL;

-- [Outbox] — CRITICAL untuk reliabilitas event publishing
CREATE INDEX idx_outbox_pending
    ON outbox(process_after, retry_count)
    WHERE status = 'pending';
-- Worker poll: "ambil outbox yang belum diproses dan sudah waktunya"

-- [Audit Logs]
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_logs_client ON audit_logs(client_id, created_at DESC);


-- ==========================================
-- PLATFORM ADMIN LAYER
-- ==========================================

-- Admin Users (Platform Operator / Superadmin)
-- Tabel ini TERPISAH dari users (tenant dashboard users)
CREATE TABLE admin_users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email               VARCHAR(255) UNIQUE NOT NULL,
    name                VARCHAR(255) NOT NULL,
    password_hash       VARCHAR(255) NOT NULL,
    
    -- Security: Admin butuh proteksi ekstra
    is_active           BOOLEAN DEFAULT true,
    is_two_fa_enabled   BOOLEAN DEFAULT false,
    two_fa_secret       TEXT,              -- Encrypted TOTP secret
    
    -- Audit
    last_login_at       TIMESTAMP,
    last_login_ip       VARCHAR(45),       -- Support IPv6
    failed_login_count  INT DEFAULT 0,
    locked_until        TIMESTAMP,         -- Lockout setelah brute force
    
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMP
);

-- Role untuk Admin (berbeda dari tenant roles)
-- Contoh: 'superadmin', 'support', 'finance', 'ops'
CREATE TABLE admin_roles (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    description  TEXT,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMP
);

-- Permission granular untuk admin
-- Contoh: 'clients.suspend', 'clients.impersonate', 
--         'platform.analytics', 'billing.manage'
CREATE TABLE admin_permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) UNIQUE NOT NULL,
    module      VARCHAR(50) NOT NULL,
    -- 'clients', 'billing', 'platform', 'support'
    action      VARCHAR(50) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMP
);

CREATE TABLE admin_role_permissions (
    admin_role_id       UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    admin_permission_id UUID NOT NULL REFERENCES admin_permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (admin_role_id, admin_permission_id)
);

-- Mapping admin ke role (satu admin bisa punya beberapa role)
CREATE TABLE admin_user_roles (
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    admin_role_id UUID NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    assigned_by   UUID REFERENCES admin_users(id),
    assigned_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (admin_user_id, admin_role_id)
);

-- ==========================================
-- PLATFORM BILLING (Subscription Tier Clients)
-- ==========================================

-- Definisi tier yang bisa diatur superadmin
CREATE TABLE platform_plans (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(50) UNIQUE NOT NULL,
    -- 'free', 'basic', 'pro', 'enterprise'
    display_name        VARCHAR(100) NOT NULL,
    price_monthly       DECIMAL(12, 2) NOT NULL DEFAULT 0,
    price_yearly        DECIMAL(12, 2) NOT NULL DEFAULT 0,

    -- Limits per tier
    max_bots            INT NOT NULL DEFAULT 1,
    max_groups          INT NOT NULL DEFAULT 1,
    max_packages        INT NOT NULL DEFAULT 3,
    max_members         INT NOT NULL DEFAULT 100,
    -- -1 = unlimited

    -- Feature flags
    features            JSONB NOT NULL DEFAULT '{}',
    -- {"white_label": true, "api_access": false, "custom_domain": true}

    is_active           BOOLEAN DEFAULT true,
    is_landing_page     BOOLEAN DEFAULT true,
    created_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMP
);

-- Riwayat billing client ke platform (B2B)
CREATE TABLE client_billings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       UUID NOT NULL REFERENCES clients(id),
    plan_id         UUID NOT NULL REFERENCES platform_plans(id),
    
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    -- 'active', 'cancelled', 'past_due', 'trialing'
    billing_cycle   VARCHAR(20) NOT NULL DEFAULT 'monthly',
    -- 'monthly', 'yearly'
    
    amount          DECIMAL(12, 2) NOT NULL,
    started_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    expired_at      TIMESTAMP NOT NULL,
    cancelled_at    TIMESTAMP,

    external_id     VARCHAR(255),
    payment_url     VARCHAR(500),
    paid_at         TIMESTAMP,
    
    -- Superadmin bisa override (kasih free plan manual, dll)
    is_manual       BOOLEAN DEFAULT false,
    note            TEXT,
    created_by      UUID,
    -- Catatan dari superadmin

    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ==========================================
-- IMPERSONATION AUDIT (Security Critical)
-- ==========================================

-- Setiap kali superadmin masuk sebagai client, WAJIB dicatat
CREATE TABLE admin_impersonation_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id   UUID NOT NULL REFERENCES admin_users(id),
    client_id       UUID NOT NULL REFERENCES clients(id),
    target_user_id  UUID REFERENCES users(id),
    -- Kalau impersonate sebagai specific user dalam client
    reason          TEXT,
    -- Wajib isi alasan (untuk compliance)
    ip_address      VARCHAR(45),
    started_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at        TIMESTAMP
);

-- ==========================================
-- INDEXES TAMBAHAN
-- ==========================================

-- Admin Users
CREATE INDEX idx_admin_users_email ON admin_users(email) WHERE deleted_at IS NULL;

-- Client Billings
CREATE INDEX idx_client_billings_client 
    ON client_billings(client_id, status);
CREATE UNIQUE INDEX idx_client_billings_external_id
    ON client_billings(external_id);
CREATE INDEX idx_client_billings_expired 
    ON client_billings(expired_at) 
    WHERE status = 'active';
-- Worker bisa cek client mana yang billing-nya mau expired

-- Impersonation Logs
CREATE INDEX idx_impersonation_admin 
    ON admin_impersonation_logs(admin_user_id, started_at DESC);
    ON admin_impersonation_logs(client_id, started_at DESC);

-- ==========================================
-- LAYER 1: Platform Discount (Superadmin → Client)
-- ==========================================
CREATE TABLE platform_discounts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  VARCHAR(255) NOT NULL,
    code                  VARCHAR(100) UNIQUE,          -- NULL = otomatis (tanpa kode)
    type                  VARCHAR(20) NOT NULL,          -- 'percentage' | 'fixed'
    value                 DECIMAL(12,2) NOT NULL,        -- 20 = 20% atau 50000 = Rp50.000
    max_discount          DECIMAL(12,2),                 -- Cap nominal, khusus type=percentage
    min_purchase          DECIMAL(12,2) NOT NULL DEFAULT 0,

    -- Scope
    max_usage             INT NOT NULL DEFAULT -1,       -- -1 = unlimited
    used_count            INT NOT NULL DEFAULT 0,
    applicable_plan_ids   UUID[],                        -- NULL = semua plan
    applicable_client_ids UUID[],                        -- NULL = semua client

    valid_from            TIMESTAMP NOT NULL DEFAULT NOW(),
    valid_until           TIMESTAMP,                     -- NULL = tidak ada expiry
    is_active             BOOLEAN NOT NULL DEFAULT true,

    created_by            UUID REFERENCES admin_users(id),
    created_at            TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMP
);

-- Relasi billing ke discount yang dipakai
ALTER TABLE client_billings
    ADD COLUMN discount_id UUID REFERENCES platform_discounts(id),
    ADD COLUMN discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    ADD COLUMN original_amount DECIMAL(12,2);

-- ==========================================
-- LAYER 2: Member Discount (Client → Telegram User)
-- ==========================================
CREATE TABLE member_discounts (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id               UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name                    VARCHAR(255) NOT NULL,
    code                    VARCHAR(100),                -- NULL = otomatis (tanpa kode)
    type                    VARCHAR(20) NOT NULL,        -- 'percentage' | 'fixed'
    value                   DECIMAL(12,2) NOT NULL,
    max_discount            DECIMAL(12,2),               -- Cap nominal, khusus percentage
    min_purchase            DECIMAL(12,2) NOT NULL DEFAULT 0,

    -- Scope & Limits
    max_usage               INT NOT NULL DEFAULT -1,     -- -1 = unlimited
    used_count              INT NOT NULL DEFAULT 0,
    max_usage_per_user      INT NOT NULL DEFAULT 1,      -- Berapa kali per member
    applicable_package_ids  UUID[],                      -- NULL = semua paket client

    valid_from              TIMESTAMP NOT NULL DEFAULT NOW(),
    valid_until             TIMESTAMP,
    is_active               BOOLEAN NOT NULL DEFAULT true,

    created_at              TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at              TIMESTAMP,

    -- Satu client tidak boleh punya kode yang sama
    UNIQUE(client_id, code)
);

-- Track usage per user untuk max_usage_per_user enforcement
CREATE TABLE member_discount_usages (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discount_id         UUID NOT NULL REFERENCES member_discounts(id),
    telegram_user_id    UUID NOT NULL REFERENCES telegram_users(id),
    order_id            UUID NOT NULL REFERENCES orders(id),
    discount_amount     DECIMAL(12,2) NOT NULL,
    used_at             TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(discount_id, order_id)  -- Satu order hanya bisa pakai satu diskon sekali
);

-- Relasi orders ke discount
ALTER TABLE orders
    ADD COLUMN discount_id UUID REFERENCES member_discounts(id),
    ADD COLUMN discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    ADD COLUMN original_amount DECIMAL(12,2);

-- ==========================================
-- INDEXES
-- ==========================================
CREATE INDEX idx_platform_discounts_code
    ON platform_discounts(code) WHERE deleted_at IS NULL AND is_active = true;

CREATE INDEX idx_platform_discounts_active
    ON platform_discounts(valid_from, valid_until) WHERE is_active = true AND deleted_at IS NULL;

CREATE INDEX idx_member_discounts_client
    ON member_discounts(client_id) WHERE deleted_at IS NULL AND is_active = true;

CREATE INDEX idx_member_discounts_code
    ON member_discounts(client_id, code) WHERE deleted_at IS NULL AND is_active = true;

CREATE INDEX idx_member_discount_usages_user
    ON member_discount_usages(discount_id, telegram_user_id);

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
    status          VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    total_targets   INT NOT NULL DEFAULT 0,
    sent_count      INT NOT NULL DEFAULT 0,
    failed_count    INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP
);

CREATE INDEX idx_broadcasts_client_bot ON broadcasts(client_id, bot_id);