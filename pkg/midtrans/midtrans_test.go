package midtrans

import (
	"crypto/sha512"
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
)

func TestEnvironmentURLs(t *testing.T) {
	tests := []struct {
		name        string
		isSandbox   bool
		wantBaseURL string
		wantSnapURL string
	}{
		{
			name:        "sandbox",
			isSandbox:   true,
			wantBaseURL: SandboxBaseURL,
			wantSnapURL: SandboxSnapURL,
		},
		{
			name:        "production",
			isSandbox:   false,
			wantBaseURL: ProductionBaseURL,
			wantSnapURL: ProductionSnapURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBaseURL, gotSnapURL := EnvironmentURLs(tt.isSandbox)
			if gotBaseURL != tt.wantBaseURL {
				t.Errorf("EnvironmentURLs() baseURL = %q, want %q", gotBaseURL, tt.wantBaseURL)
			}
			if gotSnapURL != tt.wantSnapURL {
				t.Errorf("EnvironmentURLs() snapURL = %q, want %q", gotSnapURL, tt.wantSnapURL)
			}
		})
	}
}

func TestIsSuccess(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		fraudStatus string
		want        bool
	}{
		{
			name:   "settlement is always success",
			status: "settlement",
			want:   true,
		},
		{
			name:        "capture with accept fraud",
			status:      "capture",
			fraudStatus: "accept",
			want:        true,
		},
		{
			name:        "capture with challenge fraud",
			status:      "capture",
			fraudStatus: "challenge",
			want:        false,
		},
		{
			name:        "capture with deny fraud",
			status:      "capture",
			fraudStatus: "deny",
			want:        false,
		},
		{
			name:   "pending is not success",
			status: "pending",
			want:   false,
		},
		{
			name:   "deny is not success",
			status: "deny",
			want:   false,
		},
		{
			name:   "expire is not success",
			status: "expire",
			want:   false,
		},
		{
			name:   "cancel is not success",
			status: "cancel",
			want:   false,
		},
		{
			name:   "failure is not success",
			status: "failure",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &WebhookNotification{
				TransactionStatus: tt.status,
				FraudStatus:       tt.fraudStatus,
			}
			got := n.IsSuccess()
			if got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v (status=%q, fraud=%q)", got, tt.want, tt.status, tt.fraudStatus)
			}
		})
	}
}

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "expire is expired", status: "expire", want: true},
		{name: "settlement is not expired", status: "settlement", want: false},
		{name: "pending is not expired", status: "pending", want: false},
		{name: "deny is not expired", status: "deny", want: false},
		{name: "empty string is not expired", status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &WebhookNotification{TransactionStatus: tt.status}
			got := n.IsExpired()
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsFailed(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "deny is failed", status: "deny", want: true},
		{name: "cancel is failed", status: "cancel", want: true},
		{name: "failure is failed", status: "failure", want: true},
		{name: "settlement is not failed", status: "settlement", want: false},
		{name: "pending is not failed", status: "pending", want: false},
		{name: "expire is not failed", status: "expire", want: false},
		{name: "capture is not failed", status: "capture", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &WebhookNotification{TransactionStatus: tt.status}
			got := n.IsFailed()
			if got != tt.want {
				t.Errorf("IsFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractSnapToken(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "valid snap URL",
			url:  "https://app.sandbox.midtrans.com/snap/v1/2c3f8b4a-1d5e-6f7a-8b9c-0d1e2f3a4b5c",
			want: "2c3f8b4a-1d5e-6f7a-8b9c-0d1e2f3a4b5c",
		},
		{
			name: "valid token with trailing slash",
			url:  "https://app.midtrans.com/snap/v1/abc123def456/",
			want: "abc123def456",
		},
		{
			name: "empty URL",
			url:  "",
			want: "",
		},
		{
			name: "short token (less than 10 chars)",
			url:  "https://example.com/abc",
			want: "",
		},
		{
			name: "token contains dot",
			url:  "https://example.com/a.b.c.d.e.f",
			want: "",
		},
		{
			name: "token contains query param",
			url:  "https://example.com/snap/abc123def456?page=1",
			want: "",
		},
		{
			name: "no path segments",
			url:  "https://example.com",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractSnapToken(tt.url)
			if got != tt.want {
				t.Errorf("ExtractSnapToken(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestAmountFromDecimal(t *testing.T) {
	tests := []struct {
		name    string
		amount  decimal.Decimal
		want    int64
		wantErr bool
	}{
		{
			name:   "whole number",
			amount: decimal.NewFromInt(15000),
			want:   15000,
		},
		{
			name:   "zero",
			amount: decimal.NewFromInt(0),
			want:   0,
		},
		{
			name:   "large number",
			amount: decimal.NewFromInt(999999999),
			want:   999999999,
		},
		{
			name:    "decimal fraction is rejected",
			amount:  decimal.NewFromFloat(15000.50),
			wantErr: true,
		},
		{
			name:    "small fraction is rejected",
			amount:  decimal.NewFromFloat(0.01),
			wantErr: true,
		},
		{
			name:    "negative decimal fraction is rejected",
			amount:  decimal.NewFromFloat(-100.50),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AmountFromDecimal(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("AmountFromDecimal(%v) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("AmountFromDecimal(%v) = %v, want %v", tt.amount, got, tt.want)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	serverKey := "test-server-key-123"
	client := &Client{
		config: Config{ServerKey: serverKey},
	}

	// Helper: compute expected signature
	computeSignature := func(orderID, statusCode, grossAmount string) string {
		raw := orderID + statusCode + grossAmount + serverKey
		h := sha512.New()
		h.Write([]byte(raw))
		return fmt.Sprintf("%x", h.Sum(nil))
	}

	tests := []struct {
		name        string
		orderID     string
		statusCode  string
		grossAmount string
		want        bool
	}{
		{
			name:        "valid signature",
			orderID:     "ORDER-001",
			statusCode:  "200",
			grossAmount: "50000.00",
			want:        true,
		},
		{
			name:        "empty fields",
			orderID:     "",
			statusCode:  "",
			grossAmount: "",
			want:        true,
		},
		{
			name:        "invalid signature - wrong order ID",
			orderID:     "ORDER-002",
			statusCode:  "200",
			grossAmount: "50000.00",
			want:        false,
		},
		{
			name:        "invalid signature - wrong amount",
			orderID:     "ORDER-001",
			statusCode:  "200",
			grossAmount: "60000.00",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification := &WebhookNotification{
				OrderID:     tt.orderID,
				StatusCode:  tt.statusCode,
				GrossAmount: tt.grossAmount,
			}
			// For valid cases, set the correct signature
			if tt.want {
				notification.SignatureKey = computeSignature(tt.orderID, tt.statusCode, tt.grossAmount)
			} else {
				// Use the correct signature for ORDER-001 to make it mismatch when order ID differs
				notification.SignatureKey = computeSignature("ORDER-001", "200", "50000.00")
			}

			got := client.VerifySignature(notification)
			if got != tt.want {
				if tt.want {
					t.Errorf("VerifySignature() = %v, want %v (expected a valid signature)", got, tt.want)
				} else {
					t.Errorf("VerifySignature() = %v, want %v (signature should be invalid)", got, tt.want)
				}
			}
		})
	}
}
