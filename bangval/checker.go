package bangval

import "github.com/smdmitry/bang"

// FieldError describes a single custom validation failure.
// Returned by check functions to indicate what went wrong.
type FieldError struct {
	Rule   string // machine-readable rule name (e.g., "serial_length")
	Reason string // human-readable message in Russian
}

// Fail creates a FieldError. Use inside check functions.
//
//	return bangval.Fail("serial_length", "Серия паспорта — 4 цифры")
func Fail(rule, reason string) *FieldError {
	return &FieldError{Rule: rule, Reason: reason}
}

// ─── Checker ────────────────────────────────────────────────────────────────

// Checker accumulates custom validation errors and produces a single
// *bang.Error with ClassValidation.
//
//	c := bangval.NewChecker("users.register")
//	c.Check("email", func() *bangval.FieldError { ... })
//	c.Check("password", func() *bangval.FieldError { ... })
//	return c.Err() // nil or *bang.Error
type Checker struct {
	code   string
	fields []bang.ValidationField
}

// NewChecker creates a Checker with the given error code.
func NewChecker(code string) *Checker {
	return &Checker{code: code}
}

// Check runs fn and, if it returns a *FieldError, records it under key.
// fn captures the value to check via closure — no type assertions needed.
//
//	c.Check("passports.0.serial", func() *bangval.FieldError {
//	    if len(serial) != 4 {
//	        return bangval.Fail("serial_length", "Серия паспорта должна содержать 4 цифры")
//	    }
//	    if !allDigits(serial) {
//	        return bangval.Fail("serial_digits", "Серия паспорта должна состоять только из цифр")
//	    }
//	    return nil
//	})
func (c *Checker) Check(key string, fn func() *FieldError) *Checker {
	if fe := fn(); fe != nil {
		c.fields = append(c.fields, bang.ValidationField{
			Key:    key,
			Rule:   fe.Rule,
			Reason: fe.Reason,
		})
	}
	return c
}

// CheckAll runs fn which may return multiple errors for a single key.
// Useful when one check function validates several aspects of a field
// and wants to report all failures, not just the first.
//
//	c.CheckAll("passports.0.serial", func() []bangval.FieldError {
//	    var errs []bangval.FieldError
//	    if len(serial) != 4 {
//	        errs = append(errs, *bangval.Fail("serial_length", "Серия — 4 цифры"))
//	    }
//	    if !allDigits(serial) {
//	        errs = append(errs, *bangval.Fail("serial_digits", "Только цифры"))
//	    }
//	    return errs
//	})
func (c *Checker) CheckAll(key string, fn func() []FieldError) *Checker {
	for _, fe := range fn() {
		c.fields = append(c.fields, bang.ValidationField{
			Key:    key,
			Rule:   fe.Rule,
			Reason: fe.Reason,
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
	e := bang.Validation(c.code)
	e.Fields(c.fields...)
	return e
}

// ─── Reusable check functions ───────────────────────────────────────────────

// CheckFunc is a reusable validation function for a specific type.
// Define once, use in multiple Checker.Check calls.
//
//	var checkPassportSerial = bangval.CheckFunc[string](func(v string) *bangval.FieldError {
//	    if len(v) != 4 {
//	        return bangval.Fail("serial_length", "Серия паспорта — 4 цифры")
//	    }
//	    if !allDigits(v) {
//	        return bangval.Fail("serial_digits", "Серия паспорта — только цифры")
//	    }
//	    return nil
//	})
//
//	// Usage:
//	c.Check("passports.0.serial", checkPassportSerial.Bind(p.Serial))
//	c.Check("passports.1.serial", checkPassportSerial.Bind(p2.Serial))
type CheckFn[T any] struct {
	fn func(T) *FieldError
}

// NewCheckFn creates a reusable typed check function.
func NewCheckFn[T any](fn func(T) *FieldError) CheckFn[T] {
	return CheckFn[T]{fn: fn}
}

// Bind returns a closure that checks the given value.
// Pass the result directly to Checker.Check():
//
//	c.Check("serial", checkSerial.Bind(passport.Serial))
func (cf CheckFn[T]) Bind(value T) func() *FieldError {
	return func() *FieldError {
		return cf.fn(value)
	}
}

// Run executes the check immediately and returns the result.
func (cf CheckFn[T]) Run(value T) *FieldError {
	return cf.fn(value)
}
