// Package pdf provides PDF generation capabilities for payment receipts,
// transfer proofs, and other transactional documents.
//
// It uses gofpdf (pure-Go FPDF port) to render professional PDF documents
// without any external system dependencies (no wkhtmltopdf, no headless Chrome).
package pdf

import (
	"bytes"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ─────────────────────────────────────────────────────────────
// Configuration
// ─────────────────────────────────────────────────────────────

// Config holds PDF generator settings.
type Config struct {
	// LogoData is the raw bytes of the default merchant logo (PNG/JPEG).
	// Leave nil to render a text-only header.
	LogoData []byte

	// LogoExt is the image format extension: "png" or "jpg".
	// Required when LogoData is set.
	LogoExt string

	// FontDir is the path to a directory containing custom .ttf fonts.
	// When empty, the built-in Helvetica family is used (ASCII + Latin-1 only).
	// Provide this path if you need full Unicode / CJK support.
	FontDir string
}

// ─────────────────────────────────────────────────────────────
// Client
// ─────────────────────────────────────────────────────────────

// Client orchestrates PDF document generation.
type Client struct {
	config *Config
	logger *zap.Logger
}

// NewClient creates a new PDF generator client.
func NewClient(cfg *Config, logger *zap.Logger) *Client {
	if cfg == nil {
		cfg = &Config{}
	}
	return &Client{
		config: cfg,
		logger: logger,
	}
}

// ─────────────────────────────────────────────────────────────
// Public API
// ─────────────────────────────────────────────────────────────

// GenerateReceipt renders a payment receipt PDF and returns the raw bytes.
// The returned bytes are a complete, valid PDF document ready to be stored
// (e.g. uploaded to S3) or streamed directly as an HTTP response.
func (c *Client) GenerateReceipt(data *ReceiptData) ([]byte, error) {
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("pdf: invalid receipt data: %w", err)
	}

	// Stamp generation time if not set.
	if data.GeneratedAt.IsZero() {
		data.GeneratedAt = time.Now()
	}

	tmpl := newReceiptTemplate(c.config, c.logger)
	if err := tmpl.Render(data); err != nil {
		return nil, fmt.Errorf("pdf: render receipt: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Output(buf); err != nil {
		return nil, fmt.Errorf("pdf: output receipt: %w", err)
	}

	c.logger.Info("pdf: receipt generated",
		zap.String("order_id", data.OrderID),
		zap.Int("bytes", buf.Len()),
	)

	return buf.Bytes(), nil
}

// GenerateReceiptFile renders a payment receipt and writes it to the given path.
// Useful for local debugging or filesystem-based storage.
func (c *Client) GenerateReceiptFile(data *ReceiptData, filePath string) error {
	if err := data.Validate(); err != nil {
		return fmt.Errorf("pdf: invalid receipt data: %w", err)
	}

	if data.GeneratedAt.IsZero() {
		data.GeneratedAt = time.Now()
	}

	tmpl := newReceiptTemplate(c.config, c.logger)
	if err := tmpl.Render(data); err != nil {
		return fmt.Errorf("pdf: render receipt: %w", err)
	}

	if err := tmpl.OutputFile(filePath); err != nil {
		return fmt.Errorf("pdf: write receipt file: %w", err)
	}

	c.logger.Info("pdf: receipt saved to file",
		zap.String("order_id", data.OrderID),
		zap.String("path", filePath),
	)

	return nil
}
