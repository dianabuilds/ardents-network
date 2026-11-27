// Package systemd renders the fixed infrastructure unit and performs its native
// Linux manager observations. Detached typed properties live in observation.
// Unit policy, Release authorization and process completion stay with callers.
//
// infrastructure_unit.go and infrastructure_unit_template.go own the portable
// unit declaration and exact template. infrastructure_prestart_linux.go checks
// the loaded policy against the instance's original control process.
//
// The fixed role and Endpoint references retain their own original manager.
// Each transport admits its exact read-only reply vocabulary and joins the
// original connection on Close. The absence-only connection admits no unit
// objects. Job objects are refused by every connection. Endpoint retains its
// fixed unit reference without acquiring Start or Stop authority.
//
// startup_observation_linux.go reads the version and two complete fixed-object
// rounds on one fresh connection. manager_properties_linux.go preserves the
// native signatures, numeric values and finite bounds when projecting replies.
// Cancellation joins original calls and returns no successful partial round.
// Protocol, identity and actual transport failures keep their original causes.
package systemd

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

var (
	ErrInput       = errors.New("invalid Installation manager observation")
	ErrObservation = observation.ErrObservation
	ErrUnavailable = errors.New("installation manager observation unavailable")
)
