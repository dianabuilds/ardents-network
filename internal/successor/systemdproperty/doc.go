// Package systemdproperty validates detached, typed systemd observation values.
// It grants no manager connection, process, installation or runtime authority.
package systemdproperty

import "errors"

// Shared refusal identities preserve manager and observation errors across owners.
var (
	ErrInput       = errors.New("invalid Installation manager observation")
	ErrObservation = errors.New("installation manager observation differs")
	ErrUnavailable = errors.New("installation manager observation unavailable")
)
