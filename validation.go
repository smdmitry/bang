package bang

import "errors"

// ValidationField describes a single field validation failure.
type ValidationField struct {
	Key    string `json:"key"`
	Rule   string `json:"rule,omitempty"` // unsafe, hidden from user in HTTP
	Reason string `json:"reason"`
}

// Validation creates a ClassValidation error. Optionally set a code.
//
//	bang.Validation().Code("users.register").
//	    Field("email", "email_format", "Введите корректный email.").
//	    Field("password", "min_length", "Пароль — не менее 8 символов.")
func Validation() *Error {
	return newError(ClassValidation, 1)
}

// Field adds a validation field error. Returns the same *Error for chaining.
func (e *Error) Field(key, rule, reason string) *Error {
	e.validationFields = append(e.validationFields, ValidationField{
		Key: key, Rule: rule, Reason: reason,
	})
	return e
}

// Fields adds multiple validation field errors at once.
func (e *Error) Fields(fields ...ValidationField) *Error {
	e.validationFields = append(e.validationFields, fields...)
	return e
}

// CombineValidation merges multiple validation errors into one.
// Non-validation errors and nils are ignored.
func CombineValidation(errs ...error) *Error {
	var result *Error
	for _, err := range errs {
		if err == nil {
			continue
		}
		var e *Error
		if !errors.As(err, &e) || e.class != ClassValidation {
			continue
		}
		if result == nil {
			result = &Error{
				class:            e.class,
				code:             e.code,
				title:            e.title,
				message:          e.message,
				file:             e.file,
				line:             e.line,
				validationFields: make([]ValidationField, len(e.validationFields)),
			}
			copy(result.validationFields, e.validationFields)
		} else {
			result.validationFields = append(result.validationFields, e.validationFields...)
		}
	}
	return result
}

// IsValidation reports whether err is a validation error.
func IsValidation(err error) bool { return HasClass(err, ClassValidation) }

// ValidationFields extracts the validation field list from an error.
func ValidationFields(err error) []ValidationField {
	var e *Error
	if errors.As(err, &e) {
		return e.validationFields
	}
	return nil
}
