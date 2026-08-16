package otp

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/redis/go-redis/v9"
)

const (
	// OTP code length (6 digits)
	otpLength = 6

	// DefaultTTL is the default expiry for an OTP code
	DefaultTTL = 5 * time.Minute

	// DefaultMaxAttempts is the max wrong attempts before the OTP is invalidated
	DefaultMaxAttempts = 5

	// DefaultCooldown prevents requesting a new OTP too soon
	DefaultCooldown = 60 * time.Second
)

// redisKey helpers — all keys are namespaced under "otp:"
func redisKeyCode(purpose, identifier string) string {
	return fmt.Sprintf("otp:%s:%s:code", purpose, identifier)
}

func redisKeyAttempts(purpose, identifier string) string {
	return fmt.Sprintf("otp:%s:%s:attempts", purpose, identifier)
}

func redisKeyCooldown(purpose, identifier string) string {
	return fmt.Sprintf("otp:%s:%s:cooldown", purpose, identifier)
}

// EmailOTPService manages OTP generation, storage (Redis), and verification.
// It does NOT send emails — that's the caller's responsibility.
type EmailOTPService struct {
	redis       *redis.Client
	ttl         time.Duration
	maxAttempts int
	cooldown    time.Duration
}

// Option configures the EmailOTPService.
type Option func(*EmailOTPService)

// WithTTL sets the OTP expiry duration.
func WithTTL(ttl time.Duration) Option {
	return func(s *EmailOTPService) {
		s.ttl = ttl
	}
}

// WithMaxAttempts sets the max verification attempts.
func WithMaxAttempts(max int) Option {
	return func(s *EmailOTPService) {
		s.maxAttempts = max
	}
}

// WithCooldown sets the minimum interval between OTP requests.
func WithCooldown(cooldown time.Duration) Option {
	return func(s *EmailOTPService) {
		s.cooldown = cooldown
	}
}

// NewEmailOTPService creates a new OTP service backed by Redis.
func NewEmailOTPService(redisClient *redis.Client, opts ...Option) *EmailOTPService {
	s := &EmailOTPService{
		redis:       redisClient,
		ttl:         DefaultTTL,
		maxAttempts: DefaultMaxAttempts,
		cooldown:    DefaultCooldown,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GenerateOTP creates a new OTP code for the given purpose and identifier (e.g., email).
//
// Purpose examples: "admin_login_2fa", "email_verify"
// Identifier: the user's email or ID
//
// Returns the generated code or an error if cooldown hasn't elapsed.
func (s *EmailOTPService) GenerateOTP(ctx context.Context, purpose, identifier string) (string, error) {
	cooldownKey := redisKeyCooldown(purpose, identifier)

	// Check cooldown — prevent spam
	exists, err := s.redis.Exists(ctx, cooldownKey).Result()
	if err != nil {
		return "", fmt.Errorf("otp: check cooldown: %w", err)
	}
	if exists > 0 {
		ttl, _ := s.redis.TTL(ctx, cooldownKey).Result()
		return "", helper.NewTooManyRequestsError(fmt.Sprintf("silakan tunggu %d detik sebelum meminta kode baru", int(ttl.Seconds())))
	}

	// Generate cryptographically secure 6-digit code
	code, err := generateSecureCode(otpLength)
	if err != nil {
		return "", fmt.Errorf("otp: generate code: %w", err)
	}

	codeKey := redisKeyCode(purpose, identifier)
	attemptsKey := redisKeyAttempts(purpose, identifier)

	// Store OTP in Redis with TTL (pipeline for atomicity)
	pipe := s.redis.Pipeline()
	pipe.Set(ctx, codeKey, code, s.ttl)
	pipe.Set(ctx, attemptsKey, 0, s.ttl)
	pipe.Set(ctx, cooldownKey, 1, s.cooldown)

	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("otp: store code: %w", err)
	}

	return code, nil
}

// VerifyOTP checks the provided code against the stored OTP.
// Returns nil on success.
// The OTP is deleted on success or when max attempts are exceeded.
func (s *EmailOTPService) VerifyOTP(ctx context.Context, purpose, identifier, code string) error {
	codeKey := redisKeyCode(purpose, identifier)
	attemptsKey := redisKeyAttempts(purpose, identifier)

	// Get stored code
	storedCode, err := s.redis.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		return helper.NewBadRequest("Kode OTP tidak ditemukan atau sudah kedaluwarsa")
	}
	if err != nil {
		return fmt.Errorf("otp: get code: %w", err)
	}

	// Increment attempts
	attempts, err := s.redis.Incr(ctx, attemptsKey).Result()
	if err != nil {
		return fmt.Errorf("otp: increment attempts: %w", err)
	}

	// Check max attempts — off-by-one fix: invalidasi pada percobaan ke-N
	// (dokumentasi "max N percobaan"), bukan ke-N+1.
	if int(attempts) >= s.maxAttempts {
		// Invalidate the OTP — too many wrong attempts
		s.redis.Del(ctx, codeKey, attemptsKey)
		return helper.NewTooManyRequestsError("Terlalu banyak percobaan, kode OTP telah dibatalkan. Silakan minta kode baru")
	}

	// Constant-time comparison to prevent timing attacks
	if !secureCompare(code, storedCode) {
		remaining := s.maxAttempts - int(attempts)
		return helper.NewBadRequest(fmt.Sprintf("Kode OTP salah, sisa %d percobaan", remaining))
	}

	// Success — clean up
	s.redis.Del(ctx, codeKey, attemptsKey)
	return nil
}

// Invalidate removes a pending OTP (e.g., on account lockout).
func (s *EmailOTPService) Invalidate(ctx context.Context, purpose, identifier string) error {
	codeKey := redisKeyCode(purpose, identifier)
	attemptsKey := redisKeyAttempts(purpose, identifier)
	cooldownKey := redisKeyCooldown(purpose, identifier)

	return s.redis.Del(ctx, codeKey, attemptsKey, cooldownKey).Err()
}

// =============================================================================
// Helpers
// =============================================================================

// generateSecureCode generates a cryptographically secure n-digit numeric code.
func generateSecureCode(length int) (string, error) {
	// Calculate the upper bound (10^length)
	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}

	// Pad with leading zeros if necessary
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n), nil
}

// secureCompare performs a constant-time string comparison to prevent timing attacks.
func secureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
