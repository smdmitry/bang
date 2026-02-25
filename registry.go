package bang

import (
	"fmt"
	"sync"
)

// ─── Global registries ──────────────────────────────────────────────────────

var (
	registryMu      sync.RWMutex
	messageRegistry = make(map[string]string)  // code → message template
	codeRegistry    = make(map[string]CodeCfg) // code → full config
)

// CodeCfg defines all optional parameters for a registered error code.
type CodeCfg struct {
	Class    Class  // error class (optional; if empty, caller provides it)
	Title    string // custom title (optional)
	Msg      string // message template with {{.key}} placeholders
	HTTPCode int    // HTTP status override (optional)
}

// MustRegisterMessages registers message templates for error codes.
// Panics if any code is already registered.
//
//	bang.MustRegisterMessages(map[string]string{
//	    "task.not_found": "Task '{{.key}}' not found",
//	})
func MustRegisterMessages(msgs map[string]string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for code, msg := range msgs {
		if _, exists := messageRegistry[code]; exists {
			panic(fmt.Sprintf("bang: message for code %q already registered", code))
		}
		if _, exists := codeRegistry[code]; exists {
			panic(fmt.Sprintf("bang: code %q already registered via MustRegisterCodes", code))
		}
		messageRegistry[code] = msg
	}
}

// MustRegisterCodes registers full configurations for error codes.
// Panics if any code is already registered.
//
//	bang.MustRegisterCodes(map[string]bang.CodeCfg{
//	    "task.not_found": {
//	        Class: bang.ClassNotFound,
//	        Title: "Task Not Found",
//	        Msg:   "Task '{{.key}}' not found",
//	    },
//	})
func MustRegisterCodes(cfgs map[string]CodeCfg) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for code, cfg := range cfgs {
		if _, exists := codeRegistry[code]; exists {
			panic(fmt.Sprintf("bang: code %q already registered", code))
		}
		if _, exists := messageRegistry[code]; exists {
			panic(fmt.Sprintf("bang: code %q already registered via MustRegisterMessages", code))
		}
		codeRegistry[code] = cfg
	}
}

// ─── Factory — callable error factory from a registered code ────────────────

// ErrFactory is a function that creates errors from a registered code.
// Call it to get a new *Error, use .Is() for matching.
//
//	var TaskNotFound = bang.Factory("task.not_found")
//
//	TaskNotFound()             // new error
//	TaskNotFound().Debug(...)  // with debug data
//	TaskNotFound.Is(err)       // check if err matches
type ErrFactory func() *Error

// Factory creates an ErrFactory for the given code.
// The code should be pre-registered via MustRegisterMessages or MustRegisterCodes,
// but this is not required (defaults will be used if not registered).
func Factory(code string) ErrFactory {
	return func() *Error {
		e := newError(ClassUnexpected, 2)
		e.code = code

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
		} else if msg, ok := messageRegistry[code]; ok {
			e.msgTpl = msg
			e.resolveTemplateIfReady()
		}

		return e
	}
}

// Is reports whether err matches this factory's error code.
func (f ErrFactory) Is(err error) bool {
	ref := f()
	if ref.code != "" {
		return HasCode(err, ref.code)
	}
	return HasClass(err, ref.class)
}

// ─── Register — fluent builder for error definitions ────────────────────────

// RegisterBuilder provides a fluent API for defining error codes.
type RegisterBuilder struct {
	code    string
	class   Class
	title   string
	message string
}

// Register starts building an error definition for the given code.
//
//	var TaskNotFound = bang.Register("task.not_found").
//	    Class(bang.ClassNotFound).
//	    Message("Task '{{.key}}' not found").
//	    New()
func Register(code string) *RegisterBuilder {
	return &RegisterBuilder{code: code}
}

// From copies class, title, and message from an existing error prototype.
func (b *RegisterBuilder) From(proto *Error) *RegisterBuilder {
	b.class = proto.class
	b.title = proto.title
	b.message = proto.message
	return b
}

// Class sets the error class.
func (b *RegisterBuilder) Class(class Class) *RegisterBuilder {
	b.class = class
	if b.title == "" {
		b.title = getClassMeta(class).Title
	}
	return b
}

// Title sets a custom title.
func (b *RegisterBuilder) Title(title string) *RegisterBuilder {
	b.title = title
	return b
}

// Message sets a message template with {{.key}} placeholders.
func (b *RegisterBuilder) Message(msg string) *RegisterBuilder {
	b.message = msg
	return b
}

// TypedRegisterBuilder provides a fluent API for defining typed error codes.
//
// Note: Go does not support generic methods on non-generic types, so
// Register(...).Typed[T]() is not possible. Use RegisterTyped[T](...) instead.
type TypedRegisterBuilder[T any] struct {
	builder *RegisterBuilder
}

// RegisterTyped starts building a typed error definition for the given code.
//
//	var TaskNotFound = bang.RegisterTyped[TaskNotFoundData]("task.not_found").
//	    Class(bang.ClassNotFound).
//	    Message("Task '{{.task_id}}' not found").
//	    New()
func RegisterTyped[T any](code string) *TypedRegisterBuilder[T] {
	return &TypedRegisterBuilder[T]{builder: Register(code)}
}

