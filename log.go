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
	class := e.GetClass()
	code := e.GetCode()
	file := e.GetFile()
	line := e.GetLine()
	rec := LogRecord{
		Type:    string(class) + ":" + code,
		Class:   string(class),
		Code:    code,
		Title:   e.GetTitle(),
		Message: e.GetMessage(),
		File:    file,
		Line:    line,
	}

	// Merge all data sources for the log.
	allSafe := make(map[string]any)
	if data := e.GetData(); data != nil {
		for k, v := range ToMap(data) {
			allSafe[k] = v
		}
	}
	for k, v := range e.GetSafe() {
		allSafe[k] = v
	}
	if len(allSafe) > 0 {
		rec.Data = allSafe
	}
	if debug := e.GetDebug(); len(debug) > 0 {
		rec.Debug = debug
	}
	if cause := e.GetCause(); cause != nil {
		rec.Cause = cause.Error()
	}
	if frames := e.StackFrames(); len(frames) > 0 {
		rec.Stack = frames
	}
	if validationFields := e.GetValidationFields(); len(validationFields) > 0 {
		rec.ValidationErrors = validationFields
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
	class := e.GetClass()
	code := e.GetCode()
	file := e.GetFile()
	line := e.GetLine()
	attrs := []any{
		slog.String("error.type", string(class)+":"+code),
		slog.String("error.class", string(class)),
		slog.String("error.code", code),
		slog.String("error.message", e.GetMessage()),
	}
	if file != "" {
		attrs = append(attrs, slog.String("error.file", file), slog.Int("error.line", line))
	}
	if data := e.GetData(); data != nil {
		attrs = append(attrs, slog.Any("error.data", ToMap(data)))
	}
	if safe := e.GetSafe(); len(safe) > 0 {
		attrs = append(attrs, slog.Any("error.safe", safe))
	}
	if debug := e.GetDebug(); len(debug) > 0 {
		attrs = append(attrs, slog.Any("error.debug", debug))
	}
	if cause := e.GetCause(); cause != nil {
		attrs = append(attrs, slog.String("error.cause", cause.Error()))
	}
	if validationFields := e.GetValidationFields(); len(validationFields) > 0 {
		attrs = append(attrs, slog.Any("error.validationErrors", validationFields))
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
