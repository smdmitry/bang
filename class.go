package bang

import "net/http"

// Map is a convenience alias for untyped key-value data.
type Map = map[string]any

// Class represents a category of error. Each class has default code, title,
// message, and HTTP status mapping.
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

	ClassDatabase Class = "database"

	ClassDuplicateKey        Class = "duplicate_key"
	ClassForeignKeyViolation Class = "foreign_key_violation"

	ClassExternalService Class = "external_service"
	ClassNetwork         Class = "network"
	ClassTimeout         Class = "timeout"

	ClassSerialization  Class = "serialization"
	ClassUnexpected     Class = "unexpected"
	ClassNotImplemented Class = "not_implemented"
	ClassLocked         Class = "locked"
)

// classMeta holds default metadata for each class.
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

	ClassDatabase:            {"database", "Database Error", "A database error occurred.", http.StatusInternalServerError},
	ClassDuplicateKey:        {"duplicate_key", "Duplicate Entry", "A record with the same key already exists.", http.StatusConflict},
	ClassForeignKeyViolation: {"foreign_key_violation", "Reference Error", "The referenced resource does not exist.", http.StatusUnprocessableEntity},

	ClassExternalService: {"external_service", "External Service Error", "An external service returned an error.", http.StatusBadGateway},
	ClassNetwork:         {"network", "Network Error", "A network error occurred.", http.StatusBadGateway},
	ClassTimeout:         {"timeout", "Timeout", "The operation timed out.", http.StatusGatewayTimeout},

	ClassSerialization: {"serialization", "Serialization Error", "Failed to serialize or deserialize data.", http.StatusInternalServerError},
	ClassUnexpected:    {"unexpected", "Unexpected Error", "An unexpected error occurred.", http.StatusInternalServerError},

	ClassNotImplemented: {"not_implemented", "Not Implemented", "Not Implemented.", http.StatusNotImplemented},
	ClassLocked:         {"locked", "Locked", "Locked.", http.StatusLocked},
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

// HTTPStatus returns the HTTP status code for this class.
func (c Class) HTTPStatus() int { return getClassMeta(c).HTTPStatus }

// String implements fmt.Stringer.
func (c Class) String() string { return string(c) }
