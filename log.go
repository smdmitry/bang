package bang

import (
	"encoding/json"
	"errors"
	"log/slog"
)

// LogRecord represents the full error information for logging (no filtering).
type LogRecord struct {
	Type    string `json:"type"`
	Class   string `json:"class"`
	Code    string `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`

	Data  map[string]any `json:"data,omitempty"`  // safe data
	Debug map[string]any `json:"debug,omitempty"` // unsafe data

	File  string  `json:"file,omitempty"`
	Line  int     `json:"line,omitempty"`
	Cause string  `json:"cause,omitempty"`
	Stack []Frame `json:"stack,omitempty"`

	ValidationErrors []ValidationField `json:"validationErrors,omitempty"`
}

// ToLogRecord converts an error into a structured log record.
// All fields are included (no safe/unsafe filtering).
func ToLogRecord(err error) LogRecord {
	var e *Error
	if !errors.As(err, &e) {
		e = WrapUnknown(err)
	}
	rec := LogRecord{
		Type:    string(e.class) + ":" + e.code,
		Class:   string(e.class),
		Code:    e.code,
		Title:   e.title,
		Message: e.message,
		File:    e.file,
		Line:    e.line,
	}

	// Merge all data sources for the log.
	allSafe := make(map[string]any)
	if e.data != nil {
		for k, v := range ToMap(e.data) {
			allSafe[k] = v
		}
	}
	for k, v := range e.safe {
		allSafe[k] = v
	}
	if len(allSafe) > 0 {
		rec.Data = allSafe
	}
	if len(e.debug) > 0 {
		rec.Debug = e.debug
	}
	if e.cause != nil {
		rec.Cause = e.cause.Error()
	}
	if frames := e.StackFrames(); len(frames) > 0 {
		rec.Stack = frames
	}
	if len(e.validationFields) > 0 {
		rec.ValidationErrors = e.validationFields
	}
	return rec
}

// ToJSON converts an error to a JSON string for logging.
func ToJSON(err error) string {
	b, _ := json.Marshal(ToLogRecord(err))
	return string(b)
}

// SlogAttrs returns slog attributes for structured logging.
//
//	slog.Error("operation failed", bang.SlogAttrs(err)...)
func SlogAttrs(err error) []any {
	var e *Error
	if !errors.As(err, &e) {
		return []any{slog.String("error", err.Error())}
	}
	attrs := []any{
		slog.String("error.type", string(e.class)+":"+e.code),
		slog.String("error.class", string(e.class)),
		slog.String("error.code", e.code),
		slog.String("error.message", e.message),
	}
	if e.file != "" {
		attrs = append(attrs, slog.String("error.file", e.file), slog.Int("error.line", e.line))
	}
	if e.data != nil {
		attrs = append(attrs, slog.Any("error.data", ToMap(e.data)))
	}
	if len(e.safe) > 0 {
		attrs = append(attrs, slog.Any("error.safe", e.safe))
	}
	if len(e.debug) > 0 {
		attrs = append(attrs, slog.Any("error.debug", e.debug))
	}
	if e.cause != nil {
		attrs = append(attrs, slog.String("error.cause", e.cause.Error()))
	}
	if len(e.validationFields) > 0 {
		attrs = append(attrs, slog.Any("error.validationErrors", e.validationFields))
	}
	return attrs
}

// SlogError logs an error at slog.Error level.
func SlogError(logger *slog.Logger, msg string, err error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(msg, SlogAttrs(err)...)
}
