package helper

import (
	"errors"
	"testing"
)

func TestErrNotFound_Error(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		want     string
	}{
		{"resource name", "Member", "Member tidak ditemukan"},
		{"already translated", "Member tidak ditemukan", "Member tidak ditemukan"},
		{"with 'not found' suffix", "Plan not found", "Plan tidak ditemukan"},
		{"empty resource", "", " tidak ditemukan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewNotFound(tt.resource)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNewBadRequestWrap_PreservesChain(t *testing.T) {
	cause := errors.New("underlying db failure")
	err := NewBadRequestWrap(cause)

	if err.Error() != "underlying db failure" {
		t.Errorf("Error() = %q, want sanitized message", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want true (Unwrap must preserve the chain)")
	}
	var typed *ErrBadRequest
	if !errors.As(err, &typed) {
		t.Error("errors.As to ErrBadRequest = false, want true")
	}
}

func TestErrBadRequest_Error(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"verification token", "verification token is required", "Token verifikasi wajib diisi"},
		{"invalid json", "invalid JSON payload", "Payload JSON tidak valid"},
		{"invalid bot id", "invalid bot id: abc", "ID bot tidak valid"},
		{"passthrough", "some other message", "some other message"},
		{"empty message", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewBadRequest(tt.message)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestErrConflict_Error(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"email exists", "email already exists", "Email sudah terdaftar"},
		{"2fa enabled", "2FA is already enabled", "2FA sudah diaktifkan"},
		{"slug exists", "client with slug 'my-client' already exists", "Client dengan slug 'my-client' already exists"},
		{"role exists", "role with name 'admin' already exists", "Role dengan nama 'admin' already exists"},
		{"plan exists", "plan with name 'premium' already exists", "Plan dengan nama 'premium' already exists"},
		{"delete free plan", "cannot delete the default 'free' plan", "Tidak dapat menghapus plan default 'free'"},
		{"delete plan with active clients", "cannot delete plan: 3 active clients are using this plan", "Tidak dapat menghapus plan: 3 active clients are using this plan"},
		{"generic already exists", "something already exists", "Data sudah ada"},
		{"passthrough", "unrecognized conflict", "unrecognized conflict"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewConflict(tt.message)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestErrForbidden_Error(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"superadmin assign roles", "forbidden: only superadmin can assign roles", "Hanya superadmin yang dapat menetapkan peran"},
		{"superadmin revoke roles", "forbidden: only superadmin can revoke roles", "Hanya superadmin yang dapat mencabut peran"},
		{"permission required", "forbidden: requires 'billing.read' permission", "Anda memerlukan izin 'billing.read'"},
		{"account locked", "forbidden: account is locked until 2025-01-01", "Akun terkunci hingga 2025-01-01"},
		{"generic forbidden", "forbidden: something", "Akses ditolak: something"},
		{"empty forbidden", "forbidden:", "Anda tidak memiliki izin untuk melakukan aksi ini"},
		{"non-forbidden message", "not a forbidden message", "not a forbidden message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewForbidden(tt.message)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestErrUnauthorized_Error(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"invalid email or password", "invalid email or password", "Email atau password salah"},
		{"invalid refresh token", "invalid refresh token", "Refresh token tidak valid"},
		{"token blacklisted", "token is blacklisted", "Token telah diblokir"},
		{"passthrough", "some error", "some error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewUnauthorized(tt.message)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestErrUnprocessable_Error(t *testing.T) {
	err := NewUnprocessable("custom message")
	if err.Error() != "custom message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "custom message")
	}
}

func TestErrTooManyRequests_Error(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{"with message", "Tunggu 30 detik", "Tunggu 30 detik"},
		{"empty message", "", "Terlalu banyak request, silakan coba lagi nanti"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTooManyRequestsError(tt.message)
			if err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}
