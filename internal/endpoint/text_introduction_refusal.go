//go:build linux

package endpoint

// textIntroductionRefusal identifies only a rejected input, never a failed
// delivery acknowledgement, lost authority or resource-cleanup failure.
// The receive owner may retain publication after this input has been refused.
type textIntroductionRefusal struct{ cause error }

func (refusal *textIntroductionRefusal) Error() string { return refusal.cause.Error() }
func (refusal *textIntroductionRefusal) Unwrap() error { return refusal.cause }

// Every joined branch must be a scoped refusal. errors.As alone would hide
// a simultaneous cancellation or cleanup failure behind another branch.
func onlyTextIntroductionRefusal(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*textIntroductionRefusal); ok {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !onlyTextIntroductionRefusal(cause) {
				return false
			}
		}
		return true
	}
	return false
}
