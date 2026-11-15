// Package manager owns fixed Endpoint systemd observations and commands.
// It cannot choose a unit or authorize delivery, a process, or a transition.
package manager

import "github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"

var (
	ErrInput       = systemdproperty.ErrInput
	ErrObservation = systemdproperty.ErrObservation
	ErrUnavailable = systemdproperty.ErrUnavailable
)
