package ratelimit

import (
	"context"
	"strings"
	"time"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const tokenBucketScript = `
local key = KEYS[1]

local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local data = redis.call("HMGET", key, "tokens", "timestamp")

local tokens = tonumber(data[1])
local last = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last = now
end

local delta = math.max(0, now - last)
local refill = delta * refill_rate
tokens = math.min(capacity, tokens + refill)

local allowed = tokens >= requested

if allowed then
    tokens = tokens - requested
end

redis.call("HMSET", key, "tokens", tokens, "timestamp", now)
redis.call("EXPIRE", key, 60)

return {allowed and 1 or 0, tostring(tokens)}
`

type Config struct {
	Capacity   int
	RefillRate float64
	KeyPrefix  string
	KeyFunc    func(c fiber.Ctx) string
	Next       func(c fiber.Ctx) bool
}

type RateLimiter struct {
	rdb    *redis.Client
	script *redis.Script
	config Config
}

func New(rdb *redis.Client, cfg Config) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		script: redis.NewScript(tokenBucketScript),
		config: cfg,
	}
}

func SkipUnprotectedPaths(c fiber.Ctx) bool {
	path := c.Path()
	return strings.HasPrefix(path, "/webhooks") ||
		strings.HasPrefix(path, "/metrics") ||
		path == "/health"
}

// ClientIP extracts the real client IP, preferring X-Forwarded-For
// when behind a reverse proxy (OpenShift). Falls back to c.IP().
func ClientIP(c fiber.Ctx) string {
	xff := c.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[len(parts)-1])
		if ip != "" {
			return ip
		}
	}
	return c.IP()
}

func (rl *RateLimiter) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if rl.config.Next != nil && rl.config.Next(c) {
			return c.Next()
		}

		key := rl.config.KeyPrefix
		if rl.config.KeyFunc != nil {
			key += rl.config.KeyFunc(c)
		}

		ctx, cancel := context.WithTimeout(c.Context(), 50*time.Millisecond)
		defer cancel()

		res, err := rl.script.Run(ctx, rl.rdb, []string{key},
			rl.config.Capacity,
			rl.config.RefillRate,
			float64(time.Now().Unix()),
			1,
		).Result()
		if err != nil {
			// fail open: allow request when Redis is unavailable
			zap.L().Error("rate limiter redis error", zap.String("key", key), zap.Error(err))
			return c.Next()
		}

		result, ok := res.([]interface{})
		if !ok || len(result) < 1 {
			return c.Next()
		}

		allowed, ok := result[0].(int64)
		if !ok {
			return c.Next()
		}

		if allowed == 0 {
			return helper.TooManyRequests(c, "Terlalu banyak permintaan. Silakan coba lagi nanti.")
		}

		return c.Next()
	}
}
