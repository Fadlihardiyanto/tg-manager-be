package pkg_jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// =============================================================================
// Configuration
// =============================================================================

// JWTConfig holds JWT signing and expiry configuration.
// Admin and Tenant use SEPARATE secrets for security isolation.
type JWTConfig struct {
	SecretKey     string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	Issuer        string
}

// TokenType distinguishes between admin and tenant JWTs at parse time.
type TokenType string

const (
	TokenTypeAdmin  TokenType = "admin"
	TokenTypeTenant TokenType = "tenant"
)

// =============================================================================
// Claims — Admin (Platform Operator / Superadmin)
// =============================================================================

// AdminClaims is the JWT payload for platform admin users.
// Permissions are embedded so middleware can check without a DB hit.
type AdminClaims struct {
	AdminID     uuid.UUID `json:"admin_id"`
	Email       string    `json:"email"`
	Type        TokenType `json:"type"` // always "admin"
	Roles       []string  `json:"roles,omitempty"`
	Permissions []string  `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// GenerateAdminTokens creates an access + refresh token pair for an admin user.
func GenerateAdminTokens(ctx context.Context, adminID uuid.UUID, email string, roles, permissions []string, config *JWTConfig) (accessToken, refreshToken, refreshJTI string, err error) {
	now := time.Now()
	accessJTI := uuid.New().String()

	accessClaims := AdminClaims{
		AdminID:     adminID,
		Email:       email,
		Type:        TokenTypeAdmin,
		Permissions: permissions,
		Roles:       roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessJTI,
			Subject:   adminID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(config.AccessExpiry)),
			Issuer:    config.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			Audience:  []string{"admin"},
		},
	}

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = at.SignedString([]byte(config.SecretKey))
	if err != nil {
		return "", "", "", fmt.Errorf("sign admin access token: %w", err)
	}

	// Refresh token — minimal claims, audience = "refresh"
	refreshJTI = uuid.New().String()
	refreshClaims := jwt.RegisteredClaims{
		ID:        refreshJTI,
		Subject:   adminID.String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(config.RefreshExpiry)),
		Issuer:    config.Issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Audience:  []string{"admin", "refresh"},
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = rt.SignedString([]byte(config.SecretKey))
	if err != nil {
		return "", "", "", fmt.Errorf("sign admin refresh token: %w", err)
	}

	return accessToken, refreshToken, refreshJTI, nil
}

// ParseAdminToken validates and extracts AdminClaims from a token string.
func ParseAdminToken(tokenString, secretKey string) (*AdminClaims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &AdminClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*AdminClaims)
	if !ok || !parsed.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if claims.Type != TokenTypeAdmin {
		return nil, fmt.Errorf("expected token type %q, got %q", TokenTypeAdmin, claims.Type)
	}
	return claims, nil
}

// =============================================================================
// Claims — Tenant (Client Dashboard Users)
// =============================================================================

// TenantClaims is the JWT payload for tenant dashboard users.
// ClientID scopes all data access to a single tenant.
// Permissions are embedded so middleware can check without a DB hit.
type TenantClaims struct {
	UserID      uuid.UUID `json:"user_id"`
	ClientID    uuid.UUID `json:"client_id"`
	Role        string    `json:"role"`        // 'owner', 'admin', 'manager', 'viewer'
	Permissions []string  `json:"permissions"` // flat list e.g. ["packages.create", "bots.read"]
	Type        TokenType `json:"type"`        // always "tenant"
	jwt.RegisteredClaims
}

// GenerateTenantTokens creates an access + refresh token pair for a tenant user.
func GenerateTenantTokens(ctx context.Context, userID, clientID uuid.UUID, role string, permissions []string, config *JWTConfig) (accessToken, refreshToken, refreshJTI string, err error) {
	now := time.Now()
	accessJTI := uuid.New().String()

	accessClaims := TenantClaims{
		UserID:      userID,
		ClientID:    clientID,
		Role:        role,
		Permissions: permissions,
		Type:        TokenTypeTenant,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessJTI,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(config.AccessExpiry)),
			Issuer:    config.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			Audience:  []string{"tenant"},
		},
	}

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = at.SignedString([]byte(config.SecretKey))
	if err != nil {
		return "", "", "", fmt.Errorf("sign tenant access token: %w", err)
	}

	refreshJTI = uuid.New().String()
	refreshClaims := jwt.RegisteredClaims{
		ID:        refreshJTI,
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(config.RefreshExpiry)),
		Issuer:    config.Issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Audience:  []string{"tenant", "refresh"},
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = rt.SignedString([]byte(config.SecretKey))
	if err != nil {
		return "", "", "", fmt.Errorf("sign tenant refresh token: %w", err)
	}

	return accessToken, refreshToken, refreshJTI, nil
}

// ParseTenantToken validates and extracts TenantClaims from a token string.
func ParseTenantToken(tokenString, secretKey string) (*TenantClaims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &TenantClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*TenantClaims)
	if !ok || !parsed.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if claims.Type != TokenTypeTenant {
		return nil, fmt.Errorf("expected token type %q, got %q", TokenTypeTenant, claims.Type)
	}
	return claims, nil
}

// =============================================================================
// Impersonation Token
// =============================================================================

// ImpersonationClaims is issued when a superadmin impersonates a client.
// It carries both the admin identity (for audit) and the tenant context (for data access).
type ImpersonationClaims struct {
	// Admin who is impersonating
	AdminID uuid.UUID `json:"admin_id"`

	// Tenant context being impersonated
	ClientID     uuid.UUID `json:"client_id"`
	TargetUserID uuid.UUID `json:"target_user_id"`
	Role         string    `json:"role"` // typically "owner" for full access

	Type   TokenType `json:"type"`   // "tenant" — so existing tenant middleware works
	Source string    `json:"source"` // "impersonation" — to distinguish from real tenant tokens
	jwt.RegisteredClaims
}

// GenerateImpersonationToken creates a short-lived token for admin impersonation.
// The token is valid for the tenant API routes but is auditable back to the admin.
func GenerateImpersonationToken(ctx context.Context, adminID, clientID, targetUserID uuid.UUID, config *JWTConfig) (string, error) {
	now := time.Now()
	// Impersonation tokens are short-lived (max 1 hour regardless of config)
	expiry := config.AccessExpiry
	if expiry > 1*time.Hour {
		expiry = 1 * time.Hour
	}

	claims := ImpersonationClaims{
		AdminID:      adminID,
		ClientID:     clientID,
		TargetUserID: targetUserID,
		Role:         "owner",
		Type:         TokenTypeTenant,
		Source:       "impersonation",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   targetUserID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			Issuer:    config.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			Audience:  []string{"tenant", "impersonation"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.SecretKey))
	if err != nil {
		return "", fmt.Errorf("sign impersonation token: %w", err)
	}
	return tokenString, nil
}
