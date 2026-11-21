//go:build linux

package processresource

import "errors"

// Failure returns copied original facts through wrapped/joined errors, without
// exposing native error text or mutable retained diagnostic pointers.
func Failure(err error) *FailureFacts {
	var failure *observedFailure
	if !errors.As(err, &failure) {
		return nil
	}
	facts := failure.facts
	if facts.ElapsedMS != nil {
		copy := *facts.ElapsedMS
		facts.ElapsedMS = &copy
	}
	if facts.Observation != nil {
		copy := *facts.Observation
		facts.Observation = &copy
	}
	if facts.DeadlineRemainingMS != nil {
		copy := *facts.DeadlineRemainingMS
		facts.DeadlineRemainingMS = &copy
	}
	return &facts
}
