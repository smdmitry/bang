package bang

// ─── ErrFactory — callable error definition ─────────────────────────────────

// ErrFactory is a function type that creates errors. Call it directly
// and use .Is() for matching.
//
//	var ErrUserNotFound = bang.Define(bang.ClassNotFound, "users.not_found")
//
//	ErrUserNotFound()                               // new
//	ErrUserNotFound(err)                            // wrap
//	ErrUserNotFound().Msgf("user %s not found", id) // + message
//	ErrUserNotFound.Is(err)                         // check
type ErrFactory func(wrap ...error) *Error

// Define creates an ErrFactory — a callable error definition.
func Define(class Class, code string, opts ...DefOption) ErrFactory {
	cfg := defConfig{class: class, code: code}
	for _, o := range opts {
		o(&cfg)
	}
	return func(wrap ...error) *Error {
		e := newError(class, 2)
		e.code = code
		if cfg.title != "" {
			e.title = cfg.title
		}
		if cfg.message != "" {
			e.message = cfg.message
		}
		if cfg.msgTpl != "" {
			e.msgTpl = cfg.msgTpl
		}
		if len(wrap) > 0 && wrap[0] != nil {
			e.cause = wrap[0]
		}
		return e
	}
}

// Is reports whether err matches this error definition (by code).
func (f ErrFactory) Is(err error) bool {
	ref := f()
	if ref.code != "" {
		return HasCode(err, ref.code)
	}
	return HasClass(err, ref.class)
}

// ─── TypedErrFactory — same callable, plus typed extraction ─────────────────

// TypedErrFactory has the same call signature as ErrFactory.
// The generic parameter adds a typed GetData() method.
//
//	var ErrOrderFailed = bang.DefineTyped[OrderData](bang.ClassDatabaseError, "orders.save_failed")
//
//	ErrOrderFailed()                                     // new
//	ErrOrderFailed(err).Data(OrderData{OrderID: id})     // wrap + data
//	data, ok := ErrOrderFailed.GetData(err)              // typed extract
type TypedErrFactory[T any] func(wrap ...error) *Error

// DefineTyped creates a TypedErrFactory with the same call signature as Define.
func DefineTyped[T any](class Class, code string, opts ...DefOption) TypedErrFactory[T] {
	cfg := defConfig{class: class, code: code}
	for _, o := range opts {
		o(&cfg)
	}
	return func(wrap ...error) *Error {
		e := newError(class, 2)
		e.code = code
		if cfg.title != "" {
			e.title = cfg.title
		}
		if cfg.message != "" {
			e.message = cfg.message
		}
		if cfg.msgTpl != "" {
			e.msgTpl = cfg.msgTpl
		}
		if len(wrap) > 0 && wrap[0] != nil {
			e.cause = wrap[0]
		}
		return e
	}
}

// Is reports whether err matches this typed error definition (by code).
func (f TypedErrFactory[T]) Is(err error) bool {
	ref := f()
	if ref.code != "" {
		return HasCode(err, ref.code)
	}
	return HasClass(err, ref.class)
}

// GetData extracts the typed data from err.
func (f TypedErrFactory[T]) GetData(err error) (T, bool) {
	return GetData[T](err)
}

// ─── Options ────────────────────────────────────────────────────────────────

type defConfig struct {
	class   Class
	code    string
	title   string
	message string
	msgTpl  string
}

// DefOption configures a Define/DefineTyped call.
type DefOption func(*defConfig)

func WithTitle(title string) DefOption     { return func(c *defConfig) { c.title = title } }
func WithMessage(message string) DefOption { return func(c *defConfig) { c.message = message } }
func WithMsgTpl(tpl string) DefOption      { return func(c *defConfig) { c.msgTpl = tpl } }
