package banggin

import (
	"github.com/gin-gonic/gin"
	"github.com/smdmitry/bang"
)

// HTTPResponseOptions is an alias for convenience.
type HTTPResponseOptions = bang.HTTPResponseOptions

// HTTPResponseOptionsProvider allows per-request/per-error options.
type HTTPResponseOptionsProvider func(c *gin.Context, err error) HTTPResponseOptions

type errorMiddlewareConfig struct {
	base             HTTPResponseOptions
	provider         HTTPResponseOptionsProvider
	instanceResolver func(c *gin.Context) string
}

// ErrorMiddlewareOption configures ErrorMiddleware.
type ErrorMiddlewareOption func(*errorMiddlewareConfig)

func defaultErrorMiddlewareConfig() errorMiddlewareConfig {
	return errorMiddlewareConfig{
		instanceResolver: func(c *gin.Context) string { return c.FullPath() },
	}
}

// WithBaseOptions sets base (default) options.
func WithBaseOptions(o HTTPResponseOptions) ErrorMiddlewareOption {
	return func(cfg *errorMiddlewareConfig) { cfg.base = o }
}

// WithStaticOptions sets a static options value.
func WithStaticOptions(o HTTPResponseOptions) ErrorMiddlewareOption {
	return func(cfg *errorMiddlewareConfig) {
		cfg.provider = func(_ *gin.Context, _ error) HTTPResponseOptions { return o }
	}
}

// WithOptionsProvider sets provider that returns options per request/error.
func WithOptionsProvider(p HTTPResponseOptionsProvider) ErrorMiddlewareOption {
	return func(cfg *errorMiddlewareConfig) { cfg.provider = p }
}

// WithInstanceResolver sets a resolver to populate Instance automatically.
func WithInstanceResolver(r func(c *gin.Context) string) ErrorMiddlewareOption {
	return func(cfg *errorMiddlewareConfig) { cfg.instanceResolver = r }
}

// WithInstanceFromFullPath sets Instance to gin route pattern.
func WithInstanceFromFullPath() ErrorMiddlewareOption {
	return WithInstanceResolver(func(c *gin.Context) string { return c.FullPath() })
}

// WithInstanceFromURLPath sets Instance to raw URL path.
func WithInstanceFromURLPath() ErrorMiddlewareOption {
	return WithInstanceResolver(func(c *gin.Context) string { return c.Request.URL.Path })
}

func buildHTTPResponseOptions(cfg errorMiddlewareConfig, c *gin.Context, err error) HTTPResponseOptions {
	opts := cfg.base
	if cfg.provider != nil {
		over := cfg.provider(c, err)
		opts = mergeHTTPResponseOptions(opts, over)
	}
	if opts.Instance == "" && cfg.instanceResolver != nil {
		opts.Instance = cfg.instanceResolver(c)
	}
	return opts
}

func mergeHTTPResponseOptions(base, over HTTPResponseOptions) HTTPResponseOptions {
	out := base
	if over.Instance != "" {
		out.Instance = over.Instance
	}
	out.ShowDebug = over.ShowDebug
	out.ShowSource = over.ShowSource
	if over.SourceContextLines != 0 {
		out.SourceContextLines = over.SourceContextLines
	}
	return out
}

// ErrorMiddleware writes an error response if handlers added c.Error(err)
// and nothing has been written yet.
func ErrorMiddleware(opts ...ErrorMiddlewareOption) gin.HandlerFunc {
	cfg := defaultErrorMiddlewareConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		last := c.Errors.Last()
		if last == nil || last.Err == nil {
			return
		}

		o := buildHTTPResponseOptions(cfg, c, last.Err)
		writeProblem(c, last.Err, o)
	}
}
