//go:build linux

package endpoint

// introductionRefusal identifies only a rejected input, never a failed
// delivery acknowledgement, lost authority or resource-cleanup failure.
// The receive owner may retain publication after this input has been refused.
type introductionRefusal struct{ cause error }

func (refusal *introductionRefusal) Error() string { return refusal.cause.Error() }
func (refusal *introductionRefusal) Unwrap() error { return refusal.cause }

// Every joined branch must be a scoped refusal. errors.As alone would hide
// a simultaneous cancellation or cleanup failure behind another branch.
func onlyIntroductionRefusal(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*introductionRefusal); ok {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !onlyIntroductionRefusal(cause) {
				return false
			}
		}
		return true
	}
	return false
}
