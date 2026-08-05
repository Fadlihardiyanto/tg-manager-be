package telegram

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	json "github.com/bytedance/sonic"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// BotClient provides methods to interact with the Telegram API.
type BotClient interface {
	GetBot() *tgbotapi.BotAPI
	Request(ctx context.Context, c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	Send(ctx context.Context, c tgbotapi.Chattable) (tgbotapi.Message, error)
	SetWebhook(ctx context.Context, webhookURL string, secretToken string) error
	DeleteWebhook(ctx context.Context) error
	GetWebhookInfo(ctx context.Context) (tgbotapi.WebhookInfo, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	KickChatMember(ctx context.Context, chatID int64, userID int64, untilDate time.Time) error
	UnbanChatMember(ctx context.Context, chatID int64, userID int64, onlyIfBanned bool) error
	ApproveChatJoinRequest(ctx context.Context, chatID int64, userID int64) error
	DeclineChatJoinRequest(ctx context.Context, chatID int64, userID int64) error
	CreateChatInviteLink(ctx context.Context, chatID int64, name string, createsJoinRequest bool) (string, error)
	RevokeChatInviteLink(ctx context.Context, chatID int64, inviteLink string) error
	GetChatMember(ctx context.Context, chatID int64, userID int64) (tgbotapi.ChatMember, error)
	GetChatMembersCount(ctx context.Context, chatID int64) (int, error)
	SendDocument(ctx context.Context, c tgbotapi.Chattable) (tgbotapi.Message, error)
	LeaveChat(ctx context.Context, chatID int64) error
}

type botClientImpl struct {
	bot     *tgbotapi.BotAPI
	logger  *zap.Logger
	limiter *rate.Limiter
}

// BotFactory manages the creation of Telegram bot clients.
type BotFactory interface {
	NewClient(token string) (BotClient, error)
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

type botFactoryImpl struct {
	httpClient *http.Client
	logger     *zap.Logger
	limiters   map[string]*limiterEntry
	mu         sync.Mutex
}

// NewBotFactory creates a new BotFactory.
func NewBotFactory(timeout time.Duration, logger *zap.Logger) BotFactory {
	return &botFactoryImpl{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:   logger,
		limiters: make(map[string]*limiterEntry),
	}
}

// NewClient creates a new BotClient instance for the given bot token.
func (f *botFactoryImpl) NewClient(token string) (BotClient, error) {
	bot, err := tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, f.httpClient)
	if err != nil {
		return nil, fmt.Errorf("telegram: failed to initialize bot: %w", err)
	}

	now := time.Now()
	f.mu.Lock()
	entry, exists := f.limiters[token]
	if !exists {
		// Max 25 requests per second, burst of 1 (to ensure smooth pacing)
		entry = &limiterEntry{limiter: rate.NewLimiter(rate.Limit(25), 1), lastUsed: now}
		f.limiters[token] = entry
	} else {
		entry.lastUsed = now
	}
	f.cleanupStaleLimitersLocked(now)
	f.mu.Unlock()

	return &botClientImpl{
		bot:     bot,
		logger:  f.logger,
		limiter: entry.limiter,
	}, nil
}

const (
	limiterEntryTTL   = 24 * time.Hour
	maxLimiterEntries = 1000
)

func (f *botFactoryImpl) cleanupStaleLimitersLocked(now time.Time) {
	for token, entry := range f.limiters {
		if now.Sub(entry.lastUsed) > limiterEntryTTL {
			delete(f.limiters, token)
		}
	}
	if len(f.limiters) <= maxLimiterEntries {
		return
	}

	var oldestToken string
	var oldestTime time.Time
	first := true
	for token, entry := range f.limiters {
		if first || entry.lastUsed.Before(oldestTime) {
			oldestToken = token
			oldestTime = entry.lastUsed
			first = false
		}
	}
	if oldestToken != "" {
		delete(f.limiters, oldestToken)
	}
}

// retryOnRateLimit handles HTTP 429 Too Many Requests errors by sleeping for the required duration.
func (c *botClientImpl) retryOnRateLimit(ctx context.Context, operation func() error) error {
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		if apiErr, ok := err.(*tgbotapi.Error); ok && apiErr.Code == 429 {
			retryAfter := apiErr.ResponseParameters.RetryAfter
			if retryAfter == 0 {
				retryAfter = 5 // Fallback to 5 seconds if not provided
			}

			c.logger.Warn("telegram rate limit hit, sleeping", zap.Int("retry_after", retryAfter), zap.Int("attempt", attempt))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(retryAfter) * time.Second):
				continue
			}
		}

		return err // Not a rate limit error, return immediately
	}
	return fmt.Errorf("telegram: max retries exceeded for operation")
}

