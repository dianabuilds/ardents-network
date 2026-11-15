// Package systemdproperty validates detached, typed systemd observation values.
// It grants no manager connection, process, installation or runtime authority.
package systemdproperty

import "errors"

// ErrObservation indicates an invalid bounded manager observation.
var ErrObservation = errors.New("installation manager observation differs")
