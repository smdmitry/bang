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

const CauseMaxDepth = 5

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

	pd := ProblemDetail{
		Type:      getURN(e),
		Status:    e.GetHTTPStatus(),
		Title:     e.GetTitle(),
		Detail:    e.GetMessage(),
		Instance:  opts.Instance,
		Details:   e.SafeData(),
		Errors:    getValidationErrors(e),
		Cause:     buildBangCauseChain(e, CauseMaxDepth, opts),
		DebugInfo: getDebugInfo(e, opts),
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
				Type:      getURN(ce),
				Title:     ce.SelfTitle(),
				Detail:    ce.SelfMessage(),
				Details:   ce.SelfSafeData(),
				Errors:    getValidationErrors(ce),
				DebugInfo: getDebugInfo(ce, opts),
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

func getDebugInfo(e *Error, opts HTTPResponseOptions) *DebugInfo {
	if !opts.ShowDebug {
		return nil
	}

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
	return debug
}

func getValidationErrors(e *Error) (errors []ValidationError) {
	if validationFields := e.GetValidationFields(); len(validationFields) > 0 {
		errors = make([]ValidationError, len(validationFields))
		for i, vf := range validationFields {
			errors[i] = ValidationError{Key: vf.Key, Reason: vf.Reason}
		}
	}
	return errors
}
