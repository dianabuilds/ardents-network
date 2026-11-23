//go:build linux


// Package serving owns the original Direct-Origin Source operation. It
// acquires bounded inputs and the process guard, opens the actual Network
// State, and joins original State, observations and inputs before handing
// completion back to the State owner. The command reports bounded events and
// maps outcomes to process status. State retains all signed authority and
// participation policy; this package cannot make configuration authoritative.
package serving
