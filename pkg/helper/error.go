package helper

import (
	"fmt"
	"strings"
)

type ErrNotFound struct {
	Resource string
}

func (e *ErrNotFound) Error() string {
	resource := strings.TrimSpace(e.Resource)
	lower := strings.ToLower(resource)
	if strings.Contains(lower, "tidak ditemukan") {
		return resource
	}
	if strings.HasSuffix(lower, " not found") {
		resource = strings.TrimSpace(resource[:len(resource)-len(" not found")])
	}
	return fmt.Sprintf("%s tidak ditemukan", resource)
}

type ErrBadRequest struct {
	Message string
}

func (e *ErrBadRequest) Error() string {
	message := strings.TrimSpace(e.Message)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "verification token is required"):
		return "Token verifikasi wajib diisi"
	case strings.Contains(lower, "verification token is invalid or expired"):
		return "Token verifikasi tidak valid atau sudah kedaluwarsa"
	case strings.Contains(lower, "invalid admin id in token"):
		return "ID admin dalam token tidak valid"
	case strings.Contains(lower, "invalid json payload"):
		return "Payload JSON tidak valid"
	case strings.Contains(lower, "invalid request body"):
		return "Isi request tidak valid"
	case strings.Contains(lower, "invalid bot id"):
		return "ID bot tidak valid"
	case strings.Contains(lower, "invalid request"):
		return "Request tidak valid"
	default:
		return message
	}
}

type ErrConflict struct {
	Message string
}

func (e *ErrConflict) Error() string {
	message := strings.TrimSpace(e.Message)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "2fa is already enabled"):
		return "2FA sudah diaktifkan"
	case strings.Contains(lower, "email already exists"):
		return "Email sudah terdaftar"
	case strings.Contains(lower, "client with slug") && strings.Contains(lower, "already exists"):
		return strings.Replace(message, "client with slug", "Client dengan slug", 1)
	case strings.Contains(lower, "role with name") && strings.Contains(lower, "already exists"):
		return strings.Replace(message, "role with name", "Role dengan nama", 1)
	case strings.Contains(lower, "plan with name") && strings.Contains(lower, "already exists"):
		return strings.Replace(message, "plan with name", "Plan dengan nama", 1)
	case strings.Contains(lower, "cannot delete the default 'free' plan"):
		return "Tidak dapat menghapus plan default 'free'"
	case strings.Contains(lower, "cannot delete plan:") && strings.Contains(lower, "active clients are using this plan"):
		return strings.Replace(message, "cannot delete plan", "Tidak dapat menghapus plan", 1)
	case strings.Contains(lower, "already exists"):
		return "Data sudah ada"
	default:
		return message
	}
}

type ErrForbidden struct {
	Message    string
	Permission string
}

func (e *ErrForbidden) Error() string {
	message := strings.TrimSpace(e.Message)
	lower := strings.ToLower(message)
	switch {
	case e.Permission != "":
		return fmt.Sprintf("Anda memerlukan izin %s", e.Permission)
	case strings.HasPrefix(lower, "forbidden:"):
		message = strings.TrimSpace(strings.TrimPrefix(message, "forbidden:"))
		switch {
		case strings.Contains(strings.ToLower(message), "only superadmin can assign roles"):
			return "Hanya superadmin yang dapat menetapkan peran"
		case strings.Contains(strings.ToLower(message), "only superadmin can revoke roles"):
			return "Hanya superadmin yang dapat mencabut peran"
		case strings.Contains(strings.ToLower(message), "requires '") && strings.Contains(strings.ToLower(message), " permission"):
			start := strings.Index(message, "'")
			end := strings.LastIndex(message, "'")
			if start != -1 && end > start {
				return fmt.Sprintf("Anda memerlukan izin %s", message[start:end+1])
			}
			return "Anda tidak memiliki izin untuk melakukan aksi ini"
		case strings.Contains(strings.ToLower(message), "account is locked until"):
			return strings.Replace(message, "account is locked until", "Akun terkunci hingga", 1)
		default:
			if message == "" {
				return "Anda tidak memiliki izin untuk melakukan aksi ini"
			}
			return "Akses ditolak: " + message
		}
	default:
		return message
	}
}

type ErrUnauthorized struct {
	Message string
}

func (e *ErrUnauthorized) Error() string {
	message := strings.TrimSpace(e.Message)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "invalid email or password"):
		return "Email atau password salah"
	case strings.Contains(lower, "invalid or expired temporary token"):
		return "Token sementara tidak valid atau sudah kedaluwarsa"
	case strings.Contains(lower, "invalid refresh token"):
		return "Refresh token tidak valid"
	case strings.Contains(lower, "not a refresh token"):
		return "Token ini bukan refresh token"
	case strings.Contains(lower, "token is blacklisted"):
		return "Token telah diblokir"
	default:
		return message
	}
}

type ErrUnprocessable struct {
	Message string
}

func (e *ErrUnprocessable) Error() string {
	return e.Message
}

func NewNotFound(resource string) error {
	return &ErrNotFound{Resource: resource}
}

func NewBadRequest(message string) error {
	return &ErrBadRequest{Message: message}
}

func NewConflict(message string) error {
	return &ErrConflict{Message: message}
}

func NewForbidden(message string) error {
	return &ErrForbidden{Message: message}
}

// NewForbiddenPermission returns a forbidden error with the permission name
// stored as a structured field instead of being scraped out of the message.
func NewForbiddenPermission(permission string) error {
	return &ErrForbidden{Message: fmt.Sprintf("forbidden: requires '%s' permission", permission), Permission: permission}
}

func NewUnauthorized(message string) error {
	return &ErrUnauthorized{Message: message}
}

func NewUnprocessable(message string) error {
	return &ErrUnprocessable{Message: message}
}

type ErrTooManyRequests struct {
	Message string
}

func (e *ErrTooManyRequests) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		return "Terlalu banyak request, silakan coba lagi nanti"
	}
	return message
}

func NewTooManyRequestsError(message string) error {
	return &ErrTooManyRequests{Message: message}
}
