-- ==========================================
-- ADMIN RBAC SEEDER
-- Inserts default admin roles, permissions, and role-permission mappings.
-- 
-- Run this AFTER table.sql has been applied.
-- Safe to re-run (uses ON CONFLICT DO NOTHING).
-- ==========================================

-- ==========================================
-- 1. ROLES
-- ==========================================
INSERT INTO admin_roles (id, name, display_name, description) VALUES
  (gen_random_uuid(), 'superadmin', 'Superadmin', 'Akses penuh ke seluruh platform. Bypass semua permission check.'),
  (gen_random_uuid(), 'support',    'Support',    'Customer support. Bisa melihat data client dan impersonate.'),
  (gen_random_uuid(), 'finance',    'Finance',    'Tim keuangan. Bisa mengelola billing, plan, dan diskon.'),
  (gen_random_uuid(), 'ops',        'Ops',        'Tim operasional. Bisa mengelola admin, client, roles, dan billing.')
ON CONFLICT (name) DO NOTHING;

-- ==========================================
-- 2. PERMISSIONS
-- ==========================================

-- Role & Permission Management
INSERT INTO admin_permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'roles.read',   'roles', 'read',   'Melihat daftar dan detail admin role'),
  (gen_random_uuid(), 'roles.create', 'roles', 'create', 'Membuat admin role baru'),
  (gen_random_uuid(), 'roles.update', 'roles', 'update', 'Mengubah admin role dan sync permissions'),
  (gen_random_uuid(), 'roles.delete', 'roles', 'delete', 'Menghapus admin role')
ON CONFLICT (name) DO NOTHING;

-- Admin User Management
INSERT INTO admin_permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'admins.read',   'admins', 'read',   'Melihat daftar dan detail admin user'),
  (gen_random_uuid(), 'admins.create', 'admins', 'create', 'Membuat admin user baru'),
  (gen_random_uuid(), 'admins.update', 'admins', 'update', 'Mengubah admin user, activate/deactivate, sync roles'),
  (gen_random_uuid(), 'admins.delete', 'admins', 'delete', 'Menghapus admin user')
ON CONFLICT (name) DO NOTHING;

-- Client (Tenant) Management
INSERT INTO admin_permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'clients.read',        'clients', 'read',        'Melihat daftar dan detail client'),
  (gen_random_uuid(), 'clients.create',      'clients', 'create',      'Membuat client / tenant baru'),
  (gen_random_uuid(), 'clients.update',      'clients', 'update',      'Mengubah client, kelola user client, activate/deactivate'),
  (gen_random_uuid(), 'clients.delete',      'clients', 'delete',      'Menghapus client'),
  (gen_random_uuid(), 'clients.impersonate', 'clients', 'impersonate', 'Login sebagai client (impersonation)')
ON CONFLICT (name) DO NOTHING;

-- Platform Billing
INSERT INTO admin_permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'billing.read',   'billing', 'read',   'Melihat daftar plan, billing client, dan diskon'),
  (gen_random_uuid(), 'billing.manage', 'billing', 'manage', 'Mengelola plan, assign/cancel billing, dan kelola diskon')
ON CONFLICT (name) DO NOTHING;

-- Analytics & Audit
INSERT INTO admin_permissions (id, name, module, action, description) VALUES
  (gen_random_uuid(), 'analytics.read', 'analytics', 'read', 'Melihat audit log dan platform analytics')
ON CONFLICT (name) DO NOTHING;

-- ==========================================
-- 3. ROLE-PERMISSION ASSIGNMENTS
-- ==========================================

-- SUPPORT role — read clients, impersonate, read analytics
INSERT INTO admin_role_permissions (admin_role_id, admin_permission_id)
SELECT r.id, p.id
FROM admin_roles r
CROSS JOIN admin_permissions p
WHERE r.name = 'support'
  AND p.name IN (
    'clients.read',
    'clients.impersonate',
    'analytics.read',
    'billing.read'
  )
ON CONFLICT DO NOTHING;

-- FINANCE role — billing + read clients
INSERT INTO admin_role_permissions (admin_role_id, admin_permission_id)
SELECT r.id, p.id
FROM admin_roles r
CROSS JOIN admin_permissions p
WHERE r.name = 'finance'
  AND p.name IN (
    'clients.read',
    'billing.read',
    'billing.manage',
    'analytics.read'
  )
ON CONFLICT DO NOTHING;

-- OPS role — all permissions except superadmin-only actions
INSERT INTO admin_role_permissions (admin_role_id, admin_permission_id)
SELECT r.id, p.id
FROM admin_roles r
CROSS JOIN admin_permissions p
WHERE r.name = 'ops'
  AND p.name IN (
    'roles.read', 'roles.create', 'roles.update', 'roles.delete',
    'admins.read', 'admins.create', 'admins.update', 'admins.delete',
    'clients.read', 'clients.create', 'clients.update', 'clients.delete', 'clients.impersonate',
    'billing.read', 'billing.manage',
    'analytics.read'
  )
ON CONFLICT DO NOTHING;

-- NOTE: "superadmin" role does NOT need entries in admin_role_permissions
-- because superadmin bypasses all permission checks in middleware (rbac.IsSuperAdmin).
-- This is by design — see admin_auth_middleware.go and authorize.go.

-- ==========================================
-- 4. FIRST SUPERADMIN ASSIGNMENT
-- ==========================================
-- Setelah admin user pertama dibuat (via register API), jalankan query ini
-- untuk memberikan role superadmin. Ganti 'ADMIN_USER_ID' dengan UUID admin.
--
--   INSERT INTO admin_user_roles (admin_user_id, admin_role_id)
--   SELECT 'ADMIN_USER_ID', r.id
--   FROM admin_roles r
--   WHERE r.name = 'superadmin'
--   ON CONFLICT DO NOTHING;
--
-- Atau untuk otomatis (assign superadmin ke admin pertama yang ada):
--
--   INSERT INTO admin_user_roles (admin_user_id, admin_role_id)
--   SELECT u.id, r.id
--   FROM admin_users u, admin_roles r
--   WHERE r.name = 'superadmin'
--     AND u.deleted_at IS NULL
--     AND NOT EXISTS (
--       SELECT 1 FROM admin_user_roles aur WHERE aur.admin_user_id = u.id
--     )
--   ORDER BY u.created_at ASC
--   LIMIT 1
--   ON CONFLICT DO NOTHING;
