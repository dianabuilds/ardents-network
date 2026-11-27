package state

import "context"

// sourceInspectionLocalClosure reports whether every error leaf is a local
// Source inspection close or cancellation rather than an operational failure.
func sourceInspectionLocalClosure(err error) bool {
	if err == nil {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, leaf := range many.Unwrap() {
			if !sourceInspectionLocalClosure(leaf) {
				return false
			}
		}
		return true
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return sourceInspectionLocalClosure(one.Unwrap())
	}
	return err == context.Canceled || err == errSourceInspectionClosed
}
