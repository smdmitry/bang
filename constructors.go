package bang

import "errors"

// ─── Shorthand constructors ─────────────────────────────────────────────────
// Each returns a new *Error with the appropriate class. Use the fluent builder
// to attach wrap, code, safe/debug data, etc.
//
//   bang.NotFound()
//   bang.NotFound().Wrap(err).Code("users.get").Debug("id", id)
//   bang.Database().Wrap(err).Code("repo.save")

func InternalServer() *Error       { return newError(ClassInternalServer, 1) }
func NotFound() *Error             { return newError(ClassNotFound, 1) }
func BadRequest() *Error           { return newError(ClassBadRequest, 1) }
func Unauthorized() *Error         { return newError(ClassUnauthorized, 1) }
func Forbidden() *Error            { return newError(ClassForbidden, 1) }
func Conflict() *Error             { return newError(ClassConflict, 1) }
func TooManyRequests() *Error      { return newError(ClassTooManyRequests, 1) }
func Database() *Error             { return newError(ClassDatabase, 1) }
func DuplicateKey() *Error         { return newError(ClassDuplicateKey, 1) }
func ForeignKeyViolation() *Error  { return newError(ClassForeignKeyViolation, 1) }
func ExternalService() *Error      { return newError(ClassExternalService, 1) }
func ExternalServiceError() *Error { return newError(ClassExternalService, 1) }
func Network() *Error              { return newError(ClassNetwork, 1) }
func NetworkError() *Error         { return newError(ClassNetwork, 1) }
func Timeout() *Error              { return newError(ClassTimeout, 1) }
func TimeoutError() *Error         { return newError(ClassTimeout, 1) }
func Serialization() *Error        { return newError(ClassSerialization, 1) }
func SerializationError() *Error   { return newError(ClassSerialization, 1) }
func Unexpected() *Error           { return newError(ClassUnexpected, 1) }
func NotImplemented() *Error       { return newError(ClassNotImplemented, 1) }
func Locked() *Error               { return newError(ClassLocked, 1) }

// New creates an error from a registered code. If the code has a registered
// CodeCfg, its class/title/message/httpCode are applied. If only a message
// is registered, it becomes the message template. If the code is not registered
// at all, ClassUnexpected is used with the code set.
func New(code string) *Error {
	e := newError(ClassUnexpected, 1)
	e.code = code

	// Apply registered code config.
	if cfg, ok := codeRegistry[code]; ok {
		if cfg.Class != "" {
			meta := getClassMeta(cfg.Class)
			e.class = cfg.Class
			e.title = meta.Title
			e.message = meta.Message
		}
		if cfg.Title != "" {
			e.title = cfg.Title
		}
		if cfg.Msg != "" {
			e.msgTpl = cfg.Msg
			e.resolveTemplateIfReady()
		}
		if cfg.HTTPCode != 0 {
			e.httpStatus = cfg.HTTPCode
		}
		return e
	}

	// Apply registered message.
	if msg, ok := messageRegistry[code]; ok {
		e.msgTpl = msg
		e.resolveTemplateIfReady()
	}

	return e
}

func Wrap(err error) *Error {
	return &Error{cause: err}
}

// FromClass creates an error with an arbitrary class.
func FromClass(class Class) *Error {
	return newError(class, 1)
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
