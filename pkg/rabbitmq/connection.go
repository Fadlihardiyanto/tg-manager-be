package rabbitmq

import (
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Connection wraps an AMQP connection with auto-reconnect support.
type Connection struct {
	conn   *amqp.Connection
	dsn    string
	config ConnectionConfig
	mu     sync.RWMutex
	closed chan struct{}
	logger *zap.Logger
}

// NewConnection creates a new RabbitMQ connection with auto-reconnect.
func NewConnection(dsn string, logger *zap.Logger, opts ...ConnectionOption) (*Connection, error) {
	cfg := DefaultConnectionConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	conn, err := amqp.Dial(dsn)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: failed to connect: %w", err)
	}

	c := &Connection{
		conn:   conn,
		dsn:    dsn,
		config: cfg,
		closed: make(chan struct{}),
		logger: logger,
	}

	go c.handleReconnect()

	c.logger.Info("rabbitmq: connection established")

	return c, nil
}

// Channel opens a new AMQP channel from the current connection.
func (c *Connection) Channel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil || c.conn.IsClosed() {
		return nil, fmt.Errorf("rabbitmq: connection is closed")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: failed to open channel: %w", err)
	}

	return ch, nil
}

// Close gracefully closes the connection.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.closed:
		return nil
	default:
		close(c.closed)
	}

	if c.conn != nil && !c.conn.IsClosed() {
		c.logger.Info("rabbitmq: closing connection")
		return c.conn.Close()
	}

	return nil
}

// IsClosed returns true if the connection has been closed intentionally.
func (c *Connection) IsClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

// handleReconnect listens for connection close events and attempts to reconnect.
func (c *Connection) handleReconnect() {
	for {
		if c.IsClosed() {
			return
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn == nil {
			time.Sleep(c.config.ReconnectDelay)
			continue
		}

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case err := <-notifyClose:
			if err == nil || c.IsClosed() {
				return
			}

			c.logger.Warn("rabbitmq: connection lost, attempting to reconnect...",
				zap.Error(err),
			)
			c.reconnect()

		case <-c.closed:
			return
		}
	}
}

// reconnect attempts to re-establish the connection with exponential backoff.
func (c *Connection) reconnect() {
	delay := c.config.ReconnectDelay
	// Guard: config 0/negatif → time.Sleep(0) hot-loop hammering amqp.Dial.
	if delay < time.Second {
		delay = time.Second
	}
	attempt := 0

	for {
		if c.IsClosed() {
			return
		}

		attempt++
		if c.config.MaxRetries > 0 && attempt > c.config.MaxRetries {
			c.logger.Error("rabbitmq: max reconnect retries reached, giving up",
				zap.Int("max_retries", c.config.MaxRetries),
			)
			return
		}

		c.logger.Info("rabbitmq: attempting to reconnect...",
			zap.Int("attempt", attempt),
			zap.Duration("delay", delay),
		)

		time.Sleep(delay)

		conn, err := amqp.Dial(c.dsn)
		if err != nil {
			c.logger.Warn("rabbitmq: reconnect failed", zap.Error(err))
			delay *= 2
			if delay > c.config.MaxReconnectDelay {
				delay = c.config.MaxReconnectDelay
			}
			continue
		}

		// Shutdown race: Close() bisa terjadi selama Sleep/Dial di atas —
		// dial yang selesai setelah close harus di-close, jangan bocor.
		c.mu.Lock()
		select {
		case <-c.closed:
			c.mu.Unlock()
			conn.Close()
			c.logger.Info("rabbitmq: reconnected after close, closing leaked connection")
			return
		default:
		}
		c.conn = conn
		c.mu.Unlock()

		c.logger.Info("rabbitmq: reconnected successfully",
			zap.Int("attempt", attempt),
		)
		return
	}
}
