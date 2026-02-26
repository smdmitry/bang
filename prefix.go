package bang

import "strings"

// PrefixFactory builds errors with a shared code prefix and optional defaults.
type PrefixFactory struct {
	prefix     string
	class      Class
	title      string
	message    string
	httpStatus int
}

// Prefix creates a factory with the given code prefix.
//
//	var repoerr = bang.Prefix("project.archive")
func Prefix(prefix string) PrefixFactory {
	return PrefixFactory{prefix: normalizeCodePrefix(prefix)}
}

// Class sets the default class used by New().
func (f PrefixFactory) Class(class Class) PrefixFactory {
	f.class = class
	return f
}

// Title sets the default title used by New() and wrapped adapters.
func (f PrefixFactory) Title(title string) PrefixFactory {
	f.title = title
	return f
}

// Message sets the default message/template used by New() and wrapped adapters.
func (f PrefixFactory) Message(msg string) PrefixFactory {
	f.message = msg
	return f
}

// Msg is an alias for Message.
func (f PrefixFactory) Msg(msg string) PrefixFactory {
	return f.Message(msg)
}

// HTTPStatus sets the default HTTP status used by New() and wrapped adapters.
func (f PrefixFactory) HTTPStatus(code int) PrefixFactory {
	f.httpStatus = code
	return f
}

func (f PrefixFactory) fullCode(code string) string {
	return joinCodePrefix(f.prefix, code)
}

func (f PrefixFactory) newFromClass(class Class) *Error {
	if class == "" {
		class = ClassUnexpected
	}
	e := newError(class, 3) // skip: method → newFromClass → newError
	e.codePrefix = f.prefix
	e.code = f.prefix
	if f.title != "" {
		e.title = f.title
	}
	if f.message != "" {
		e.msgTpl = f.message
		e.resolveTemplateIfReady()
	}
	if f.httpStatus != 0 {
		e.httpStatus = f.httpStatus
	}
	return e
}

// New creates a new error using factory defaults.
//
// If code is provided, it is joined with the configured prefix.
func (f PrefixFactory) New(code ...string) *Error {
	if len(code) > 0 {
		e := New(f.fullCode(code[0]))
		e.codePrefix = f.prefix
		if f.class != "" {
			meta := getClassMeta(f.class)
			e.class = f.class
			e.title = meta.Title
			e.message = meta.Message
		}
		if f.title != "" {
			e.title = f.title
		}
		if f.message != "" {
			e.msgTpl = f.message
			e.resolveTemplateIfReady()
		}
		if f.httpStatus != 0 {
			e.httpStatus = f.httpStatus
		}
		return e
	}
	return f.newFromClass(f.class)
}

// FromClass creates an error with an arbitrary class and the configured prefix.
func (f PrefixFactory) FromClass(class Class) *Error {
	return f.newFromClass(class)
}

func (f PrefixFactory) Wrap(err error) *Error {
	return &Error{cause: err}
}

// Shorthand constructors.
func (f PrefixFactory) InternalServer() *Error      { return f.newFromClass(ClassInternalServer) }
func (f PrefixFactory) Validation() *Error          { return f.newFromClass(ClassValidation) }
func (f PrefixFactory) NotFound() *Error            { return f.newFromClass(ClassNotFound) }
func (f PrefixFactory) BadRequest() *Error          { return f.newFromClass(ClassBadRequest) }
func (f PrefixFactory) Unauthorized() *Error        { return f.newFromClass(ClassUnauthorized) }
func (f PrefixFactory) Forbidden() *Error           { return f.newFromClass(ClassForbidden) }
func (f PrefixFactory) Conflict() *Error            { return f.newFromClass(ClassConflict) }
func (f PrefixFactory) TooManyRequests() *Error     { return f.newFromClass(ClassTooManyRequests) }
func (f PrefixFactory) Database() *Error            { return f.newFromClass(ClassDatabase) }
func (f PrefixFactory) DuplicateKey() *Error        { return f.newFromClass(ClassDuplicateKey) }
func (f PrefixFactory) ForeignKeyViolation() *Error { return f.newFromClass(ClassForeignKeyViolation) }
func (f PrefixFactory) ExternalService() *Error     { return f.newFromClass(ClassExternalService) }
func (f PrefixFactory) ExternalServiceError() *Error {
	return f.newFromClass(ClassExternalService)
}
func (f PrefixFactory) Network() *Error { return f.newFromClass(ClassNetwork) }
func (f PrefixFactory) NetworkError() *Error {
	return f.newFromClass(ClassNetwork)
}
func (f PrefixFactory) Timeout() *Error { return f.newFromClass(ClassTimeout) }
func (f PrefixFactory) TimeoutError() *Error {
	return f.newFromClass(ClassTimeout)
}
func (f PrefixFactory) Serialization() *Error { return f.newFromClass(ClassSerialization) }
func (f PrefixFactory) SerializationError() *Error {
	return f.newFromClass(ClassSerialization)
}
func (f PrefixFactory) Unexpected() *Error     { return f.newFromClass(ClassUnexpected) }
func (f PrefixFactory) NotImplemented() *Error { return f.newFromClass(ClassNotImplemented) }
func (f PrefixFactory) Locked() *Error         { return f.newFromClass(ClassLocked) }

// Define creates an ErrFactory with a code prefixed by this factory.
func (f PrefixFactory) Define(code string, cfgs ...CodeCfg) ErrFactory {
	fullCode := f.fullCode(code)
	var cfg CodeCfg
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	registryMu.Lock()
	codeRegistry[fullCode] = cfg
	registryMu.Unlock()
	return Factory(fullCode)
}

// Register starts building an error definition with a prefixed code.
func (f PrefixFactory) Register(code string) *RegisterBuilder {
	return Register(f.fullCode(code))
}

// Factory creates an ErrFactory for a prefixed code.
func (f PrefixFactory) Factory(code string) ErrFactory {
	return Factory(f.fullCode(code))
}

// WrapDBError wraps DB errors and applies this prefix.
func (f PrefixFactory) WrapDBError(err error, code ...string) *Error {
	e := WrapDBError(err)
	return f.applyWrappedError(e, code...)
}

// WrapRedisError wraps Redis errors and applies this prefix.
func (f PrefixFactory) WrapRedisError(err error, code ...string) *Error {
	e := WrapRedisError(err)
	return f.applyWrappedError(e, code...)
}

func (f PrefixFactory) applyWrappedError(e *Error, code ...string) *Error {
	if e == nil {
		return nil
	}
	e.codePrefix = f.prefix
	if len(code) > 0 {
		e.Code(code[0])
	} else {
		e.code = f.prefix
	}
	if f.title != "" {
		e.title = f.title
	}
	if f.message != "" {
		e.msgTpl = f.message
		e.resolveTemplateIfReady()
	}
	if f.httpStatus != 0 {
		e.httpStatus = f.httpStatus
	}
	return e
}

func normalizeCodePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	return strings.Trim(prefix, ".")
}

func joinCodePrefix(prefix, code string) string {
	prefix = normalizeCodePrefix(prefix)
	code = strings.TrimPrefix(strings.TrimSpace(code), ".")

	switch {
	case prefix == "":
		return code
	case code == "":
		return prefix
	case code == prefix || strings.HasPrefix(code, prefix+"."):
		return code
	default:
		return prefix + "." + code
	}
}
