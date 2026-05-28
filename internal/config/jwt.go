package config

import (
	pkg_jwt "github.com/Fadlihardiyanto/telegram-management-app/pkg/jwt"
)

func NewJWTConfig(c *Config) *pkg_jwt.JWTConfig {
	return &pkg_jwt.JWTConfig{
		SecretKey:     c.JWT.AdminSecretKey,
		AccessExpiry:  c.JWT.AdminAccessExpiry,
		RefreshExpiry: c.JWT.AdminRefreshExpiry,
		Issuer:        c.App.Name,
	}
}
