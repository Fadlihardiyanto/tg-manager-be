package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/mailer"
)

// QueueMailer publishes email tasks to RabbitMQ notification exchange.
type QueueMailer struct {
	publisher      *RabbitMQPublisher
	publishTimeout time.Duration
}

type QueueMailerOption func(*QueueMailer)

// NewQueueMailer creates a new queue-based mailer.
func NewQueueMailer(publisher *RabbitMQPublisher, opts ...QueueMailerOption) *QueueMailer {
	mailer := &QueueMailer{
		publisher:      publisher,
		publishTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(mailer)
	}
	if mailer.publishTimeout <= 0 {
		mailer.publishTimeout = 5 * time.Second
	}
	return mailer
}

func WithPublishTimeout(timeout time.Duration) QueueMailerOption {
	return func(m *QueueMailer) {
		m.publishTimeout = timeout
	}
}

// SendOTP enqueues an OTP email task.
func (m *QueueMailer) SendOTP(to, code, purpose string) error {
	if m.publisher == nil {
		return fmt.Errorf("queue mailer: publisher is nil")
	}

	payload := model.EmailNotificationPayload{
		Type:    model.EmailNotificationOTP,
		To:      to,
		Code:    code,
		Purpose: purpose,
	}

	return m.publish(payload)
}

// SendAdminVerificationEmail enqueues an admin verification email task.
func (m *QueueMailer) SendAdminVerificationEmail(to, name, verificationLink string) error {
	if m.publisher == nil {
		return fmt.Errorf("queue mailer: publisher is nil")
	}

	payload := model.EmailNotificationPayload{
		Type:             model.EmailNotificationAdminVerification,
		To:               to,
		Name:             name,
		VerificationLink: verificationLink,
	}

	return m.publish(payload)
}

// SendTenantVerificationEmail enqueues a tenant verification email task.
func (m *QueueMailer) SendTenantVerificationEmail(to, name, verificationLink string) error {
	if m.publisher == nil {
		return fmt.Errorf("queue mailer: publisher is nil")
	}

	payload := model.EmailNotificationPayload{
		Type:             model.EmailNotificationTenantVerification,
		To:               to,
		Name:             name,
		VerificationLink: verificationLink,
	}

	return m.publish(payload)
}

func (m *QueueMailer) publish(payload model.EmailNotificationPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), m.publishTimeout)
	defer cancel()

	return m.publisher.PublishNotification(ctx, payload)
}

var _ mailer.Sender = (*QueueMailer)(nil)
