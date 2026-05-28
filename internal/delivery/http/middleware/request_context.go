package middleware

import (
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const (
	HeaderRequestID = "X-Request-ID"
	HeaderClientID  = "X-Client-ID"
)

// RequestContext injects request_id and client_id into the request context.
// request_id is taken from the response header populated by requestid middleware,
// while client_id is read from X-Client-ID when provided.
func RequestContext() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		requestID := ctx.GetRespHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}

		clientID := ctx.Get(HeaderClientID)
		if clientID == "" {
			clientID = uuid.NewString()
		}

		requestCtx := logger.WithRequestID(ctx.Context(), requestID)
		requestCtx = logger.WithClientID(requestCtx, clientID)
		ctx.SetContext(requestCtx)
		ctx.Set(HeaderRequestID, requestID)

		return ctx.Next()
	}
}
