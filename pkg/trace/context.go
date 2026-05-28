package trace

import "context"

type contextKey string

const (
	messageIDKey     contextKey = "message_id"
	correlationIDKey contextKey = "correlation_id"
)

func WithMessageID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, messageIDKey, id)
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationIDKey, id)
}

func MessageIDFromContext(ctx context.Context) string {
	value := ctx.Value(messageIDKey)
	if id, ok := value.(string); ok {
		return id
	}
	return ""
}

func CorrelationIDFromContext(ctx context.Context) string {
	value := ctx.Value(correlationIDKey)
	if id, ok := value.(string); ok {
		return id
	}
	return ""
}
