package midtrans

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	json "github.com/bytedance/sonic"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

const (
	SandboxBaseURL    = "https://api.sandbox.midtrans.com"
	SandboxSnapURL    = "https://app.sandbox.midtrans.com/snap/v1"
	ProductionBaseURL = "https://api.midtrans.com"
	ProductionSnapURL = "https://app.midtrans.com/snap/v1"
)

// snapRetries is the number of retries on transient failures (network / 5xx).
// 4xx (config/validation) and the final attempt are never retried.
const snapRetries = 2

// EnvironmentURLs returns the BaseURL and SnapURL for the specified environment.
func EnvironmentURLs(isSandbox bool) (baseURL, snapURL string) {
	if isSandbox {
		return SandboxBaseURL, SandboxSnapURL
	}
	return ProductionBaseURL, ProductionSnapURL
}

type Config struct {
	ServerKey string
	ClientKey string
	BaseURL   string // https://app.sandbox.midtrans.com untuk sandbox
	SnapURL   string // https://app.sandbox.midtrans.com/snap/v1
}

type Client struct {
	config     Config
	httpClient *http.Client
	logger     *zap.Logger
}

func NewClient(cfg Config, logger *zap.Logger) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) log() *zap.Logger {
	if c.logger == nil {
		return zap.NewNop()
	}
	return c.logger
}

// ── Snap Request/Response ─────────────────────────────────────

type SnapRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	CustomerDetails    CustomerDetails    `json:"customer_details"`
	ItemDetails        []ItemDetail       `json:"item_details"`
	Callbacks          *SnapCallbacks     `json:"callbacks,omitempty"`
	Expiry             *SnapExpiry        `json:"expiry,omitempty"`
	NotificationURL    string             `json:"notification_url,omitempty"`
}

type TransactionDetails struct {
	OrderID     string `json:"order_id"`
	GrossAmount int64  `json:"gross_amount"`
}

type CustomerDetails struct {
	FirstName string `json:"first_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone,omitempty"`
}

type ItemDetail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

type SnapCallbacks struct {
	Finish string `json:"finish"`
}

type SnapExpiry struct {
	Duration int    `json:"duration"`
	Unit     string `json:"unit"` // "minute", "hour", "day"
}

type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// ── Notification (Webhook Payload dari Midtrans) ──────────────

type WebhookNotification struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"`
	TransactionID     string `json:"transaction_id"`
	OrderID           string `json:"order_id"`
	GrossAmount       string `json:"gross_amount"`
	PaymentType       string `json:"payment_type"`
	SignatureKey      string `json:"signature_key"`
	StatusCode        string `json:"status_code"`
	FraudStatus       string `json:"fraud_status"`
}

// IsSuccess menentukan apakah transaksi dianggap berhasil
func (n *WebhookNotification) IsSuccess() bool {
	switch n.TransactionStatus {
	case "capture":
		return n.FraudStatus == "accept"
	case "settlement":
		return true
	default:
		return false
	}
}

// IsExpired menentukan apakah transaksi expired
func (n *WebhookNotification) IsExpired() bool {
	return n.TransactionStatus == "expire"
}

// IsFailed menentukan apakah transaksi gagal
func (n *WebhookNotification) IsFailed() bool {
	return n.TransactionStatus == "deny" ||
		n.TransactionStatus == "cancel" ||
		n.TransactionStatus == "failure"
}

// ── Client Methods ────────────────────────────────────────────

// CreateSnapToken membuat Snap payment token via Midtrans API
func (c *Client) CreateSnapToken(ctx context.Context, req *SnapRequest) (*SnapResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/transactions", c.config.SnapURL)

	var lastErr error
	for attempt := 0; attempt <= snapRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("Authorization", "Basic "+c.basicAuth())
		if req.NotificationURL != "" {
			httpReq.Header.Set("X-Override-Notification", req.NotificationURL)
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			c.log().Warn("midtrans: snap create network error (retrying)",
				zap.String("order_id", req.TransactionDetails.OrderID),
				zap.Int64("gross_amount", req.TransactionDetails.GrossAmount),
				zap.Int("attempt", attempt+1),
				zap.Error(err))
			if attempt < snapRetries {
				time.Sleep(retryBackoff(attempt))
				continue
			}
			break
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read response body: %w", readErr)
		}

		// Retry on 5xx (transient server error); never on 4xx (config/validation).
		if resp.StatusCode >= 500 && attempt < snapRetries {
			c.log().Warn("midtrans: snap create 5xx (retrying)",
				zap.String("order_id", req.TransactionDetails.OrderID),
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", attempt+1))
			time.Sleep(retryBackoff(attempt))
			continue
		}

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			errMsg := c.errorBody(bodyBytes)
			c.log().Error("midtrans: snap create failed",
				zap.String("order_id", req.TransactionDetails.OrderID),
				zap.Int64("gross_amount", req.TransactionDetails.GrossAmount),
				zap.Int("status", resp.StatusCode),
				zap.String("response", errMsg))
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, errMsg)
		}

		var snapResp SnapResponse
		if err := json.Unmarshal(bodyBytes, &snapResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		// json.Unmarshal tidak error pada field yang hilang — 2xx dengan
		// payload partial/proxy HTML menghasilkan token kosong sebagai sukses.
		// Checkout dengan token kosong = payment silently broken.
		if snapResp.Token == "" {
			c.log().Error("midtrans: snap create returned empty token",
				zap.String("order_id", req.TransactionDetails.OrderID),
				zap.Int("status", resp.StatusCode),
				zap.Int("response_bytes", len(bodyBytes)))
			return nil, fmt.Errorf("midtrans: empty snap token in response")
		}

		c.log().Info("midtrans: snap token created",
			zap.String("order_id", req.TransactionDetails.OrderID),
			zap.Int64("gross_amount", req.TransactionDetails.GrossAmount))
		return &snapResp, nil
	}

	return nil, fmt.Errorf("http request: %w", lastErr)
}

