package bangval

import "github.com/smdmitry/bang"

// FieldError describes a single custom validation failure.
type FieldError struct {
	Rule   string
	Reason string
}

// Fail creates a FieldError. Use inside check functions.
//
//	return bangval.Fail("serial_length", "Серия паспорта — 4 цифры")
func Fail(rule, reason string) *FieldError {
	return &FieldError{Rule: rule, Reason: reason}
}

// ─── Checker ────────────────────────────────────────────────────────────────

// Checker accumulates custom validation errors and produces a single *bang.Error.
type Checker struct {
	code   string
	fields []bang.ValidationField
}

// NewChecker creates a Checker with the given error code.
func NewChecker(code string) *Checker {
	return &Checker{code: code}
}

// Check runs fn and, if it returns a *FieldError, records it under key.
func (c *Checker) Check(key string, fn func() *FieldError) *Checker {
	if fe := fn(); fe != nil {
		c.fields = append(c.fields, bang.ValidationField{
			Key: key, Rule: fe.Rule, Reason: fe.Reason,
		})
	}
	return c
}

// CheckAll runs fn which may return multiple errors for a single key.
func (c *Checker) CheckAll(key string, fn func() []FieldError) *Checker {
	for _, fe := range fn() {
		c.fields = append(c.fields, bang.ValidationField{
			Key: key, Rule: fe.Rule, Reason: fe.Reason,
		})
	}
	return c
}

// AddField adds a pre-built validation field error directly.
func (c *Checker) AddField(key, rule, reason string) *Checker {
	c.fields = append(c.fields, bang.ValidationField{
		Key: key, Rule: rule, Reason: reason,
	})
	return c
}

// HasErrors reports whether any validation errors have been recorded.
func (c *Checker) HasErrors() bool {
	return len(c.fields) > 0
}

// Err returns a *bang.Error with all recorded validation fields,
// or nil if there are no errors.
func (c *Checker) Err() *bang.Error {
	if len(c.fields) == 0 {
		return nil
	}
	e := bang.Validation().Code(c.code)
	e.Fields(c.fields...)
	return e
}

// ─── Reusable check functions ───────────────────────────────────────────────

// CheckFn is a reusable validation function for a specific type.
//
//	var checkSerial = bangval.NewCheckFn(func(v string) *bangval.FieldError {
//	    if len(v) != 4 {
//	        return bangval.Fail("serial_length", "Серия паспорта — 4 цифры")
//	    }
//	    return nil
//	})
//
//	c.Check("passports.0.serial", checkSerial.Bind(p.Serial))
type CheckFn[T any] struct {
	fn func(T) *FieldError
}

// NewCheckFn creates a reusable typed check function.
func NewCheckFn[T any](fn func(T) *FieldError) CheckFn[T] {
	return CheckFn[T]{fn: fn}
}

// Bind returns a closure that checks the given value.
func (cf CheckFn[T]) Bind(value T) func() *FieldError {
	return func() *FieldError {
		return cf.fn(value)
	}
}

// Run executes the check immediately and returns the result.
func (cf CheckFn[T]) Run(value T) *FieldError {
	return cf.fn(value)
}
