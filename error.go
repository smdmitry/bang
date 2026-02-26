package bang

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var projectRoot string

func init() {
	dir, _ := os.Getwd()

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			projectRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

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

const causeFallbackMaxDepth = 5

// newError creates a new Error, auto-capturing file and line of the caller.
func newError(class Class, callerSkip int) *Error {
	e := &Error{
		class: class,
	}
	if _, file, line, ok := runtime.Caller(callerSkip + 1); ok {
		if projectRoot != "" {
			if rel, err := filepath.Rel(projectRoot, file); err == nil {
				file = rel
			}
		}

		e.file = file
		e.line = line
	}
	return e
}

// ─── Builder methods ────────────────────────────────────────────────────────

// Wrap wraps another error as the cause.
func (e *Error) Wrap(err error) *Error {
	e.cause = err
	return e
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
	if data := e.GetData(); data != nil {
		for k, v := range ToMap(data) {
			values[k] = v
		}
	}
	if safe := e.GetSafe(); safe != nil {
		for k, v := range safe {
			values[k] = v
		}
	}
	if debug := e.GetDebug(); debug != nil {
		for k, v := range debug {
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
	class := e.GetClass()
	code := e.GetCode()
	message := e.GetMessage()

	b.WriteString("[")
	b.WriteString(string(class))
	b.WriteString("]")
	if code != "" {
		b.WriteString(" ")
		b.WriteString(code)
	}
	b.WriteString(": ")
	b.WriteString(message)
	if cause := e.GetCause(); cause != nil {
		b.WriteString(": ")
		b.WriteString(cause.Error())
	}
	return b.String()
}

// Unwrap returns the wrapped error for errors.Is / errors.As support.
func (e *Error) Unwrap() error { return e.cause }

// Is reports whether target matches this error (by code or class).
// This enables errors.Is(err, bang.NotFound()) style checks.
func (e *Error) Is(target error) bool {
	if matcher, ok := target.(classMatcher); ok {
		return e.GetClass() == matcher.bangClass()
	}

	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	if targetCode, currentCode := t.GetCode(), e.GetCode(); targetCode != "" && currentCode != "" && currentCode == targetCode {
		return true
	}
	return e.GetClass() == t.GetClass()
}

// ─── Accessors ──────────────────────────────────────────────────────────────

func (e *Error) walkBangCauses(maxDepth int, fn func(*Error) bool) {
	if e == nil || maxDepth <= 0 || fn == nil {
		return
	}
	cur := e.cause
	for depth := 0; cur != nil && depth < maxDepth; depth++ {
		ce, ok := cur.(*Error)
		if ok && ce != nil && fn(ce) {
			return
		}
		cur = errors.Unwrap(cur)
	}
}

func (e *Error) GetClass() Class {
	if e == nil {
		return ClassUnexpected
	}
	if e.class != "" {
		return e.class
	}
	var class Class
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.class == "" {
			return false
		}
		class = ce.class
		return true
	})
	if class != "" {
		return class
	}
	return ClassUnexpected
}
func (e *Error) SelfClass() Class {
	return e.class
}

func (e *Error) GetCodePrefix() string {
	if e == nil {
		return ""
	}
	if e.codePrefix != "" {
		return e.codePrefix
	}
	var codePrefix string
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.codePrefix == "" {
			return false
		}
		codePrefix = ce.codePrefix
		return true
	})
	return codePrefix
}
func (e *Error) SelfCodePrefix() string {
	return e.codePrefix
}

func (e *Error) GetCode() string {
	if e == nil {
		return ""
	}
	if e.code != "" {
		return e.code
	}
	var code string
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.code == "" {
			return false
		}
		code = ce.code
		return true
	})
	return code
}

func (e *Error) SelfCode() string {
	return e.code
}

func (e *Error) GetTitle() string {
	if e == nil {
		return ""
	}
	if e.title != "" {
		return e.title
	}
	var title string
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.title == "" {
			return false
		}
		title = ce.title
		return true
	})
	if title != "" {
		return title
	}
	if class := e.GetClass(); class != "" {
		return getClassMeta(class).Title
	}
	return ""
}

func (e *Error) SelfTitle() string {
	return e.title
}

func (e *Error) GetMsgTpl() string {
	if e == nil {
		return ""
	}
	if e.msgTpl != "" {
		return e.msgTpl
	}
	var msgTpl string
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.msgTpl == "" {
			return false
		}
		msgTpl = ce.msgTpl
		return true
	})
	return msgTpl
}

func (e *Error) SelfMsgTpl() string {
	return e.msgTpl
}

func (e *Error) GetMessage() string {
	if e == nil {
		return ""
	}
	if e.message != "" {
		return e.message
	}
	var message string
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.message == "" {
			return false
		}
		message = ce.message
		return true
	})
	if message != "" {
		return message
	}
	if class := e.GetClass(); class != "" {
		return getClassMeta(class).Message
	}
	return ""
}

func (e *Error) SelfMessage() string {
	return e.message
}

