package channel

import "time"

// MinDeadline retains the earlier original absolute bound for setup and
// terminal cleanup. It is the one shared tie-break for every Route owner:
// equal deadlines keep the first argument's exact reading, so the retained
// original bound (always passed first) keeps its identity, including any
// monotonic reading, regardless of which owner resolves the bound. Comparison
// itself follows time.Time.Before, so a monotonic reading still decides two
// live readings of the same instant. The helper grants no deadline authority;
// each owner still enforces its own original bound at its physical effects.
func MinDeadline(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
