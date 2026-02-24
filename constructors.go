package bang

import "errors"

// applyArgs parses flexible constructor arguments: any combination of error and string.
func applyArgs(e *Error, args []any) {
	for _, arg := range args {
		switch v := arg.(type) {
		case error:
			e.cause = v
		case string:
			e.code = v
		}
	}
}

func NewFromClass(class Class, callerSkip int, args []any) *Error {
	return newFromClass(class, callerSkip+1, args)
}
func newFromClass(class Class, callerSkip int, args []any) *Error {
	e := newError(class, callerSkip+1)
	applyArgs(e, args)
	return e
}

// ─── Shorthand constructors ─────────────────────────────────────────────────
// Each accepts any combination of arguments:
//   ()                 — default code, no wrap
//   ("code")           — custom code
//   (err)              — wrap error, default code
//   (err, "code")      — wrap error + custom code

func InternalServer(args ...any) *Error  { return newFromClass(ClassInternalServer, 1, args) }
func NotFound(args ...any) *Error        { return newFromClass(ClassNotFound, 1, args) }
func Unauthorized(args ...any) *Error    { return newFromClass(ClassUnauthorized, 1, args) }
func Forbidden(args ...any) *Error       { return newFromClass(ClassForbidden, 1, args) }
func Conflict(args ...any) *Error        { return newFromClass(ClassConflict, 1, args) }
func TooManyRequests(args ...any) *Error { return newFromClass(ClassTooManyRequests, 1, args) }

func DatabaseError(args ...any) *Error       { return newFromClass(ClassDatabaseError, 1, args) }
func DuplicateKey(args ...any) *Error        { return newFromClass(ClassDuplicateKey, 1, args) }
func ForeignKeyViolation(args ...any) *Error { return newFromClass(ClassForeignKeyViolation, 1, args) }

func ExternalServiceError(args ...any) *Error {
	return newFromClass(ClassExternalServiceError, 1, args)
}
func NetworkError(args ...any) *Error { return newFromClass(ClassNetworkError, 1, args) }
func TimeoutError(args ...any) *Error { return newFromClass(ClassTimeoutError, 1, args) }

func SerializationError(args ...any) *Error { return newFromClass(ClassSerializationError, 1, args) }
func Unexpected(args ...any) *Error         { return newFromClass(ClassUnexpected, 1, args) }

func BadRequest(args ...any) *Error     { return newFromClass(ClassBadRequest, 1, args) }
func NotImplemented(args ...any) *Error { return newFromClass(ClassNotImplemented, 1, args) }

// New creates an error with an arbitrary class.
func New(class Class, args ...any) *Error {
	return newFromClass(class, 1, args)
}

// WrapUnknown wraps a non-bang error as ClassUnexpected.
// If err is already an *Error, it is returned as-is.
// If err is nil, returns nil.
func WrapUnknown(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return newError(ClassUnexpected, 1).Wrap(err)
}

var ErrNotFound = NotFound()
