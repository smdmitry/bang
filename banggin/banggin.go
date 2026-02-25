package banggin

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/smdmitry/bang"
)

// ShouldBindJSON binds JSON request body into T. On error, aborts with a bang validation error.
func ShouldBindJSON[T any](c *gin.Context) *T {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(bang.WrapValidationError(err))
		c.Abort()
		return nil
	}
	return &req
}

// AbortError adds err to gin context and aborts request processing.
func AbortError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

// RecoveryMiddleware recovers panics and writes an RFC 7807 response.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			HandleRecoveryErr(c, recovered, bang.HTTPResponseOptions{})
		}()
		c.Next()
	}
}

// HandleRecoveryErr wraps a recovered panic into a bang error and writes the response.
func HandleRecoveryErr(c *gin.Context, err any, opts bang.HTTPResponseOptions) error {
	wrapped := panicErr(err)
	_ = c.Error(wrapped)
	c.Abort()

	if c.Writer.Written() {
		return wrapped
	}
	writeProblem(c, wrapped, opts)
	return wrapped
}

// AbortBind converts bind/validation errors into bang and aborts request processing.
func AbortBind(c *gin.Context, err error) {
	if err == nil {
		return
	}
	wrappedErr := bang.WrapValidationError(err)
	if wrappedErr == nil {
		return
	}
	_ = c.Error(wrappedErr)
	c.Abort()
}

// WriteProblem writes a ProblemDetail response for the given error.
func WriteProblem(c *gin.Context, err error, opts bang.HTTPResponseOptions) {
	writeProblem(c, err, opts)
}

func shouldLogError(status int) bool {
	return status >= 500
}

func writeProblem(c *gin.Context, err error, opts bang.HTTPResponseOptions) {
	pd := bang.ToHTTP(err, opts)
	status := pd.Status

	if shouldLogError(status) {
		attrs := append(bang.SlogAttrs(err),
			slog.Int("http.status", status),
			slog.String("http.method", c.Request.Method),
			slog.String("http.path", c.Request.URL.Path),
		)
		slog.Error("request failed", attrs...)
	}

	if len(opts.Instance) == 0 {
		opts.Instance = c.Request.URL.Path
	}
	bang.WriteHTTP(c.Writer, err, opts)
}

func panicErr(v any) error {
	e := bang.Unexpected().Code("panic.recovered").
		WithStack().
		Debug(
			"panic.type", fmt.Sprintf("%T", v),
			"panic.value", fmt.Sprint(v),
		)
	if panicAsErr, ok := v.(error); ok {
		return e.Wrap(panicAsErr)
	}
	return e
}
