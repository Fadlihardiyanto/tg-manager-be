package messaging

import (
	"context"

	json "github.com/bytedance/sonic"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/mailer"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/trace"
	"go.uber.org/zap"
)

// NotificationHandler processes queued email notifications.
type NotificationHandler struct {
	mailer mailer.Sender
	logger *zap.Logger
}

// NewNotificationHandler creates a new email notification handler.
func NewNotificationHandler(mailer mailer.Sender, logger *zap.Logger) *NotificationHandler {
	return &NotificationHandler{
		mailer: mailer,
		logger: logger,
	}
}

// Handle routes email tasks to the SMTP mailer.
func (h *NotificationHandler) Handle(ctx context.Context, body []byte) error {
	messageID := trace.MessageIDFromContext(ctx)
	correlationID := trace.CorrelationIDFromContext(ctx)
	logFields := []zap.Field{
		zap.String("message_id", messageID),
		zap.String("correlation_id", correlationID),
	}

	var payload model.EmailNotificationPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("notification: invalid payload, dropping message", append(logFields, zap.Error(err))...)
		return nil
	}

	switch payload.Type {
	case model.EmailNotificationOTP:
		if payload.To == "" || payload.Code == "" || payload.Purpose == "" {
			h.logger.Error("notification: missing otp fields, dropping message",
				append(logFields, zap.String("to", payload.To))...,
			)
			return nil
		}
		return h.mailer.SendOTP(payload.To, payload.Code, payload.Purpose)
	case model.EmailNotificationAdminVerification:
		if payload.To == "" || payload.Name == "" || payload.VerificationLink == "" {
			h.logger.Error("notification: missing admin verification fields, dropping message",
				append(logFields, zap.String("to", payload.To))...,
			)
			return nil
		}
		return h.mailer.SendAdminVerificationEmail(payload.To, payload.Name, payload.VerificationLink)
	case model.EmailNotificationTenantVerification:
		if payload.To == "" || payload.Name == "" || payload.VerificationLink == "" {
			h.logger.Error("notification: missing tenant verification fields, dropping message",
				append(logFields, zap.String("to", payload.To))...,
			)
			return nil
		}
		return h.mailer.SendTenantVerificationEmail(payload.To, payload.Name, payload.VerificationLink)
	default:
		h.logger.Error("notification: unknown type, dropping message",
			append(logFields, zap.String("type", string(payload.Type)))...,
		)
		return nil
	}
}
