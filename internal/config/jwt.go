package config

import (
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
)

func NewJWTConfig(c *Config) *pkg_jwt.JWTConfig {
	return &pkg_jwt.JWTConfig{
		// Admin
		AdminSecretKey:     c.JWT.AdminSecretKey,
		AdminAccessExpiry:  c.JWT.AdminAccessExpiry,
		AdminRefreshExpiry: c.JWT.AdminRefreshExpiry,

		// Tenant
		TenantSecretKey:     c.JWT.TenantSecretKey,
		TenantAccessExpiry:  c.JWT.TenantAccessExpiry,
		TenantRefreshExpiry: c.JWT.TenantRefreshExpiry,

		Issuer: c.App.Name,
	}
}
