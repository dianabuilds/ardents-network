package broker

import "time"

const (
	Connection         Surface = "connection"
	Administration     Surface = "administration"
	GenericUnqualified         = "generic/unqualified"
)

// Surface is the closed M10 set of generic local application surfaces.
type Surface string

// Grant permits one opaque local Principal to use exactly one Application
// Interface surface. It is not a network credential or sandbox assertion.
type Grant struct {
	Principal   [32]byte
	Surface     Surface
	PermitDrain bool
}

// Config fixes one volatile Broker generation and its explicit grants.
type Config struct {
	ID     [32]byte
	Grants []Grant
	Clock  func() time.Time
}

// Receipt records one consumed admission capability and its finite issue and
// expiry window without exporting the underlying Principal or Local Grant
// representation. ExpiresAt is not an active Connection lease deadline.
type Receipt struct {
	Session, Principal, Broker, Grant [32]byte
	Surface                           Surface
	IssuedAt, ExpiresAt               int64
}

// IsolationObservation makes the absence of a qualified local Isolation
// Boundary explicit to every generic Broker caller.
type IsolationObservation struct{ state string }

// State returns the selected isolation observation or an empty value for an
// uninitialized observation.
func (observation IsolationObservation) State() string { return observation.state }
