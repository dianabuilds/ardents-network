//go:build linux

package introduction

// Refusal identifies only a rejected input, never a failed
// delivery acknowledgement, lost authority or resource-cleanup failure.
// The receive owner may retain publication after this input has been refused.
type Refusal struct{ cause error }

// NewRefusal scopes one rejected-input cause.
func NewRefusal(cause error) *Refusal { return &Refusal{cause: cause} }

func (refusal *Refusal) Error() string { return refusal.cause.Error() }
func (refusal *Refusal) Unwrap() error { return refusal.cause }

// OnlyRefusal reports whether every joined branch is a scoped refusal.
// errors.As alone would hide a simultaneous cancellation or cleanup failure
// behind another branch.
func OnlyRefusal(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*Refusal); ok {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !OnlyRefusal(cause) {
				return false
			}
		}
		return true
	}
	return false
}
