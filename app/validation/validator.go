package validation

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Struct validates a DTO and returns human-readable messages, or nil when valid.
func Struct(payload interface{}) []string {
	err := validate.Struct(payload)
	if err == nil {
		return nil
	}

	var messages []string
	var invalid *validator.InvalidValidationError
	if ok := asInvalid(err, &invalid); ok {
		return []string{"invalid validation target"}
	}

	for _, fe := range err.(validator.ValidationErrors) {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required", field))
		case "email":
			messages = append(messages, fmt.Sprintf("%s must be a valid email", field))
		case "max":
			messages = append(messages, fmt.Sprintf("%s must be at most %s characters", field, fe.Param()))
		case "oneof":
			messages = append(messages, fmt.Sprintf("%s must be one of: %s", field, fe.Param()))
		default:
			messages = append(messages, fmt.Sprintf("%s is invalid", field))
		}
	}
	return messages
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	if v, ok := err.(*validator.InvalidValidationError); ok {
		*target = v
		return true
	}
	return false
}