// GetBot returns the underlying go-telegram-bot-api instance.
func (c *botClientImpl) GetBot() *tgbotapi.BotAPI {
	return c.bot
}

func (c *botClientImpl) Request(ctx context.Context, chattable tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	var response *tgbotapi.APIResponse
	err := c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(chattable)
		if err != nil {
			return err
		}
		response = resp
		if response == nil {
			return fmt.Errorf("telegram: empty API response")
		}
		if !response.Ok {
			return fmt.Errorf("telegram: api response not ok: %s", response.Description)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *botClientImpl) Send(ctx context.Context, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return tgbotapi.Message{}, fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	var result tgbotapi.Message
	err := c.retryOnRateLimit(ctx, func() error {
		c.logger.Debug("sending message", zap.Any("chattable", chattable))
		msg, err := c.bot.Send(chattable)
		if err != nil {
			return err
		}
		result = msg
		return nil
	})
	if err != nil {
		c.logger.Error("failed to send message", zap.Error(err), zap.Any("chattable", chattable))
		return tgbotapi.Message{}, err
	}
	return result, nil
}

// SetWebhook sets the webhook URL for the bot.
func (c *botClientImpl) SetWebhook(ctx context.Context, webhookURL string, secretToken string) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	params := make(tgbotapi.Params)
	params.AddNonEmpty("url", webhookURL)
	if secretToken != "" {
		params.AddNonEmpty("secret_token", secretToken)
	}

	c.logger.Info("telegram: setting webhook", zap.String("url", webhookURL))

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.MakeRequest("setWebhook", params)
		if err != nil {
			return err
		}
		if !resp.Ok {
			return fmt.Errorf("telegram: failed to set webhook, api response: %s", resp.Description)
		}
		return nil
	})
}

// DeleteWebhook deletes the current webhook.
func (c *botClientImpl) DeleteWebhook(ctx context.Context) error {
	config := tgbotapi.DeleteWebhookConfig{
		DropPendingUpdates: true,
	}
	_, err := c.Request(ctx, config)
	if err != nil {
		return fmt.Errorf("telegram: failed to delete webhook: %w", err)
	}
	return nil
}

// GetWebhookInfo retrieves current webhook status.
func (c *botClientImpl) GetWebhookInfo(ctx context.Context) (tgbotapi.WebhookInfo, error) {
	return c.bot.GetWebhookInfo()
}

// SendMessage sends a text message to a chat.
func (c *botClientImpl) SendMessage(ctx context.Context, chatID int64, text string) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML

	_, err := c.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("telegram: failed to send message: %w", err)
	}
	return nil
}

// KickChatMember removes a member from a chat.
func (c *botClientImpl) KickChatMember(ctx context.Context, chatID int64, userID int64, untilDate time.Time) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		UntilDate: untilDate.Unix(),
	}

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(config)
		if err != nil {
			return fmt.Errorf("telegram: failed to kick chat member: %w", err)
		}
		if !resp.Ok {
			return fmt.Errorf("telegram: failed to kick chat member, api response: %s", resp.Description)
		}
		return nil
	})
}

// UnbanChatMember unbans a member from a chat.
func (c *botClientImpl) UnbanChatMember(ctx context.Context, chatID int64, userID int64, onlyIfBanned bool) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		OnlyIfBanned: onlyIfBanned,
	}

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(config)
		if err != nil {
			return fmt.Errorf("telegram: failed to unban chat member: %w", err)
		}
		if !resp.Ok {
			return fmt.Errorf("telegram: failed to unban chat member, api response: %s", resp.Description)
		}
		return nil
	})
}

// ApproveChatJoinRequest approves a user's request to join a chat.
func (c *botClientImpl) ApproveChatJoinRequest(ctx context.Context, chatID int64, userID int64) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.ApproveChatJoinRequestConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
		UserID: userID,
	}

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(config)
		if err != nil {
			return fmt.Errorf("telegram: failed to approve join request: %w", err)
		}

		if !resp.Ok {
			return fmt.Errorf("telegram: failed to approve join request, api response: %s", resp.Description)
		}

		return nil
	})
}

