package desiredstate

import "strings"

const (
	Present  = "present"
	Running  = "running"
	Stopped  = "stopped"
	Disabled = "disabled"
	Removed  = "removed"
)

func Normalize(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case Running:
		return Running
	case Stopped:
		return Stopped
	case Disabled:
		return Disabled
	case Removed:
		return Removed
	default:
		return Present
	}
}
