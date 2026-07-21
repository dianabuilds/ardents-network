package policy

type Result struct {
	Allowed bool
	Reason  Denial
}

func Allow() Result {
	return Result{Allowed: true}
}

func Deny(code, message string) Result {
	return Result{Reason: newDenial(code, message)}
}

func (r Result) Error() error {
	if r.Allowed {
		return nil
	}
	return r.Reason
}
