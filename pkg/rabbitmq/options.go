package rabbitmq

import (
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchange names
const (
	ExchangeTelegram     = "tg-manager.telegram"
	ExchangePayment      = "tg-manager.payment"
	ExchangeNotification = "tg-manager.notification"
	ExchangeDLX          = "tg-manager.dlx"
)

// Queue names
const (
	QueueTelegramAction      = "task.telegram_action"
	QueueTelegramActionDLQ   = "task.telegram_action.dlq"
	QueueNotification        = "task.notification"
	QueuePaymentWebhook      = "task.payment_webhook"
	QueuePaymentWebhookDLQ   = "task.payment_webhook.dlq"
	QueueGatekeeping         = "task.telegram_gatekeeping"
	QueueGatekeepingDLQ      = "task.telegram_gatekeeping.dlq"
	QueueEnforcer            = "task.telegram_enforcer"
	QueueEnforcerDLQ         = "task.telegram_enforcer.dlq"
	QueueExpiryReminder      = "task.telegram_expiry_reminder"
	QueueExpiryReminderDLQ   = "task.telegram_expiry_reminder.dlq"
)

// Routing keys
const (
	RoutingKeyTelegramAction = "telegram.action"
	RoutingKeyPaymentWebhook = "payment.webhook"
	RoutingKeyNotification   = "notification.send"
	RoutingKeyGatekeeping    = "telegram.gatekeeping"
	RoutingKeyEnforcer       = "telegram.enforcer"
	RoutingKeyExpiryReminder = "telegram.expiry_reminder"
)

// ExchangeConfig holds the configuration for declaring an exchange.
type ExchangeConfig struct {
	Name       string
	Kind       string // direct, topic, fanout, headers
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

// QueueConfig holds the configuration for declaring a queue.
type QueueConfig struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table
}

// BindingConfig holds the configuration for binding a queue to an exchange.
type BindingConfig struct {
	QueueName    string
	RoutingKey   string
	ExchangeName string
	NoWait       bool
	Args         amqp.Table
}

// TopologyConfig holds the complete topology declaration.
type TopologyConfig struct {
	Exchanges []ExchangeConfig
	Queues    []QueueConfig
	Bindings  []BindingConfig
}

// ConnectionConfig holds connection configuration.
type ConnectionConfig struct {
	ReconnectDelay    time.Duration
	MaxReconnectDelay time.Duration
	MaxRetries        int // 0 = unlimited
}

// DefaultConnectionConfig returns the default connection configuration.
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		ReconnectDelay:    1 * time.Second,
		MaxReconnectDelay: 30 * time.Second,
		MaxRetries:        0,
	}
}

// ConnectionOption is a function that configures a Connection.
type ConnectionOption func(*ConnectionConfig)

// WithReconnectDelay sets the initial reconnect delay.
func WithReconnectDelay(d time.Duration) ConnectionOption {
	return func(c *ConnectionConfig) {
		c.ReconnectDelay = d
	}
}

// WithMaxReconnectDelay sets the maximum reconnect delay (backoff cap).
func WithMaxReconnectDelay(d time.Duration) ConnectionOption {
	return func(c *ConnectionConfig) {
		c.MaxReconnectDelay = d
	}
}

// WithMaxRetries sets the maximum number of reconnect attempts. 0 = unlimited.
func WithMaxRetries(n int) ConnectionOption {
	return func(c *ConnectionConfig) {
		c.MaxRetries = n
	}
}
