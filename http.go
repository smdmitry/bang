package bang

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ProblemDetail represents an RFC 7807 Problem Details response.
type ProblemDetail struct {
	Type     string `json:"type"`
	Status   int    `json:"status"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`

	ErrorType string            `json:"errorType"`
	Details   map[string]any    `json:"details,omitempty"`
	Errors    []ValidationError `json:"errors,omitempty"`

	DebugInfo *DebugInfo `json:"debug,omitempty"`
}

// ValidationError is the user-facing validation field error (rule omitted).
type ValidationError struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// DebugInfo contains diagnostic information (development/admin only).
type DebugInfo struct {
	AllData map[string]any `json:"data,omitempty"`
	Stack   []Frame        `json:"stack,omitempty"`
	File    string         `json:"file,omitempty"`
	Line    int            `json:"line,omitempty"`
	Source  string         `json:"source,omitempty"`
	Cause   string         `json:"cause,omitempty"`
}

// HTTPResponseOptions configures ProblemDetail generation.
type HTTPResponseOptions struct {
	Instance           string
	ShowDebug          bool
	ShowSource         bool
	SourceContextLines int
}

// ToHTTP converts an error to a ProblemDetail.
func ToHTTP(err error, opts HTTPResponseOptions) ProblemDetail {
	var e *Error
	if !errors.As(err, &e) {
		e = WrapUnknown(err)
	}

	pd := ProblemDetail{
		Type:   fmt.Sprintf("urn:error:%s:%s", e.class, e.code),
		Status: e.class.HTTPStatus(),
		Title:  e.title,
		Detail: e.message,
	}
	if opts.Instance != "" {
		pd.Instance = opts.Instance
	}

	if e.class == ClassValidation {
		pd.ErrorType = "validation"
	} else {
		pd.ErrorType = "business"
	}

	if e.data != nil {
		pd.Details = FilterSafe(e.data)
	}

	if len(e.validationFields) > 0 {
		pd.Errors = make([]ValidationError, len(e.validationFields))
		for i, vf := range e.validationFields {
			pd.Errors[i] = ValidationError{Key: vf.Key, Reason: vf.Reason}
		}
	}

	if opts.ShowDebug {
		debug := &DebugInfo{File: e.file, Line: e.line}

		allData := make(map[string]any)
		if e.data != nil {
			for k, v := range ToMap(e.data) {
				allData[k] = v
			}
		}
		if e.debug != nil {
			for k, v := range ToMap(e.debug) {
				allData[k] = v
			}
		}
		if len(allData) > 0 {
			debug.AllData = allData
		}
		if frames := e.StackFrames(); len(frames) > 0 {
			debug.Stack = frames
		}
		if e.cause != nil {
			debug.Cause = e.cause.Error()
		}
		if opts.ShowSource && e.file != "" && e.line > 0 {
			ctx := opts.SourceContextLines
			if ctx <= 0 {
				ctx = 3
			}
			debug.Source = readSourceContext(e.file, e.line, ctx)
		}
		pd.DebugInfo = debug
	}

	return pd
}

// WriteHTTP writes the ProblemDetail as a JSON HTTP response.
func WriteHTTP(w http.ResponseWriter, err error, opts HTTPResponseOptions) {
	pd := ToHTTP(err, opts)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(pd.Status)
	_ = json.NewEncoder(w).Encode(pd)
}

func readSourceContext(file string, line, contextLines int) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	start := line - contextLines - 1
	if start < 0 {
		start = 0
	}
	end := line + contextLines
	if end > len(lines) {
		end = len(lines)
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		marker := "  "
		if i == line-1 {
			marker = "» "
		}
		fmt.Fprintf(&b, "%s%4d | %s\n", marker, i+1, lines[i])
	}
	return b.String()
}
