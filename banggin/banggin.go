package banggin

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/smdmitry/bang"
)

func ShouldBindJSON[T any](c *gin.Context) *T {
	var req T
	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Println(err)
		fmt.Println(BindErr(err))
		_ = c.Error(BindErr(err))
		c.Abort()
		return nil
	}

	return &req
}

func AbortError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

// ErrorMiddleware is a single JSON serialization point for errors collected in gin.Context.
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		last := c.Errors.Last()
		if last == nil || last.Err == nil {
			return
		}

		writeProblem(c, last.Err, bang.HTTPResponseOptions{})
	}
}

// RecoveryMiddleware recovers panics and writes an RFC7807 response.
// It works with and without ErrorMiddleware.
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
// The response is serialized by ErrorMiddleware.
func AbortBind(c *gin.Context, err error) {
	if err == nil {
		return
	}

	bindErr := BindErr(err)
	if bindErr == nil {
		return
	}

	_ = c.Error(bindErr)
	c.Abort()
}

func shouldLogError(status int) bool {
	return status >= 500 || true
}

func WriteProblem(c *gin.Context, err error, opts bang.HTTPResponseOptions) {
	writeProblem(c, err, opts)
}
func writeProblem(c *gin.Context, err error, opts bang.HTTPResponseOptions) {
	status := bang.ToHTTP(err, opts).Status
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
	e := bang.Unexpected("panic.recovered").
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
