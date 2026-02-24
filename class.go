package bang

import "net/http"

// Class represents a category of error. Each class has a default code, title, message,
// and HTTP status mapping.
type Class string

const (
	ClassInternalServer  Class = "internal_server"
	ClassValidation      Class = "validation"
	ClassBadRequest      Class = "bad_request"
	ClassNotFound        Class = "not_found"
	ClassUnauthorized    Class = "unauthorized"
	ClassForbidden       Class = "forbidden"
	ClassConflict        Class = "conflict"
	ClassTooManyRequests Class = "too_many_requests"

	ClassDatabaseError       Class = "database_error"
	ClassDuplicateKey        Class = "duplicate_key"
	ClassForeignKeyViolation Class = "foreign_key_violation"

	ClassExternalServiceError Class = "external_service_error"
	ClassNetworkError         Class = "network_error"
	ClassTimeoutError         Class = "timeout_error"

	ClassSerializationError Class = "serialization_error"
	ClassUnexpected         Class = "unexpected"
	ClassNotImplemented     Class = "not_impllemented"
)

// classMeta holds the default metadata for each class (internal).
type classMeta struct {
	DefaultCode string
	Title       string
	Message     string
	HTTPStatus  int
}

var classRegistry = map[Class]classMeta{
	ClassInternalServer:  {"internal_server", "Internal Server Error", "An internal error occurred. Please try again later.", http.StatusInternalServerError},
	ClassValidation:      {"validation", "Validation Error", "The request contains invalid data.", http.StatusUnprocessableEntity},
	ClassBadRequest:      {"bad_request", "Bad Request", "The request is invalid.", http.StatusBadRequest},
	ClassNotFound:        {"not_found", "Not Found", "The requested resource was not found.", http.StatusNotFound},
	ClassUnauthorized:    {"unauthorized", "Unauthorized", "Authentication is required.", http.StatusUnauthorized},
	ClassForbidden:       {"forbidden", "Forbidden", "You do not have permission to perform this action.", http.StatusForbidden},
	ClassConflict:        {"conflict", "Conflict", "The request conflicts with the current state.", http.StatusConflict},
	ClassTooManyRequests: {"too_many_requests", "Too Many Requests", "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests},

	ClassDatabaseError:       {"database_error", "Database Error", "A database error occurred.", http.StatusInternalServerError},
	ClassDuplicateKey:        {"duplicate_key", "Duplicate Entry", "A record with the same key already exists.", http.StatusConflict},
	ClassForeignKeyViolation: {"foreign_key_violation", "Reference Error", "The referenced resource does not exist.", http.StatusUnprocessableEntity},

	ClassExternalServiceError: {"external_service_error", "External Service Error", "An external service returned an error.", http.StatusBadGateway},
	ClassNetworkError:         {"network_error", "Network Error", "A network error occurred.", http.StatusBadGateway},
	ClassTimeoutError:         {"timeout_error", "Timeout", "The operation timed out.", http.StatusGatewayTimeout},

	ClassSerializationError: {"serialization_error", "Serialization Error", "Failed to serialize or deserialize data.", http.StatusInternalServerError},
	ClassUnexpected:         {"unexpected", "Unexpected Error", "An unexpected error occurred.", http.StatusInternalServerError},

	ClassNotImplemented: {"not_implemented", "Not Implemented", "Not Implemented.", http.StatusNotImplemented},
}

// ClassDef describes a custom error class for registration.
type ClassDef struct {
	DefaultCode string
	Title       string
	Message     string
	HTTPStatus  int
}

// RegisterClass registers a custom error class.
func RegisterClass(class Class, def ClassDef) {
	classRegistry[class] = classMeta{
		DefaultCode: def.DefaultCode,
		Title:       def.Title,
		Message:     def.Message,
		HTTPStatus:  def.HTTPStatus,
	}
}

func getClassMeta(class Class) classMeta {
	if m, ok := classRegistry[class]; ok {
		return m
	}
	return classRegistry[ClassUnexpected]
}

// HTTPStatus returns the HTTP status code mapped to this class.
func (c Class) HTTPStatus() int { return getClassMeta(c).HTTPStatus }

// String implements fmt.Stringer.
func (c Class) String() string { return string(c) }
