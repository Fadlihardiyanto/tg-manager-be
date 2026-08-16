-- ==========================================
-- TENANT RBAC SEEDER
-- Inserts default roles, permissions, and role-permission mappings.
-- 
-- Run this AFTER table.sql has been applied.
-- Safe to re-run (uses ON CONFLICT DO NOTHING).
-- ==========================================

-- ==========================================
-- 1. ROLES
-- ==========================================
INSERT INTO roles (id, name, display_name, description) VALUES
  (gen_random_uuid(), 'owner',   'Owner',   'Pemilik bisnis. Memiliki akses penuh ke semua fitur tenant.'),
  (gen_random_uuid(), 'admin',   'Admin',   'Administrator tenant. Bisa mengelola bot, grup, paket, dan melihat analytics.'),
  (gen_random_uuid(), 'manager', 'Manager', 'Manajer. Bisa mengelola grup dan paket, tapi tidak bisa mengelola bot dan tim.'),
  (gen_random_uuid(), 'viewer',  'Viewer',  'Hanya bisa melihat data. Tidak bisa melakukan perubahan apapun.')
ON CONFLICT (name) DO NOTHING;

-- ==========================================
-- 2. PERMISSIONS
-- ==========================================

-- Bot Management
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'bots.create', 'bots', 'create', 'Menambahkan bot Telegram baru'),
  (gen_random_uuid(), 'bots.read',   'bots', 'read',   'Melihat daftar dan detail bot'),
  (gen_random_uuid(), 'bots.update', 'bots', 'update', 'Mengubah konfigurasi bot'),
  (gen_random_uuid(), 'bots.delete', 'bots', 'delete', 'Menghapus bot')
ON CONFLICT (name) DO NOTHING;

-- Group Management
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'groups.create', 'groups', 'create', 'Mendaftarkan grup Telegram baru'),
  (gen_random_uuid(), 'groups.read',   'groups', 'read',   'Melihat daftar dan detail grup'),
  (gen_random_uuid(), 'groups.update', 'groups', 'update', 'Mengubah konfigurasi grup'),
  (gen_random_uuid(), 'groups.delete', 'groups', 'delete', 'Menghapus grup')
ON CONFLICT (name) DO NOTHING;

-- Package Management
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'packages.create', 'packages', 'create', 'Membuat paket langganan baru'),
  (gen_random_uuid(), 'packages.read',   'packages', 'read',   'Melihat daftar dan detail paket'),
  (gen_random_uuid(), 'packages.update', 'packages', 'update', 'Mengubah paket langganan'),
  (gen_random_uuid(), 'packages.delete', 'packages', 'delete', 'Menghapus paket langganan')
ON CONFLICT (name) DO NOTHING;

-- Order & Subscription
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'orders.read',          'orders', 'read',   'Melihat daftar dan detail order'),
  (gen_random_uuid(), 'subscriptions.read',   'subscriptions', 'read',   'Melihat daftar dan detail subscription'),
  (gen_random_uuid(), 'subscriptions.cancel', 'subscriptions', 'cancel', 'Membatalkan subscription aktif')
ON CONFLICT (name) DO NOTHING;

-- Team Management
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'team.invite',  'team', 'invite',  'Mengundang anggota tim baru'),
  (gen_random_uuid(), 'team.read',    'team', 'read',    'Melihat daftar anggota tim'),
  (gen_random_uuid(), 'team.update',  'team', 'update',  'Mengubah role anggota tim'),
  (gen_random_uuid(), 'team.remove',  'team', 'remove',  'Menghapus anggota tim')
ON CONFLICT (name) DO NOTHING;

-- Analytics & Dashboard
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'analytics.read', 'analytics', 'read', 'Melihat dashboard analytics')
ON CONFLICT (name) DO NOTHING;

-- Discounts
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'discounts.create', 'discounts', 'create', 'Membuat diskon baru'),
  (gen_random_uuid(), 'discounts.read',   'discounts', 'read',   'Melihat daftar dan detail diskon'),
  (gen_random_uuid(), 'discounts.update', 'discounts', 'update', 'Mengubah diskon'),
  (gen_random_uuid(), 'discounts.delete', 'discounts', 'delete', 'Menghapus diskon')
ON CONFLICT (name) DO NOTHING;

-- Member Management (telegram_users + subscriptions view)
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'members.read', 'members', 'read', 'Melihat daftar dan detail member (Telegram user + subscription)')
ON CONFLICT (name) DO NOTHING;

-- Daily Report (laporan harian worker + daftar kegagalan)
INSERT INTO permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'reports.read',   'reports', 'read',   'Melihat pengaturan dan daftar kegagalan laporan harian'),
  (gen_random_uuid(), 'reports.update', 'reports', 'update', 'Mengubah pengaturan laporan harian')
ON CONFLICT (name) DO NOTHING;

-- ==========================================
-- 3. ROLE-PERMISSION ASSIGNMENTS
-- ==========================================

-- ADMIN role — semua kecuali team management (hanya owner yang bisa manage tim)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin'
  AND p.name IN (
    'bots.create', 'bots.read', 'bots.update', 'bots.delete',
    'groups.create', 'groups.read', 'groups.update', 'groups.delete',
    'packages.create', 'packages.read', 'packages.update', 'packages.delete',
    'orders.read',
    'subscriptions.read', 'subscriptions.cancel',
    'team.read',
    'analytics.read',
    'discounts.create', 'discounts.read', 'discounts.update', 'discounts.delete',
    'members.read',
    'reports.read',
    'reports.update',
  )
ON CONFLICT DO NOTHING;

-- MANAGER role — kelola grup, paket, lihat order/subscription, analytics
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'manager'
  AND p.name IN (
    'bots.read',
    'groups.create', 'groups.read', 'groups.update',
    'packages.create', 'packages.read', 'packages.update',
    'orders.read',
    'subscriptions.read',
    'analytics.read',
    'discounts.read',
    'members.read',
    'reports.read',
  )
ON CONFLICT DO NOTHING;

-- VIEWER role — read-only semua
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'viewer'
  AND p.name IN (
    'bots.read',
    'groups.read',
    'packages.read',
    'orders.read',
    'subscriptions.read',
    'team.read',
    'analytics.read',
    'discounts.read',
    'members.read',
    'reports.read',
  )
ON CONFLICT DO NOTHING;

-- NOTE: "owner" role does NOT need entries in role_permissions
-- because owner bypasses all permission checks in middleware (like superadmin).
-- This is by design — see tenant_auth_middleware.go.
