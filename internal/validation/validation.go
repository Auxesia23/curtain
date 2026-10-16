package validation

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validate is the validator instance used by all handlers.
var Validate = validator.New()

// ErrorMessage converts ValidationErrors into a user-friendly message.
func ErrorMessage(err error) string {
	ves, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}
	msgs := make([]string, 0, len(ves))
	for _, fe := range ves {
		msgs = append(msgs, fieldMessage(fe))
	}
	return strings.Join(msgs, ", ")
}

// ValidateVar validates one value against a tag, such as a query parameter.
func ValidateVar(value any, tag string) error {
	return Validate.Var(value, tag)
}

func fieldMessage(fe validator.FieldError) string {
	field := strings.ToLower(fe.Field())
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("valid %s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of [%s]", field, strings.ReplaceAll(fe.Param(), " ", ", "))
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	default:
		return fmt.Sprintf("%s is invalid (%s)", field, fe.Tag())
	}
}
