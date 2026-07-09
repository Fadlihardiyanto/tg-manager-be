package pkg_jwt

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testConfig() *JWTConfig {
	return &JWTConfig{
		AdminSecretKey:      "admin-secret-key-for-testing-purposes-123",
		AdminAccessExpiry:   time.Hour,
		AdminRefreshExpiry:  24 * time.Hour,
		TenantSecretKey:     "tenant-secret-key-for-testing-purposes-456",
		TenantAccessExpiry:  time.Hour,
		TenantRefreshExpiry: 24 * time.Hour,
		Issuer:              "test-issuer",
	}
}

func TestGenerateAdminTokens_ParseAdminToken(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	adminID := uuid.New()
	email := "admin@test.com"
	roles := []string{"superadmin"}
	permissions := []string{"all", "packages.create"}

	access, refresh, refreshJTI, err := GenerateAdminTokens(ctx, adminID, email, roles, permissions, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}

	if access == "" {
		t.Fatal("access token should not be empty")
	}
	if refresh == "" {
		t.Fatal("refresh token should not be empty")
	}
	if refreshJTI == "" {
		t.Fatal("refresh JTI should not be empty")
	}

	claims, err := ParseAdminToken(access, cfg.AdminSecretKey)
	if err != nil {
		t.Fatalf("ParseAdminToken() error = %v", err)
	}

	if claims.AdminID != adminID {
		t.Errorf("AdminID = %v, want %v", claims.AdminID, adminID)
	}
	if claims.Email != email {
		t.Errorf("Email = %q, want %q", claims.Email, email)
	}
	if claims.Type != TokenTypeAdmin {
		t.Errorf("Type = %q, want %q", claims.Type, TokenTypeAdmin)
	}
	if !stringSliceEqual(claims.Roles, roles) {
		t.Errorf("Roles = %v, want %v", claims.Roles, roles)
	}
	if !stringSliceEqual(claims.Permissions, permissions) {
		t.Errorf("Permissions = %v, want %v", claims.Permissions, permissions)
	}
}

func TestParseAdminToken_WrongSecret(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	access, _, _, err := GenerateAdminTokens(ctx, uuid.New(), "a@b.com", nil, nil, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}

	_, err = ParseAdminToken(access, "wrong-secret-key-that-is-long-enough-for-sure-!!!")
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestParseAdminToken_ExpiredToken(t *testing.T) {
	cfg := testConfig()
	cfg.AdminAccessExpiry = -time.Hour // expired
	ctx := context.Background()

	access, _, _, err := GenerateAdminTokens(ctx, uuid.New(), "a@b.com", nil, nil, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}

	_, err = ParseAdminToken(access, cfg.AdminSecretKey)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestParseAdminToken_InvalidTokenString(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"malformed token", "not-a-jwt-token"},
		{"garbage", "header.payload.signature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAdminToken(tt.token, testConfig().AdminSecretKey)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseAdminRefreshToken_Subject(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	adminID := uuid.New()

	_, refresh, _, err := GenerateAdminTokens(ctx, adminID, "a@b.com", nil, nil, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}

	claims, err := ParseAdminRefreshToken(refresh, cfg.AdminSecretKey)
	if err != nil {
		t.Fatalf("ParseAdminRefreshToken() error = %v", err)
	}

	if claims.Subject != adminID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, adminID.String())
	}
}

func TestParseAdminRefreshToken_Expired(t *testing.T) {
	cfg := testConfig()
	cfg.AdminRefreshExpiry = -time.Hour
	ctx := context.Background()

	_, refresh, _, err := GenerateAdminTokens(ctx, uuid.New(), "a@b.com", nil, nil, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}

	_, err = ParseAdminRefreshToken(refresh, cfg.AdminSecretKey)
	if err == nil {
		t.Fatal("expected error for expired refresh token, got nil")
	}
}

