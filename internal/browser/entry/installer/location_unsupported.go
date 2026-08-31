//go:build !windows && !linux

package installer

// defaultLocation deliberately has no value outside the selected Windows and
// Ubuntu participant profiles. Install and remove then fail closed rather than
// write a Linux-shaped Firefox registration on an unqualified platform.
func defaultLocation() location {
	return location{}
}
