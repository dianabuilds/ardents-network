package decision

import "ardents/internal/policy/reason"

type Result struct {
	Allowed bool
	Reason  reason.Denial
}

func Allow() Result {
	return Result{Allowed: true}
}

func Deny(code, message string) Result {
	return Result{Reason: reason.New(code, message)}
}

func (r Result) Error() error {
	if r.Allowed {
		return nil
	}
	return r.Reason
}
