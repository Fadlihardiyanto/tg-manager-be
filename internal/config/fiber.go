package config

import (
	"errors"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"go.uber.org/zap"
)

// NewFiber creates a new Fiber app instance with the provided configuration.
func NewFiber(cfg *AppConfig) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.Name,
		ErrorHandler: NewErrorHandler(),
		JSONEncoder:  sonic.Marshal,
		JSONDecoder:  sonic.Unmarshal,
		BodyLimit:    1 * 1024 * 1024,
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			if cfg.Env == "development" {
				return true
			}
			return origin == cfg.AllowedOrigin
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

	app.Use(func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Set("X-XSS-Protection", "0")
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Set("Content-Security-Policy", "default-src 'self'")
		return c.Next()
	})

	return app
}

// NewErrorHandler returns a centralized error handler that maps domain errors
// to appropriate HTTP responses using zap for logging.
func NewErrorHandler() fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		zap.L().Error("HTTP error", zap.Error(err), zap.String("path", c.Path()), zap.String("method", c.Method()))

		var errConflict *helper.ErrConflict
		if errors.As(err, &errConflict) {
			return helper.Conflict(c, errConflict.Error())
		}

		var errNotFound *helper.ErrNotFound
		if errors.As(err, &errNotFound) {
			return helper.NotFound(c, errNotFound.Error())
		}

		var errBadRequest *helper.ErrBadRequest
		if errors.As(err, &errBadRequest) {
			return helper.BadRequest(c, errBadRequest.Error())
		}

		var errForbidden *helper.ErrForbidden
		if errors.As(err, &errForbidden) {
			return helper.Forbidden(c, errForbidden.Error())
		}

		var errUnauthorized *helper.ErrUnauthorized
		if errors.As(err, &errUnauthorized) {
			return helper.Unauthorized(c, errUnauthorized.Error())
		}

		var errUnprocessable *helper.ErrUnprocessable
		if errors.As(err, &errUnprocessable) {
			return helper.UnprocessableEntity(c, errUnprocessable.Error())
		}

		var errTooManyRequests *helper.ErrTooManyRequests
		if errors.As(err, &errTooManyRequests) {
			return helper.TooManyRequests(c, errTooManyRequests.Error())
		}

		var fiberErr *fiber.Error
		if errors.As(err, &fiberErr) {
			return c.Status(fiberErr.Code).JSON(helper.Response{
				Success: false,
				Code:    fiberErr.Code,
				Message: fiberErr.Message,
			})
		}

		return helper.InternalError(c, "Terjadi kesalahan pada server")
	}
}
