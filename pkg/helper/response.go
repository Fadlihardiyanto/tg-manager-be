package helper

import (
	"github.com/Fadlihardiyanto/telegram-management-app/pkg/logger"
	"github.com/gofiber/fiber/v3"
)

type Response struct {
	Success   bool        `json:"success"`
	Code      int         `json:"code,omitempty"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Errors    interface{} `json:"errors,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Meta      *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func Success(c fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success:   true,
		Code:      fiber.StatusOK,
		Message:   message,
		Data:      data,
		RequestID: logger.RequestID(c),
	})
}

func SuccessWithMeta(c fiber.Ctx, message string, data interface{}, meta *Meta) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success:   true,
		Code:      fiber.StatusOK,
		Message:   message,
		Data:      data,
		Meta:      meta,
		RequestID: logger.RequestID(c),
	})
}

func Created(c fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success:   true,
		Code:      fiber.StatusCreated,
		Message:   message,
		Data:      data,
		RequestID: logger.RequestID(c),
	})
}

func BadRequest(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(Response{
		Code:      fiber.StatusBadRequest,
		Success:   false,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func Unauthorized(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Response{
		Code:      fiber.StatusUnauthorized,
		Success:   false,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func Forbidden(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusForbidden).JSON(Response{
		Success:   false,
		Code:      fiber.StatusForbidden,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func NotFound(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusNotFound).JSON(Response{
		Success:   false,
		Code:      fiber.StatusNotFound,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func UnprocessableEntity(c fiber.Ctx, errors interface{}) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(Response{
		Success:   false,
		Code:      fiber.StatusUnprocessableEntity,
		Message:   "Validasi input gagal",
		Errors:    errors,
		RequestID: logger.RequestID(c),
	})
}

func InternalError(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(Response{
		Success:   false,
		Code:      fiber.StatusInternalServerError,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func Conflict(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(Response{
		Success:   false,
		Code:      fiber.StatusConflict,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func TooManyRequests(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(Response{
		Success:   false,
		Code:      fiber.StatusTooManyRequests,
		Message:   message,
		RequestID: logger.RequestID(c),
	})
}

func NewMeta(page, limit int, total int64) *Meta {
	// Guard: limit=0 (query param tidak ter-clamp di beberapa controller)
	// → division by zero → panic. Default 1 halaman per limit 20.
	if limit < 1 {
		limit = 20
	}
	if page < 1 {
		page = 1
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	return &Meta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
