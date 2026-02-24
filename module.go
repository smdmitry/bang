package bang

// Module provides a code prefix for a package or subsystem.
// All codes produced through Module are automatically prefixed.
//
//	var Mod = bang.NewModule("orders.repo")
//
//	Mod.NotFound(err, "get_by_id")                       // code → "orders.repo.get_by_id"
//	var ErrNotFound = Mod.Define(bang.ClassNotFound, "not_found") // code → "orders.repo.not_found"
type Module struct {
	prefix string
}

// NewModule creates a Module with the given code prefix.
func NewModule(prefix string) Module {
	return Module{prefix: prefix}
}

func (m Module) fullCode(code string) string {
	if code == "" {
		return m.prefix
	}
	return m.prefix + "." + code
}

func (m Module) applyModuleArgs(e *Error, args []any) {
	for _, arg := range args {
		switch v := arg.(type) {
		case error:
			e.cause = v
		case string:
			e.code = m.fullCode(v)
		}
	}
}

func (m Module) newFromClass(class Class, args []any) *Error {
	e := newError(class, 3) // skip: method → newFromClass → newError
	m.applyModuleArgs(e, args)
	// If no string arg passed, use prefix as code.
	if e.code == getClassMeta(class).DefaultCode {
		e.code = m.prefix
	}
	return e
}

// ─── Shorthand constructors (mirror top-level ones) ─────────────────────────

func (m Module) InternalServer(args ...any) *Error { return m.newFromClass(ClassInternalServer, args) }
func (m Module) NotFound(args ...any) *Error       { return m.newFromClass(ClassNotFound, args) }
func (m Module) Unauthorized(args ...any) *Error   { return m.newFromClass(ClassUnauthorized, args) }
func (m Module) Forbidden(args ...any) *Error      { return m.newFromClass(ClassForbidden, args) }
func (m Module) Conflict(args ...any) *Error       { return m.newFromClass(ClassConflict, args) }
func (m Module) TooManyRequests(args ...any) *Error {
	return m.newFromClass(ClassTooManyRequests, args)
}

func (m Module) DatabaseError(args ...any) *Error { return m.newFromClass(ClassDatabaseError, args) }
func (m Module) DuplicateKey(args ...any) *Error  { return m.newFromClass(ClassDuplicateKey, args) }
func (m Module) ForeignKeyViolation(args ...any) *Error {
	return m.newFromClass(ClassForeignKeyViolation, args)
}

func (m Module) ExternalServiceError(args ...any) *Error {
	return m.newFromClass(ClassExternalServiceError, args)
}
func (m Module) NetworkError(args ...any) *Error { return m.newFromClass(ClassNetworkError, args) }
func (m Module) TimeoutError(args ...any) *Error { return m.newFromClass(ClassTimeoutError, args) }

func (m Module) SerializationError(args ...any) *Error {
	return m.newFromClass(ClassSerializationError, args)
}
func (m Module) Unexpected(args ...any) *Error { return m.newFromClass(ClassUnexpected, args) }

func (m Module) New(class Class, args ...any) *Error { return m.newFromClass(class, args) }

// ─── Define / DefineTyped through Module ────────────────────────────────────

// Define creates an ErrFactory with the code prefixed by this module.
func (m Module) Define(class Class, code string, opts ...DefOption) ErrFactory {
	return Define(class, m.fullCode(code), opts...)
}

// ModuleDefineTyped creates a TypedErrFactory with the code prefixed by a module.
// (Go methods cannot have type parameters, hence this is a top-level function.)
//
//	var Mod = bang.NewModule("orders.repo")
//	var ErrOrderFailed = bang.ModuleDefineTyped[OrderData](Mod, bang.ClassDatabaseError, "save_failed")
func ModuleDefineTyped[T any](m Module, class Class, code string, opts ...DefOption) TypedErrFactory[T] {
	return DefineTyped[T](class, m.fullCode(code), opts...)
}

// ─── WrapDBError / WrapRedisError through Module ────────────────────────────

func (m Module) WrapDBError(err error, code ...string) *Error {
	e := WrapDBError(err)
	if e != nil && len(code) > 0 {
		e.code = m.fullCode(code[0])
	} else if e != nil {
		e.code = m.prefix
	}
	return e
}

func (m Module) WrapRedisError(err error, code ...string) *Error {
	e := WrapRedisError(err)
	if e != nil && len(code) > 0 {
		e.code = m.fullCode(code[0])
	} else if e != nil {
		e.code = m.prefix
	}
	return e
}
