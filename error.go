package bang

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Error is the core error type. It implements the error interface and supports
// a fluent builder pattern for easy construction.
type Error struct {
	class      Class
	codePrefix string // optional code prefix applied by prefixed factories
	code       string
	title      string
	message    string
	msgTpl     string // message template with {{.key}} placeholders

	data  any            // typed data (struct with expose tags)
	safe  map[string]any // user-visible untyped data
	debug map[string]any // debug-only untyped data

	cause error // wrapped error

	validationFields []ValidationField

	stack      []uintptr
	file       string // auto-captured source file
	line       int    // auto-captured source line
	httpStatus int    // optional HTTP status override
}

// newError creates a new Error, auto-capturing file and line of the caller.
func newError(class Class, callerSkip int) *Error {
	meta := getClassMeta(class)
	e := &Error{
		class:   class,
		code:    meta.DefaultCode,
		title:   meta.Title,
		message: meta.Message,
	}
	if _, file, line, ok := runtime.Caller(callerSkip + 1); ok {
		e.file = file
		e.line = line
	}
	return e
}

// ─── Builder methods ────────────────────────────────────────────────────────

// Wrap wraps another error as the cause.
func (e *Error) Wrap(err error) *Error {
	if err != nil {
		e.cause = err
	}
	return e
}

func (e *Error) inheritParent(parent *Error) {
	e.class = parent.class
	e.codePrefix = parent.codePrefix
	e.code = parent.code
	e.title = parent.title
	e.message = parent.message
	e.msgTpl = parent.msgTpl
	e.data = parent.data
	e.httpStatus = parent.httpStatus

	if len(parent.safe) > 0 {
		e.safe = make(map[string]any, len(parent.safe))
		for k, v := range parent.safe {
			e.safe[k] = v
		}
	} else {
		e.safe = nil
	}
	if len(parent.debug) > 0 {
		e.debug = make(map[string]any, len(parent.debug))
		for k, v := range parent.debug {
			e.debug[k] = v
		}
	} else {
		e.debug = nil
	}
	if len(parent.validationFields) > 0 {
		e.validationFields = make([]ValidationField, len(parent.validationFields))
		copy(e.validationFields, parent.validationFields)
	} else {
		e.validationFields = nil
	}
}

