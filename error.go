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
	class   Class
	code    string
	title   string
	message string
	msgTpl  string // message template with {key} placeholders

	data  any // typed data (struct with expose tags)
	debug any // unsafe debug data

	cause error // wrapped error

	validationFields []ValidationField

	stack []uintptr
	file  string // auto-captured source file
	line  int    // auto-captured source line
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

// Code sets a unique error code. Recommended format: module.submodule.error_descr
func (e *Error) Code(code string) *Error {
	e.code = code
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
	return e
}

// Msgf sets a user-facing message using fmt.Sprintf formatting.
func (e *Error) Msgf(format string, args ...any) *Error {
	e.message = fmt.Sprintf(format, args...)
	return e
}

// MsgTpl sets a message template with {key} placeholders resolved from Data and Debug.
// Values are resolved immediately from already-set data/debug, and re-resolved
// when Data/Debug/DebugKV is called later.
//
//	bang.NotFound("products.get").
//	    DebugKV("sku", "X1", "warehouse", "msk-1").
//	    MsgTpl("товар {sku} не найден на складе {warehouse}")
func (e *Error) MsgTpl(tpl string) *Error {
	e.msgTpl = tpl
	e.message = e.resolveTemplate(tpl)
	return e
}

// Data attaches typed data. The struct should use `expose:"true"` tags on fields
// that are safe to show to end users.
func (e *Error) Data(data any) *Error {
	e.data = data
	if e.msgTpl != "" {
		e.message = e.resolveTemplate(e.msgTpl)
	}
	return e
}

// Debug attaches debug-only data (unsafe, never exposed to end users in production).
//
// Supported forms:
//
//	Debug(map[string]any{"id": id})
//	Debug("id", id, "query", query)
//	Debug(map[string]any{"id": id}, "extra", value)
func (e *Error) Debug(args ...any) *Error {
	if len(args) == 0 {
		return e
	}

	// Fast path: exactly one map
	if len(args) == 1 {
		if mm, ok := args[0].(map[string]any); ok {
			e.debug = mm
			if e.msgTpl != "" {
				e.message = e.resolveTemplate(e.msgTpl)
			}
			return e
		}
	}

	m := make(map[string]any)

	// KV list + optional embedded maps
	for i := 0; i < len(args); {
		switch v := args[i].(type) {
		case map[string]any:
			for k, val := range v {
				m[k] = val
			}
			i++

		case string:
			if i+1 < len(args) {
				m[v] = args[i+1]
				i += 2
			} else {
				i++ // dangling key, ignore
			}

		default:
			i++ // ignore unknown item
		}
	}

	e.debug = m

	if e.msgTpl != "" {
		e.message = e.resolveTemplate(e.msgTpl)
	}
	return e
}

// Optional alias for backward compatibility.
//func (e *Error) DebugKV(kvs ...any) *Error { return e.Debug(kvs...) }

// Wrap wraps another error as the cause.
func (e *Error) Wrap(err error) *Error {
	e.cause = err
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

// ─── Template resolution ────────────────────────────────────────────────────

func (e *Error) resolveTemplate(tpl string) string {
	values := make(map[string]any)
	if e.data != nil {
		for k, v := range ToMap(e.data) {
			values[k] = v
		}
	}
	if e.debug != nil {
		for k, v := range ToMap(e.debug) {
			values[k] = v
		}
	}
	return replacePlaceholders(tpl, values)
}

func replacePlaceholders(tpl string, values map[string]any) string {
	result := tpl
	for k, v := range values {
		result = strings.ReplaceAll(result, "{"+k+"}", fmt.Sprint(v))
	}
	return result
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

// ─── Accessors ──────────────────────────────────────────────────────────────

func (e *Error) GetClass() Class                        { return e.class }
func (e *Error) GetCode() string                        { return e.code }
func (e *Error) GetTitle() string                       { return e.title }
func (e *Error) GetMessage() string                     { return e.message }
func (e *Error) GetData() any                           { return e.data }
func (e *Error) GetDebug() any                          { return e.debug }
func (e *Error) GetCause() error                        { return e.cause }
func (e *Error) GetStack() []uintptr                    { return e.stack }
func (e *Error) GetFile() string                        { return e.file }
func (e *Error) GetLine() int                           { return e.line }
func (e *Error) GetValidationFields() []ValidationField { return e.validationFields }

// ─── Inspection helpers ─────────────────────────────────────────────────────

// Is reports whether any error in the chain matches by code or class.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	if t.code != "" && e.code == t.code {
		return true
	}
	return e.class == t.class
}

// HasClass reports whether this error has the given class.
func HasClass(err error, class Class) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.class == class
	}
	return false
}

// HasCode reports whether this error has the given code.
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
