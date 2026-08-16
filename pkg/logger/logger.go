package logger

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	clientIDKey  contextKey = "client_id"
)

// WithRequestID stores a request_id in the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithClientID stores a client_id in the context.
func WithClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, clientIDKey, clientID)
}

// RequestID retrieves the request_id from context.
// Accepts both context.Context and any type with Context() method (e.g. fiber.Ctx).
// Returns empty string if not set.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// ClientID retrieves the client_id from context. Returns empty string if not set.
func ClientID(ctx context.Context) string {
	if v, ok := ctx.Value(clientIDKey).(string); ok {
		return v
	}
	return ""
}

// NewContext creates a new context with request_id and client_id.
// Generates a new UUID for request_id and uses the provided client_id.
func NewContext(ctx context.Context, clientID string) context.Context {
	ctx = WithRequestID(ctx, uuid.New().String())
	ctx = WithClientID(ctx, clientID)
	return ctx
}

// NewContextWithDefaults creates a new context with auto-generated request_id
// and a placeholder client_id (UUID). Use this when client_id is not yet available.
func NewContextWithDefaults(ctx context.Context) context.Context {
	ctx = WithRequestID(ctx, uuid.New().String())
	ctx = WithClientID(ctx, uuid.New().String())
	return ctx
}

// FromContext extracts request_id and client_id from context and returns
// a new logger with those fields attached.
func FromContext(ctx context.Context, log *zap.Logger) *zap.Logger {
	// Nil guard: nil logger / nil ctx tidak boleh panic — nop logger aman.
	if log == nil {
		log = zap.NewNop()
	}
	if ctx == nil {
		return log
	}

	fields := []zap.Field{}

	if reqID := RequestID(ctx); reqID != "" {
		fields = append(fields, zap.String("request_id", reqID))
	}

	if clientID := ClientID(ctx); clientID != "" {
		fields = append(fields, zap.String("client_id", clientID))
	}

	if len(fields) > 0 {
		return log.With(fields...)
	}

	return log
}
