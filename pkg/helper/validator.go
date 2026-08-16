package helper

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

func ValidateStruct(v *validator.Validate, data interface{}) map[string]string {
	if v == nil {
		return map[string]string{"_": "validator tidak dikonfigurasi"}
	}
	err := v.Struct(data)
	if err == nil {
		return nil
	}

	// Bare assertion panic pada InvalidValidationError (nil/non-struct input).
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return map[string]string{"_": "data tidak dapat divalidasi"}
	}

	errors := make(map[string]string)
	for _, e := range validationErrors {
		errors[e.Field()] = translateError(e)
	}
	return errors
}

func translateError(e validator.FieldError) string {
	field := e.Field()

	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s wajib diisi", field)
	case "email":
		return fmt.Sprintf("%s harus berupa alamat email yang valid", field)
	case "min":
		if e.Type().Kind().String() == "string" {
			return fmt.Sprintf("%s minimal %s karakter", field, e.Param())
		}
		return fmt.Sprintf("%s minimal %s", field, e.Param())
	case "max":
		if e.Type().Kind().String() == "string" {
			return fmt.Sprintf("%s maksimal %s karakter", field, e.Param())
		}
		return fmt.Sprintf("%s maksimal %s", field, e.Param())
	case "gt":
		return fmt.Sprintf("%s harus lebih dari %s", field, e.Param())
	case "gte":
		return fmt.Sprintf("%s harus lebih dari atau sama dengan %s", field, e.Param())
	case "lt":
		return fmt.Sprintf("%s harus kurang dari %s", field, e.Param())
	case "lte":
		return fmt.Sprintf("%s harus kurang dari atau sama dengan %s", field, e.Param())
	case "oneof":
		return fmt.Sprintf("%s harus salah satu dari: %s", field, strings.ReplaceAll(e.Param(), " ", ", "))
	case "uuid", "uuid4":
		return fmt.Sprintf("%s harus berupa UUID yang valid", field)
	case "numeric":
		return fmt.Sprintf("%s harus berupa angka", field)
	case "alpha":
		return fmt.Sprintf("%s hanya boleh berisi huruf", field)
	case "alphanum":
		return fmt.Sprintf("%s hanya boleh berisi huruf dan angka", field)
	case "len":
		return fmt.Sprintf("%s harus tepat %s karakter", field, e.Param())
	case "url":
		return fmt.Sprintf("%s harus berupa URL yang valid", field)
	case "no_hp":
		return fmt.Sprintf("%s harus berupa nomor HP yang valid (10-15 digit)", field)
	case "nominal":
		return fmt.Sprintf("%s harus lebih dari 0", field)
	default:
		return fmt.Sprintf("%s tidak valid", field)
	}
}
