package bang

// classMatcher is implemented by internal sentinel errors that match by class.
type classMatcher interface {
	bangClass() Class
}

type classSentinel struct {
	class Class
}

func (s classSentinel) Error() string {
	return string(s.class)
}

func (s classSentinel) bangClass() Class {
	return s.class
}

var (
	ErrInternalServer  error = classSentinel{class: ClassInternalServer}
	ErrValidation      error = classSentinel{class: ClassValidation}
	ErrBadRequest      error = classSentinel{class: ClassBadRequest}
	ErrNotFound        error = classSentinel{class: ClassNotFound}
	ErrUnauthorized    error = classSentinel{class: ClassUnauthorized}
	ErrForbidden       error = classSentinel{class: ClassForbidden}
	ErrConflict        error = classSentinel{class: ClassConflict}
	ErrTooManyRequests error = classSentinel{class: ClassTooManyRequests}

	ErrDatabase            error = classSentinel{class: ClassDatabase}
	ErrDuplicateKey        error = classSentinel{class: ClassDuplicateKey}
	ErrForeignKeyViolation error = classSentinel{class: ClassForeignKeyViolation}

	ErrExternalService error = classSentinel{class: ClassExternalService}
	ErrNetwork         error = classSentinel{class: ClassNetwork}
	ErrTimeout         error = classSentinel{class: ClassTimeout}

	ErrSerialization  error = classSentinel{class: ClassSerialization}
	ErrUnexpected     error = classSentinel{class: ClassUnexpected}
	ErrNotImplemented error = classSentinel{class: ClassNotImplemented}
	ErrLocked         error = classSentinel{class: ClassLocked}
)
