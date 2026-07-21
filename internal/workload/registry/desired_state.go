// Package registry owns admitted workload and hosted-service specifications.
// It does not own execution state or readiness observations.
package registry

import "strings"

const (
	DesiredPresent  = "present"
	DesiredRunning  = "running"
	DesiredStopped  = "stopped"
	DesiredDisabled = "disabled"
	DesiredRemoved  = "removed"
)

func NormalizeDesired(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case DesiredRunning:
		return DesiredRunning
	case DesiredStopped:
		return DesiredStopped
	case DesiredDisabled:
		return DesiredDisabled
	case DesiredRemoved:
		return DesiredRemoved
	default:
		return DesiredPresent
	}
}
