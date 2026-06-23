package config

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

// NewValidator creates a new validator instance with custom validations
// for the TG-Manager domain.
func NewValidator() *validator.Validate {
	v := validator.New()

	// Allow numeric validation tags (min, max, gte, lte) on decimal.Decimal.
	v.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if !field.IsValid() {
			return nil
		}
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return nil
			}
			field = field.Elem()
		}
		val, ok := field.Interface().(decimal.Decimal)
		if !ok {
			return nil
		}
		return val.InexactFloat64()
	}, decimal.Decimal{}, &decimal.Decimal{})

	// Use JSON tag names as field names in error messages
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// telegram_id: must be a positive integer (Telegram user/chat IDs)
	v.RegisterValidation("telegram_id", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() > 0
	})

	// bot_token: basic format validation (digits:alphanumeric)
	v.RegisterValidation("bot_token", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		parts := strings.SplitN(val, ":", 2)
		if len(parts) != 2 {
			return false
		}
		// First part should be digits (bot ID)
		for _, c := range parts[0] {
			if c < '0' || c > '9' {
				return false
			}
		}
		// Second part should be non-empty (token)
		return len(parts[1]) > 0
	})

	// price: must be greater than 0
	v.RegisterValidation("price", func(fl validator.FieldLevel) bool {
		return fl.Field().Float() > 0
	})

	// duration_days: must be at least 1
	v.RegisterValidation("duration_days", func(fl validator.FieldLevel) bool {
		return fl.Field().Int() >= 1
	})

	// slug: URL-safe slug (lowercase alphanumeric and hyphens, no leading/trailing hyphens)
	v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		if len(val) == 0 {
			return false
		}
		if val[0] == '-' || val[len(val)-1] == '-' {
			return false
		}
		for _, c := range val {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
		return true
	})

	return v
}
