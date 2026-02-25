package bang

// Module provides a code prefix for a package or subsystem.
// All codes produced through Module are automatically prefixed.
//
//	var Mod = bang.NewModule("orders.repo")
//
//	Mod.NotFound().Code("get_by_id")  // code → "orders.repo.get_by_id"
//	Mod.New("get_by_id")              // code → "orders.repo.get_by_id"
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

func (m Module) newFromClass(class Class) *Error {
	e := newError(class, 3) // skip: method → newFromClass → newError
	e.code = m.prefix
	return e
}

// ─── Shorthand constructors ─────────────────────────────────────────────────

func (m Module) InternalServer() *Error       { return m.newFromClass(ClassInternalServer) }
func (m Module) NotFound() *Error             { return m.newFromClass(ClassNotFound) }
func (m Module) BadRequest() *Error           { return m.newFromClass(ClassBadRequest) }
func (m Module) Unauthorized() *Error         { return m.newFromClass(ClassUnauthorized) }
func (m Module) Forbidden() *Error            { return m.newFromClass(ClassForbidden) }
func (m Module) Conflict() *Error             { return m.newFromClass(ClassConflict) }
func (m Module) TooManyRequests() *Error      { return m.newFromClass(ClassTooManyRequests) }
func (m Module) DuplicateKey() *Error         { return m.newFromClass(ClassDuplicateKey) }
func (m Module) ForeignKeyViolation() *Error  { return m.newFromClass(ClassForeignKeyViolation) }
func (m Module) ExternalServiceError() *Error { return m.newFromClass(ClassExternalService) }
func (m Module) NetworkError() *Error         { return m.newFromClass(ClassNetwork) }
func (m Module) TimeoutError() *Error         { return m.newFromClass(ClassTimeout) }
func (m Module) SerializationError() *Error   { return m.newFromClass(ClassSerialization) }
func (m Module) Unexpected() *Error           { return m.newFromClass(ClassUnexpected) }
func (m Module) Validation() *Error           { return m.newFromClass(ClassValidation) }

// New creates an error from a code, prefixed with the module prefix.
// If the full code is registered, its config is applied.
func (m Module) New(code string) *Error {
	fullCode := m.fullCode(code)
	e := New(fullCode) // uses the global New which checks registry
	return e
}

// FromClass creates an error with an arbitrary class and the module prefix as code.
func (m Module) FromClass(class Class) *Error {
	e := newError(class, 2)
	e.code = m.prefix
	return e
}

// ─── Define / TypedFactory through Module ───────────────────────────────────

// Define creates an ErrFactory with the code prefixed by this module.
func (m Module) Define(code string, cfgs ...CodeCfg) ErrFactory {
	fullCode := m.fullCode(code)
	var cfg CodeCfg
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	registryMu.Lock()
	codeRegistry[fullCode] = cfg
	registryMu.Unlock()
	return Factory(fullCode)
}

// Register starts building an error definition with the code prefixed by this module.
func (m Module) Register(code string) *RegisterBuilder {
	return Register(m.fullCode(code))
}

// Factory creates an ErrFactory for a code prefixed by this module.
func (m Module) Factory(code string) ErrFactory {
	return Factory(m.fullCode(code))
}

// ─── Wrap adapters through Module ───────────────────────────────────────────

// WrapDBError wraps a DB error with auto-classification and module-prefixed code.
func (m Module) WrapDBError(err error, code ...string) *Error {
	e := WrapDBError(err)
	if e != nil {
		if len(code) > 0 {
			e.code = m.fullCode(code[0])
		} else {
			e.code = m.prefix
		}
	}
	return e
}

// WrapRedisError wraps a Redis error with auto-classification and module-prefixed code.
func (m Module) WrapRedisError(err error, code ...string) *Error {
	e := WrapRedisError(err)
	if e != nil {
		if len(code) > 0 {
			e.code = m.fullCode(code[0])
		} else {
			e.code = m.prefix
		}
	}
	return e
}
