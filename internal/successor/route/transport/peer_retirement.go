package transport

// peerRetirement retains the native adapter cause. Classification is diagnostic:
// it does not establish successful completion or replace a physical join.
type peerRetirement struct{ cause error }

func (e *peerRetirement) Error() string { return e.cause.Error() }
func (e *peerRetirement) Unwrap() error { return e.cause }

// MarkPeerRetirement records an adapter's exact native peer-close observation.
// Adapters must classify individual causes before marking; an aggregate cannot
// acquire the category from just one branch. Original causes remain available.
func MarkPeerRetirement(cause error) error {
	if cause == nil {
		return nil
	}
	// Native errors may unwrap a standard closed sentinel. Reject aggregates
	// anywhere in that chain rather than confusing unwrapping with aggregation.
	for current := cause; current != nil; {
		if _, ok := current.(interface{ Unwrap() []error }); ok {
			return cause
		}
		wrapped, ok := current.(interface{ Unwrap() error })
		if !ok {
			break
		}
		current = wrapped.Unwrap()
	}
	return &peerRetirement{cause: cause}
}

// IsPeerRetirementCause requires every leaf to represent peer retirement.
// Local shutdown, timeout and unrelated failures cannot inherit another leaf's
// category. Native socket errno recognition is selected by the host platform.
func IsPeerRetirementCause(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !IsPeerRetirementCause(child) {
				return false
			}
		}
		return true
	}
	if _, ok := err.(*peerRetirement); ok {
		return true
	}
	if isPeerSocketRetirement(err) {
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return IsPeerRetirementCause(wrapped.Unwrap())
	}
	return false
}
