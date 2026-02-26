package bang

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// ProblemDetail represents an RFC 7807 Problem Details response.
type ProblemDetail struct {
	Type     string `json:"type"`
	Status   int    `json:"status"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`

	Details map[string]any    `json:"details,omitempty"`
	Errors  []ValidationError `json:"errors,omitempty"`
	Cause   []ProblemCause    `json:"cause,omitempty"`

	DebugInfo *DebugInfo `json:"debug,omitempty"`
}

// ProblemCause is a public cause-chain element with the same shape as
// ProblemDetail, except it has no status/instance fields.
type ProblemCause struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Detail    string            `json:"detail"`
	Details   map[string]any    `json:"details,omitempty"`
	Errors    []ValidationError `json:"errors,omitempty"`
	Cause     []ProblemCause    `json:"cause,omitempty"`
	DebugInfo *DebugInfo        `json:"debug,omitempty"`
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

	class := e.GetClass()
	title := e.GetTitle()
	detail := e.GetMessage()

	status := class.HTTPStatus()
	if httpStatus := e.GetHTTPStatus(); httpStatus != 0 {
		status = httpStatus
	}

	pd := ProblemDetail{
		Type:   getURN(e),
		Status: status,
		Title:  title,
		Detail: detail,
	}
	if opts.Instance != "" {
		pd.Instance = opts.Instance
	}

	// Safe data: expose:"true" fields from typed Data + Safe map.
	safeData := e.SafeData()
	if len(safeData) > 0 {
		pd.Details = safeData
	}

	// Validation errors (without rule — rule is unsafe).
	if validationFields := e.GetValidationFields(); len(validationFields) > 0 {
		pd.Errors = make([]ValidationError, len(validationFields))
		for i, vf := range validationFields {
			pd.Errors[i] = ValidationError{Key: vf.Key, Reason: vf.Reason}
		}
	}

	// Public cause chain (only nested bang errors), max depth 5.
	pd.Cause = buildBangCauseChain(e, 5, opts)

	// Debug info (only for development builds or admin users).
	if opts.ShowDebug {
		file := e.GetFile()
		line := e.GetLine()
		debug := &DebugInfo{File: file + ":" + strconv.Itoa(line)}

		allData := e.UnsafeData()
		if len(allData) > 0 {
			debug.AllData = allData
		}
		if frames := e.StackFrames(); len(frames) > 0 {
			debug.Stack = frames
		}
		if cause := firstNonBangCause(e.GetCause(), 1); cause != nil {
			debug.Cause = cause.Error()
		}
		if opts.ShowSource && file != "" && line > 0 {
			ctx := opts.SourceContextLines
			if ctx <= 0 {
				ctx = 3
			}
			debug.Source = readSourceContext(file, line, ctx)
		}
		pd.DebugInfo = debug
	}

	return pd
}

func buildBangCauseChain(e *Error, maxDepth int, opts HTTPResponseOptions) []ProblemCause {
	if e == nil || maxDepth <= 0 {
		return nil
	}

	var chain []ProblemCause
	cur := e.GetCause()
	for cur != nil && len(chain) < maxDepth {
		if ce, ok := cur.(*Error); ok {
			cause := ProblemCause{
				Type:   getURN(ce),
				Title:  ce.GetTitle(),
				Detail: ce.GetMessage(),
			}
			if details := ce.SafeData(); len(details) > 0 {
				cause.Details = details
			}
			if validationFields := ce.GetValidationFields(); len(validationFields) > 0 {
				cause.Errors = make([]ValidationError, len(validationFields))
				for i, vf := range validationFields {
					cause.Errors[i] = ValidationError{Key: vf.Key, Reason: vf.Reason}
				}
			}

			if opts.ShowDebug {
				file := ce.GetFile()
				line := ce.GetLine()
				debug := &DebugInfo{File: file + ":" + strconv.Itoa(line)}

				allData := ce.UnsafeData()
				if len(allData) > 0 {
					debug.AllData = allData
				}
				if frames := ce.StackFrames(); len(frames) > 0 {
					debug.Stack = frames
				}
				if ceCause := firstNonBangCause(ce.GetCause(), 1); ceCause != nil {
					debug.Cause = ceCause.Error()
				}
				if opts.ShowSource && file != "" && line > 0 {
					ctx := opts.SourceContextLines
					if ctx <= 0 {
						ctx = 3
					}
					debug.Source = readSourceContext(file, line, ctx)
				}
				cause.DebugInfo = debug
			}

			chain = append(chain, cause)
		}
		cur = errors.Unwrap(cur)
	}
	if len(chain) == 0 {
		return nil
	}
	return chain
}

func firstNonBangCause(err error, maxDepth int) error {
	if err == nil || maxDepth <= 0 {
		return nil
	}
	cur := err
	for depth := 0; cur != nil && depth < maxDepth; depth++ {
		if _, ok := cur.(*Error); !ok {
			return cur
		}
		cur = errors.Unwrap(cur)
	}
	return nil
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

func getURN(err *Error) string {
	urnType := string(err.GetClass())
	code := err.GetCode()

	if len(code) > 0 {
		urnType += ":" + code
	}

	return "urn:" + urnType
}
