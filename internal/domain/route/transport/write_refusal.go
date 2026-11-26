package transport

import (
	"errors"
	"net"
)

// unstartedWrite retains a failed write whose native adapter proved no socket
// output started across the complete record. It grants no peer or success fact.
type unstartedWrite struct{ cause error }

func (e *unstartedWrite) Error() string { return e.cause.Error() }
func (e *unstartedWrite) Unwrap() error { return e.cause }

// MarkUnstartedWrite is an adapter-only provenance seam. The original closed
// cause remains failed; the adapter must retain unchanged native attempts,
// absence of active/previously failed writes, and its own closed socket guard.
func MarkUnstartedWrite(cause error) error {
	if !errors.Is(cause, net.ErrClosed) {
		return cause
	}
	for current := cause; current != nil; {
		if _, aggregate := current.(interface{ Unwrap() []error }); aggregate {
			return cause
		}
		wrapped, ok := current.(interface{ Unwrap() error })
		if !ok {
			break
		}
		current = wrapped.Unwrap()
	}
	return &unstartedWrite{cause: cause}
}

// IsUnstartedWrite recognizes only sealed whole-write adapter provenance.
// A raw closed error or aggregate cannot acquire this category by resemblance.
func IsUnstartedWrite(err error) bool {
	for err != nil {
		if _, aggregate := err.(interface{ Unwrap() []error }); aggregate {
			return false
		}
		if _, ok := err.(*unstartedWrite); ok {
			return true
		}
		wrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = wrapped.Unwrap()
	}
	return false
}
