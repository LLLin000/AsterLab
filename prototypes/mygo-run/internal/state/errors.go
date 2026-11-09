package state

import "errors"

var (
	ErrInvalid  = errors.New("invalid")
	ErrConflict = errors.New("conflict")
	ErrNotFound = errors.New("not found")
	ErrInternal = errors.New("internal")
)

func MapError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case hasPrefix(msg, "INVALID:"):
		return wrap(ErrInvalid, msg)
	case hasPrefix(msg, "NOT_FOUND:"):
		return wrap(ErrNotFound, msg)
	case hasPrefix(msg, "CONFLICT:"):
		return wrap(ErrConflict, msg)
	default:
		return wrap(ErrInternal, msg)
	}
}

func hasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func wrap(base error, msg string) error {
	return &typedError{base: base, msg: msg}
}

type typedError struct {
	base error
	msg  string
}

func (e *typedError) Error() string { return e.msg }
func (e *typedError) Is(target error) bool { return e.base == target }