// Code sets a unique error code. Recommended format: module.submodule.error_descr
func (e *Error) Code(code string) *Error {
	if e.codePrefix != "" {
		code = joinCodePrefix(e.codePrefix, code)
	}
	e.code = code
	// Check if this code has a registered config and apply it.
	if cfg, ok := codeRegistry[code]; ok {
		if cfg.Class != "" {
			e.class = cfg.Class
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
	}
	if msg, ok := messageRegistry[code]; ok && e.msgTpl == "" {
		e.msgTpl = msg
		e.resolveTemplateIfReady()
	}
	return e
}

// Title overrides the default title (rarely needed).
func (e *Error) Title(title string) *Error {
	e.title = title
	return e
}

// Msg sets a custom user-facing message.
func (e *Error) Msg(message string) *Error {
	e.message = message
	e.msgTpl = message // store as template in case it has placeholders
	e.resolveTemplateIfReady()
	return e
}

// Msgf sets a user-facing message using fmt.Sprintf formatting.
func (e *Error) Msgf(format string, args ...any) *Error {
	e.message = fmt.Sprintf(format, args...)
	e.msgTpl = "" // explicit format, no template
	return e
}

// Safe adds user-visible key-value pairs. Keys and values alternate.
//
//	e.Safe("key", "value", "key2", "value2")
func (e *Error) Safe(kvs ...any) *Error {
	if e.safe == nil {
		e.safe = make(map[string]any)
	}
	for i := 0; i+1 < len(kvs); i += 2 {
		if k, ok := kvs[i].(string); ok {
			e.safe[k] = kvs[i+1]
		}
	}
	e.resolveTemplateIfReady()
	return e
}

// SafeMap adds user-visible key-value pairs from a map.
func (e *Error) SafeMap(m Map) *Error {
	if e.safe == nil {
		e.safe = make(map[string]any)
	}
	for k, v := range m {
		e.safe[k] = v
	}
	e.resolveTemplateIfReady()
	return e
}

// Debug adds debug-only key-value pairs (unsafe, not shown to users).
//
//	e.Debug("keyd", "valued", "key2", "value2")
func (e *Error) Debug(kvs ...any) *Error {
	if e.debug == nil {
		e.debug = make(map[string]any)
	}
	for i := 0; i+1 < len(kvs); i += 2 {
		if k, ok := kvs[i].(string); ok {
			e.debug[k] = kvs[i+1]
		}
	}
	e.resolveTemplateIfReady()
	return e
}

// DebugMap adds debug-only key-value pairs from a map.
func (e *Error) DebugMap(m Map) *Error {
	if e.debug == nil {
		e.debug = make(map[string]any)
	}
	for k, v := range m {
		e.debug[k] = v
	}
	e.resolveTemplateIfReady()
	return e
}

// DebugKV is an alias for Debug.
func (e *Error) DebugKV(kvs ...any) *Error {
	return e.Debug(kvs...)
}

// Data attaches typed data. The struct should use `expose:"true"` tags on fields
// that are safe to show to end users, and `json` tags for serialization names.
func (e *Error) Data(data any) *Error {
	e.data = data
	e.resolveTemplateIfReady()
	return e
}

// WithStack captures the full stack trace at this point.
func (e *Error) WithStack() *Error {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(2, pcs[:])
	e.stack = pcs[:n]
	return e
}

// HTTPStatus overrides the default HTTP status for this error.
func (e *Error) HTTPStatus(code int) *Error {
	e.httpStatus = code
	return e
}

// ─── Template resolution ────────────────────────────────────────────────────

func (e *Error) resolveTemplateIfReady() {
	if e.msgTpl == "" {
		return
	}
	e.message = resolveTemplate(e.msgTpl, e.allTemplateValues())
}

func (e *Error) allTemplateValues() map[string]any {
	values := make(map[string]any)
	if e.data != nil {
		for k, v := range ToMap(e.data) {
			values[k] = v
		}
	}
	if e.safe != nil {
		for k, v := range e.safe {
			values[k] = v
		}
	}
	if e.debug != nil {
		for k, v := range e.debug {
			values[k] = v
		}
	}
	return values
}

// resolveTemplate replaces {{.key}} placeholders in the template string.
func resolveTemplate(tpl string, values map[string]any) string {
	if len(values) == 0 {
		return handleMissingKeys(tpl)
	}
	result := tpl
	for k, v := range values {
		result = strings.ReplaceAll(result, "{{."+k+"}}", fmt.Sprint(v))
	}
	return handleMissingKeys(result)
}

func handleMissingKeys(s string) string {
	mode := getTemplateMissingKeyMode()
	switch mode {
	case MissingKeyRemove:
		// Remove unresolved {{.xxx}} placeholders.
		for {
			start := strings.Index(s, "{{.")
			if start == -1 {
				break
			}
			end := strings.Index(s[start:], "}}")
			if end == -1 {
				break
			}
			s = s[:start] + s[start+end+2:]
		}
	case MissingKeyError:
		// In error mode, we could panic, but for safety we just keep as-is.
		// The registration step validates templates.
	default:
		// MissingKeyKeep — leave as is (default).
	}
	return s
}

// ─── error interface ────────────────────────────────────────────────────────

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("[")
	b.WriteString(string(e.class))
	b.WriteString("]")
	if e.code != "" {
		b.WriteString(" ")
		b.WriteString(e.code)
	}
	b.WriteString(": ")
	b.WriteString(e.message)
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

// Unwrap returns the wrapped error for errors.Is / errors.As support.
func (e *Error) Unwrap() error { return e.cause }

// Is reports whether target matches this error (by code or class).
// This enables errors.Is(err, bang.NotFound()) style checks.
func (e *Error) Is(target error) bool {
	if matcher, ok := target.(classMatcher); ok {
		return e.class == matcher.bangClass()
	}

	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	if t.code != "" && e.code != "" && e.code == t.code {
		return true
	}
	return e.class == t.class
}

// ─── Accessors ──────────────────────────────────────────────────────────────

func (e *Error) GetClass() Class  { return e.class }
func (e *Error) GetCode() string  { return e.code }
func (e *Error) GetTitle() string { return e.title }
func (e *Error) GetMessage() string {
	if e.message == "" {
		return getClassMeta(e.class).Message
	}
	return e.message
}
func (e *Error) GetData() any             { return e.data }
func (e *Error) GetSafe() map[string]any  { return e.safe }
func (e *Error) GetDebug() map[string]any { return e.debug }
func (e *Error) GetCause() error          { return e.cause }
func (e *Error) GetStack() []uintptr      { return e.stack }
func (e *Error) GetFile() string          { return e.file }
func (e *Error) GetLine() int             { return e.line }
func (e *Error) GetHTTPStatus() int       { return e.httpStatus }
func (e *Error) GetValidationFields() []ValidationField {
	return e.validationFields
}

// SafeData returns all user-visible data: expose:"true" fields from typed Data
// merged with Safe map.
func (e *Error) SafeData() map[string]any {
	result := make(map[string]any)
	if e.data != nil {
		for k, v := range FilterSafe(e.data) {
			result[k] = v
		}
	}
	for k, v := range e.safe {
		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// AllData returns all data merged: typed Data (all fields) + Safe + Debug.
func (e *Error) AllData() map[string]any {
	result := make(map[string]any)
	if e.data != nil {
		for k, v := range ToMap(e.data) {
			result[k] = v
		}
	}
	for k, v := range e.safe {
		result[k] = v
	}
	for k, v := range e.debug {
		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ─── Inspection helpers ─────────────────────────────────────────────────────

// HasClass reports whether err has the given class.
func HasClass(err error, class Class) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.class == class
	}
	return false
}

// HasCode reports whether err has the given code.
func HasCode(err error, code string) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.code == code
	}
	return false
}

// GetData extracts typed data from an error.
func GetData[T any](err error) (T, bool) {
	var zero T
	var e *Error
	if !errors.As(err, &e) {
		return zero, false
	}
	if d, ok := e.data.(T); ok {
		return d, true
	}
	return zero, false
}

// AsBang extracts the *Error from any error chain.
func AsBang(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// ─── Stack frames ───────────────────────────────────────────────────────────

// Frame represents a single stack frame.
type Frame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// StackFrames returns human-readable stack frames.
func (e *Error) StackFrames() []Frame {
	if len(e.stack) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(e.stack)
	var result []Frame
	for {
		frame, more := frames.Next()
		result = append(result, Frame{
			Function: frame.Function,
			File:     frame.File,
			Line:     frame.Line,
		})
		if !more {
			break
		}
	}
	return result
}
