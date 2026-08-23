package broker

import "time"

const (
	Connection     = "connection"
	Administration = "administration"
)

// Grant permits one opaque local Principal to use exactly one Application
// Interface surface. It is not a network credential or sandbox assertion.
type Grant struct {
	Principal [32]byte
	Surface   string
}

// Config fixes one volatile Broker generation and its explicit grants.
type Config struct {
	ID     [32]byte
	Grants []Grant
	Clock  func() time.Time
}

// Receipt records one consumed ephemeral session without exporting the
// underlying Principal or Local Grant representation.
type Receipt struct {
	Session, Principal, Broker, Grant [32]byte
	Surface                           string
	IssuedAt, ExpiresAt               int64
}

// IsolationObservation makes the absence of a qualified local Isolation
// Boundary explicit to every generic Broker caller.
type IsolationObservation struct{ State string }