// errorBody returns a safe string representation of an error response body,
// falling back to the raw (truncated) body when it is not JSON.
func (c *Client) errorBody(body []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err == nil && m != nil {
		b, _ := json.Marshal(m)
		return string(b)
	}
	s := string(body)
	if len(s) > 500 {
		s = s[:500] + "..."
	}
	return s
}

func retryBackoff(attempt int) time.Duration {
	return time.Duration(200*(attempt+1)) * time.Millisecond
}

// VerifySignature memvalidasi signature dari webhook Midtrans
// Formula: SHA512(order_id + status_code + gross_amount + server_key)
func (c *Client) VerifySignature(notification *WebhookNotification) bool {
	rawStr := notification.OrderID +
		notification.StatusCode +
		notification.GrossAmount +
		c.config.ServerKey

	h := sha512.New()
	h.Write([]byte(rawStr))
	expected := fmt.Sprintf("%x", h.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(expected), []byte(notification.SignatureKey)) == 1
}

func (c *Client) ClientKey() string {
	return c.config.ClientKey
}

func ExtractSnapToken(paymentURL string) string {
	if paymentURL == "" {
		return ""
	}
	parts := strings.Split(strings.TrimRight(paymentURL, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	token := parts[len(parts)-1]
	if strings.Contains(token, ".") || strings.Contains(token, "?") || len(token) < 10 {
		return ""
	}
	return token
}

func (c *Client) basicAuth() string {
	credentials := c.config.ServerKey + ":"
	return base64.StdEncoding.EncodeToString([]byte(credentials))
}

func AmountFromDecimal(amount decimal.Decimal) (int64, error) {
	if !amount.Equal(amount.Truncate(0)) {
		return 0, fmt.Errorf("amount must be an integer")
	}
	return amount.IntPart(), nil
}

// ValidateServerKey verifies that a Midtrans Server Key is valid by making
// an authenticated request to the Midtrans API. A 401 means the key is invalid.
func ValidateServerKey(ctx context.Context, serverKey string, isSandbox bool) error {
	snapURL := SandboxSnapURL
	if !isSandbox {
		snapURL = ProductionSnapURL
	}

	url := fmt.Sprintf("%s/transactions", snapURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString("{}"))
	if err != nil {
		return fmt.Errorf("create validation request: %w", err)
	}

	credentials := serverKey + ":"
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("Server Key tidak valid")
	}

	// Jika status bukan 401 (misal 400 Bad Request karena JSON kosong),
	// maka autentikasi berhasil (kunci valid).
	return nil
}

func (c *Client) CancelTransaction(ctx context.Context, orderID string) error {
	url := fmt.Sprintf("%s/v2/%s/cancel", c.config.BaseURL, orderID)

	var lastErr error
	for attempt := 0; attempt <= snapRetries; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return fmt.Errorf("gagal membuat request pembatalan: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("Authorization", "Basic "+c.basicAuth())

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			lastErr = err
			c.log().Warn("midtrans: cancel network error (retrying)",
				zap.String("order_id", orderID),
				zap.Int("attempt", attempt+1),
				zap.Error(err))
			if attempt < snapRetries {
				time.Sleep(retryBackoff(attempt))
				continue
			}
			break
		}

		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("gagal membaca response pembatalan: %w", readErr)
		}

		if resp.StatusCode >= 500 && attempt < snapRetries {
			c.log().Warn("midtrans: cancel 5xx (retrying)",
				zap.String("order_id", orderID),
				zap.Int("status", resp.StatusCode),
				zap.Int("attempt", attempt+1))
			time.Sleep(retryBackoff(attempt))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			errMsg := c.errorBody(bodyBytes)
			c.log().Warn("midtrans: cancel transaction failed",
				zap.String("order_id", orderID),
				zap.Int("status", resp.StatusCode),
				zap.String("response", errMsg))
			return fmt.Errorf("midtrans error %d: %s", resp.StatusCode, errMsg)
		}

		c.log().Info("midtrans: transaction cancelled", zap.String("order_id", orderID))
		return nil
	}

	return fmt.Errorf("http request ke Midtrans gagal: %w", lastErr)
}