// DeclineChatJoinRequest declines a user's request to join a chat.
func (c *botClientImpl) DeclineChatJoinRequest(ctx context.Context, chatID int64, userID int64) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	params := make(tgbotapi.Params)
	params.AddNonZero64("chat_id", chatID)
	params.AddNonZero64("user_id", userID)

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.MakeRequest("declineChatJoinRequest", params)
		if err != nil {
			return fmt.Errorf("telegram: failed to decline join request: %w", err)
		}

		if !resp.Ok {
			return fmt.Errorf("telegram: failed to decline join request, api response: %s", resp.Description)
		}

		return nil
	})
}

// CreateChatInviteLink creates an additional invite link for a chat.
func (c *botClientImpl) CreateChatInviteLink(ctx context.Context, chatID int64, name string, createsJoinRequest bool) (string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.CreateChatInviteLinkConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
		Name: name,
	}

	if createsJoinRequest {
		config.CreatesJoinRequest = true
	} else {
		config.MemberLimit = 1 // Fallback limit
	}

	var inviteLink string
	err := c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(config)
		if err != nil {
			return fmt.Errorf("telegram: failed to create invite link: %w", err)
		}

		if !resp.Ok {
			return fmt.Errorf("telegram: failed to create invite link, api response: %s", resp.Description)
		}

		var link tgbotapi.ChatInviteLink
		if err := json.Unmarshal(resp.Result, &link); err != nil {
			return fmt.Errorf("telegram: failed to parse invite link: %w", err)
		}

		inviteLink = link.InviteLink
		return nil
	})

	if err != nil {
		return "", err
	}

	return inviteLink, nil
}

// RevokeChatInviteLink revokes a previously created invite link.
func (c *botClientImpl) RevokeChatInviteLink(ctx context.Context, chatID int64, inviteLink string) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.RevokeChatInviteLinkConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
		InviteLink: inviteLink,
	}

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(config)
		if err != nil {
			return err
		}
		if !resp.Ok {
			return fmt.Errorf("telegram: failed to revoke invite link, api response: %s", resp.Description)
		}
		return nil
	})
}

// GetChatMember gets information about a member of a chat.
func (c *botClientImpl) GetChatMember(ctx context.Context, chatID int64, userID int64) (tgbotapi.ChatMember, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return tgbotapi.ChatMember{}, fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	}

	var member tgbotapi.ChatMember
	err := c.retryOnRateLimit(ctx, func() error {
		result, err := c.bot.GetChatMember(config)
		if err != nil {
			return err
		}
		member = result
		return nil
	})
	if err != nil {
		return tgbotapi.ChatMember{}, fmt.Errorf("telegram: failed to get chat member: %w", err)
	}

	return member, nil
}

// SendDocument sends a document (file) to a chat with rate limiting and retry on 429.
func (c *botClientImpl) SendDocument(ctx context.Context, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return tgbotapi.Message{}, fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	var result tgbotapi.Message
	err := c.retryOnRateLimit(ctx, func() error {
		msg, err := c.bot.Send(chattable)
		if err != nil {
			return err
		}
		result = msg
		return nil
	})
	if err != nil {
		return tgbotapi.Message{}, err
	}
	return result, nil
}

// GetChatMembersCount retrieves the number of members in a chat.
func (c *botClientImpl) GetChatMembersCount(ctx context.Context, chatID int64) (int, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return 0, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	config := tgbotapi.ChatMemberCountConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}

	c.logger.Debug("getting chat member count", zap.Int64("chat_id", chatID))
	count, err := c.bot.GetChatMembersCount(config)
	if err != nil {
		c.logger.Error("failed to get chat member count", zap.Int64("chat_id", chatID), zap.Error(err))
		return 0, fmt.Errorf("get chat members count API call: %w", err)
	}
	return count, nil
}

// LeaveChat makes the bot leave a chat.
func (c *botClientImpl) LeaveChat(ctx context.Context, chatID int64) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("telegram: rate limiter wait failed: %w", err)
	}

	config := tgbotapi.LeaveChatConfig{
		ChatID: chatID,
	}

	return c.retryOnRateLimit(ctx, func() error {
		resp, err := c.bot.Request(config)
		if err != nil {
			return fmt.Errorf("telegram: failed to leave chat: %w", err)
		}
		if !resp.Ok {
			return fmt.Errorf("telegram: failed to leave chat, api response: %s", resp.Description)
		}
		return nil
	})
}
