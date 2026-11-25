package processfixture

// OnlyFailureBranches requires every retained failure branch to match accept.
// Some classifiers recognize an entire typed failure, including its own causes.
func OnlyFailureBranches(err error, accept func(error) bool) bool {
	if err == nil {
		return false
	}
	if accept(err) {
		return true
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		children := wrapped.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !OnlyFailureBranches(child, accept) {
				return false
			}
		}
		return true
	case interface{ Unwrap() error }:
		return OnlyFailureBranches(wrapped.Unwrap(), accept)
	default:
		return false
	}
}
