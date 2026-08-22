//go:build !windows

package update

// isActivationBusy has no portable non-Windows equivalent. Native rename
// failures on these platforms are retained as their original failure evidence
// for deterministic recovery rather than being guessed to be contention.
func isActivationBusy(error) bool { return false }
