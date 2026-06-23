package config

import (
	"errors"

	"github.com/Fadlihardiyanto/telegram-management-app/pkg/helper"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"go.uber.org/zap"
)

// NewFiber creates a new Fiber app instance with the provided configuration.
func NewFiber(cfg *AppConfig) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.Name,
		ErrorHandler: NewErrorHandler(),
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

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