// From copies class, title, and message from an existing error prototype.
func (b *TypedRegisterBuilder[T]) From(proto *Error) *TypedRegisterBuilder[T] {
	b.builder.From(proto)
	return b
}

// Class sets the error class.
func (b *TypedRegisterBuilder[T]) Class(class Class) *TypedRegisterBuilder[T] {
	b.builder.Class(class)
	return b
}

// Title sets a custom title.
func (b *TypedRegisterBuilder[T]) Title(title string) *TypedRegisterBuilder[T] {
	b.builder.Title(title)
	return b
}

// Message sets a message template with {{.key}} placeholders.
func (b *TypedRegisterBuilder[T]) Message(msg string) *TypedRegisterBuilder[T] {
	b.builder.Message(msg)
	return b
}

// New finalises the typed registration and returns a TypedFactory.
func (b *TypedRegisterBuilder[T]) New() TypedFactory[T] {
	return Typed[T](b.builder)
}

// New finalises the registration and returns an ErrFactory.
// It registers the code in the global registry.
func (b *RegisterBuilder) New() ErrFactory {
	class := b.class
	if class == "" {
		class = ClassUnexpected
	}

	cfg := CodeCfg{
		Class: class,
		Title: b.title,
		Msg:   b.message,
	}

	registryMu.Lock()
	codeRegistry[b.code] = cfg
	registryMu.Unlock()

	code := b.code
	title := b.title
	msg := b.message

	return func() *Error {
		e := newError(class, 2)
		e.code = code
		if title != "" {
			e.title = title
		}
		if msg != "" {
			e.msgTpl = msg
			e.resolveTemplateIfReady()
		}
		return e
	}
}

// ─── TypedFactory — factory with typed data ─────────────────────────────────

// TypedFactory is a function that creates errors requiring typed data.
//
//	var TaskNotFound = bang.RegisterTyped[TaskNotFoundData]("task.not_found").
//	    Class(bang.ClassNotFound).
//	    Message("Task '{{.task_id}}' not found").
//	    New()
//
//	TaskNotFound(TaskNotFoundData{TaskID: "123"}) // data required
type TypedFactory[T any] func(data T) *Error

// Typed finalises the registration and returns a TypedFactory.
func Typed[T any](b *RegisterBuilder) TypedFactory[T] {
	class := b.class
	if class == "" {
		class = ClassUnexpected
	}

	cfg := CodeCfg{
		Class: class,
		Title: b.title,
		Msg:   b.message,
	}

	registryMu.Lock()
	codeRegistry[b.code] = cfg
	registryMu.Unlock()

	code := b.code
	title := b.title
	msg := b.message

	return func(data T) *Error {
		e := newError(class, 2)
		e.code = code
		if title != "" {
			e.title = title
		}
		e.data = data
		if msg != "" {
			e.msgTpl = msg
			e.resolveTemplateIfReady()
		}
		return e
	}
}

// Is reports whether err matches this typed factory's error code.
func (f TypedFactory[T]) Is(err error) bool {
	ref := f(*new(T))
	if ref.code != "" {
		return HasCode(err, ref.code)
	}
	return HasClass(err, ref.class)
}

// GetData extracts the typed data from err.
func (f TypedFactory[T]) GetData(err error) (T, bool) {
	return GetData[T](err)
}

// ─── Template missing key mode ──────────────────────────────────────────────

// MissingKeyMode controls how unresolved template placeholders are handled.
type MissingKeyMode int

const (
	// MissingKeyKeep leaves unresolved {{.xxx}} as-is in the message (default).
	MissingKeyKeep MissingKeyMode = iota
	// MissingKeyRemove removes unresolved {{.xxx}} from the message.
	MissingKeyRemove
	// MissingKeyError treats unresolved placeholders as an error (logs a warning).
	MissingKeyError
)

var (
	missingKeyMu   sync.RWMutex
	missingKeyMode = MissingKeyKeep
)

// SetTemplateMissingKeyMode configures how unresolved template placeholders are handled.
//
//	bang.SetTemplateMissingKeyMode(bang.MissingKeyKeep)   // default: leave as is
//	bang.SetTemplateMissingKeyMode(bang.MissingKeyRemove) // remove placeholders
//	bang.SetTemplateMissingKeyMode(bang.MissingKeyError)  // treat as error
func SetTemplateMissingKeyMode(mode MissingKeyMode) {
	missingKeyMu.Lock()
	defer missingKeyMu.Unlock()
	missingKeyMode = mode
}

func getTemplateMissingKeyMode() MissingKeyMode {
	missingKeyMu.RLock()
	defer missingKeyMu.RUnlock()
	return missingKeyMode
}

// ResetRegistry clears all registered codes and messages. Useful for tests.
func ResetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	messageRegistry = make(map[string]string)
	codeRegistry = make(map[string]CodeCfg)
}