func TestGenerateTenantTokens_ParseTenantToken(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	userID := uuid.New()
	clientID := uuid.New()
	role := "owner"
	permissions := []string{"bots.read", "bots.create"}

	access, refresh, refreshJTI, err := GenerateTenantTokens(ctx, userID, clientID, role, permissions, cfg)
	if err != nil {
		t.Fatalf("GenerateTenantTokens() error = %v", err)
	}

	if access == "" {
		t.Fatal("access token should not be empty")
	}
	if refresh == "" {
		t.Fatal("refresh token should not be empty")
	}
	if refreshJTI == "" {
		t.Fatal("refresh JTI should not be empty")
	}

	claims, err := ParseTenantToken(access, cfg.TenantSecretKey)
	if err != nil {
		t.Fatalf("ParseTenantToken() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.ClientID != clientID {
		t.Errorf("ClientID = %v, want %v", claims.ClientID, clientID)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.Type != TokenTypeTenant {
		t.Errorf("Type = %q, want %q", claims.Type, TokenTypeTenant)
	}
	if !stringSliceEqual(claims.Permissions, permissions) {
		t.Errorf("Permissions = %v, want %v", claims.Permissions, permissions)
	}
}

func TestParseTenantToken_WrongSecret(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	access, _, _, err := GenerateTenantTokens(ctx, uuid.New(), uuid.New(), "owner", nil, cfg)
	if err != nil {
		t.Fatalf("GenerateTenantTokens() error = %v", err)
	}

	_, err = ParseTenantToken(access, cfg.AdminSecretKey) // wrong secret
	if err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestParseTenantToken_Expired(t *testing.T) {
	cfg := testConfig()
	cfg.TenantAccessExpiry = -time.Hour
	ctx := context.Background()

	access, _, _, err := GenerateTenantTokens(ctx, uuid.New(), uuid.New(), "owner", nil, cfg)
	if err != nil {
		t.Fatalf("GenerateTenantTokens() error = %v", err)
	}

	_, err = ParseTenantToken(access, cfg.TenantSecretKey)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestTokenTypeMismatch(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	adminAccess, _, _, err := GenerateAdminTokens(ctx, uuid.New(), "a@b.com", nil, nil, cfg)
	if err != nil {
		t.Fatalf("GenerateAdminTokens() error = %v", err)
	}

	_, err = ParseTenantToken(adminAccess, cfg.TenantSecretKey)
	if err == nil {
		t.Fatal("expected error when parsing admin token as tenant token, got nil")
	}
}

func TestParseTenantRefreshToken_Subject(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	userID := uuid.New()

	_, refresh, _, err := GenerateTenantTokens(ctx, userID, uuid.New(), "owner", nil, cfg)
	if err != nil {
		t.Fatalf("GenerateTenantTokens() error = %v", err)
	}

	claims, err := ParseTenantRefreshToken(refresh, cfg.TenantSecretKey)
	if err != nil {
		t.Fatalf("ParseTenantRefreshToken() error = %v", err)
	}

	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
}

func TestGenerateImpersonationToken(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()
	adminID := uuid.New()
	clientID := uuid.New()
	targetUserID := uuid.New()

	token, err := GenerateImpersonationToken(ctx, adminID, clientID, targetUserID, cfg)
	if err != nil {
		t.Fatalf("GenerateImpersonationToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("token should not be empty")
	}

	// impersonation token is signed with tenant secret, parse as tenant token
	claims, err := ParseTenantToken(token, cfg.TenantSecretKey)
	if err != nil {
		t.Fatalf("ParseTenantToken() error = %v", err)
	}

	// impersonation claims use Subject for targetUserID
	if claims.Subject != targetUserID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, targetUserID.String())
	}
	if claims.ClientID != clientID {
		t.Errorf("ClientID = %v, want %v", claims.ClientID, clientID)
	}
	if claims.Type != TokenTypeTenant {
		t.Errorf("Type = %q, want %q", claims.Type, TokenTypeTenant)
	}
}

func TestGenerateImpersonationToken_ExpiryCappedAt1Hour(t *testing.T) {
	cfg := testConfig()
	cfg.TenantAccessExpiry = 24 * time.Hour // > 1 hour
	ctx := context.Background()

	token, err := GenerateImpersonationToken(ctx, uuid.New(), uuid.New(), uuid.New(), cfg)
	if err != nil {
		t.Fatalf("GenerateImpersonationToken() error = %v", err)
	}

	claims, err := ParseTenantToken(token, cfg.TenantSecretKey)
	if err != nil {
		t.Fatalf("ParseTenantToken() error = %v", err)
	}

	expiryDuration := time.Until(claims.ExpiresAt.Time)
	if expiryDuration > time.Hour {
		t.Errorf("impersonation token expiry %v exceeds 1 hour cap", expiryDuration)
	}
}

func TestImpersonationToken_ParseWithAdminSecretFails(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	token, err := GenerateImpersonationToken(ctx, uuid.New(), uuid.New(), uuid.New(), cfg)
	if err != nil {
		t.Fatalf("GenerateImpersonationToken() error = %v", err)
	}

	_, err = ParseTenantToken(token, cfg.AdminSecretKey)
	if err == nil {
		t.Fatal("expected error when parsing impersonation token with admin secret, got nil")
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