func (e *Error) GetData() any {
	if e == nil {
		return nil
	}
	if e.data != nil {
		return e.data
	}
	var data any
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.data == nil {
			return false
		}
		data = ce.data
		return true
	})
	return data
}

func (e *Error) SelfData() any {
	return e.data
}

func (e *Error) GetSafe() map[string]any {
	if e == nil {
		return nil
	}
	if len(e.safe) > 0 {
		return e.safe
	}
	var safe map[string]any
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if len(ce.safe) == 0 {
			return false
		}
		safe = ce.safe
		return true
	})
	return safe
}

func (e *Error) SelfSafe() map[string]any {
	return e.safe
}

func (e *Error) GetDebug() map[string]any {
	if e == nil {
		return nil
	}
	if len(e.debug) > 0 {
		return e.debug
	}
	var debug map[string]any
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if len(ce.debug) == 0 {
			return false
		}
		debug = ce.debug
		return true
	})
	return debug
}

func (e *Error) SelfDebug() map[string]any {
	return e.debug
}

func (e *Error) GetCause() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *Error) GetValidationFields() []ValidationField {
	if e == nil {
		return nil
	}
	if len(e.validationFields) > 0 {
		return e.validationFields
	}
	var fields []ValidationField
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if len(ce.validationFields) == 0 {
			return false
		}
		fields = ce.validationFields
		return true
	})
	return fields
}

func (e *Error) SelfValidationFields() []ValidationField {
	return e.validationFields
}

func (e *Error) GetStack() []uintptr {
	if e == nil {
		return nil
	}
	if len(e.stack) > 0 {
		return e.stack
	}
	var stack []uintptr
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if len(ce.stack) == 0 {
			return false
		}
		stack = ce.stack
		return true
	})
	return stack
}

func (e *Error) SelfStack() []uintptr {
	return e.stack
}

func (e *Error) GetFile() string {
	if e == nil {
		return ""
	}
	if e.file != "" {
		return e.file
	}
	var file string
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.file == "" {
			return false
		}
		file = ce.file
		return true
	})
	return file
}

func (e *Error) SelfFile() string {
	return e.file
}

func (e *Error) GetLine() int {
	if e == nil {
		return 0
	}
	if e.line > 0 {
		return e.line
	}
	var line int
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.line <= 0 {
			return false
		}
		line = ce.line
		return true
	})
	return line
}

func (e *Error) SelfLine() int {
	return e.line
}

func (e *Error) GetHTTPStatus() int {
	if e == nil {
		return http.StatusInternalServerError
	}
	if e.httpStatus > 0 {
		return e.httpStatus
	}
	var status int
	e.walkBangCauses(causeFallbackMaxDepth, func(ce *Error) bool {
		if ce.httpStatus <= 0 {
			return false
		}
		status = ce.httpStatus
		return true
	})
	if status > 0 {
		return status
	}
	class := e.GetClass()
	if class != "" {
		return class.HTTPStatus()
	}
	return http.StatusInternalServerError
}

func (e *Error) SelfHTTPStatus() int {
	return e.httpStatus
}

// SelfSafeData returns user-visible data from the current error only:
// expose:"true" fields from local typed Data merged with local Safe map.
func (e *Error) SelfSafeData() map[string]any {
	result := make(map[string]any)
	for k, v := range e.SelfSafe() {
		result[k] = v
	}

	if data := e.SelfData(); data != nil {
		for k, v := range FilterSafe(data) {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// SafeData returns all user-visible data: expose:"true" fields from typed Data
// merged with Safe map.
func (e *Error) SafeData() map[string]any {
	result := make(map[string]any)
	for k, v := range e.GetSafe() {
		result[k] = v
	}

	if data := e.GetData(); data != nil {
		for k, v := range FilterSafe(data) {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// AllData returns all data merged: typed Data (all fields) + Safe + Debug.
func (e *Error) AllData() map[string]any {
	result := make(map[string]any)
	for k, v := range e.GetDebug() {
		result[k] = v
	}
	for k, v := range e.GetSafe() {
		result[k] = v
	}

	if data := e.GetData(); data != nil {
		for k, v := range ToMap(data) {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// UnsafeData returns only unsafe data: non-exposed typed Data fields + Debug.
// Safe map and expose:"true" typed fields are excluded.
func (e *Error) UnsafeData() map[string]any {
	result := make(map[string]any)

	for k, v := range e.GetDebug() {
		result[k] = v
	}

	if data := e.GetData(); data != nil {
		all := ToMap(data)
		safe := FilterSafe(data)
		for k, v := range all {
			if _, ok := safe[k]; ok {
				continue
			}
			result[k] = v
		}
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
		return e.GetClass() == class
	}
	return false
}

// HasCode reports whether err has the given code.
func HasCode(err error, code string) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.GetCode() == code
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
	if d, ok := e.GetData().(T); ok {
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
	stack := e.GetStack()
	if len(stack) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(stack)
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
