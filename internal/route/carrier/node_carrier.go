package carrier

import "time"

// CarrierProfile is one exact State-authorized adjacent-leg transport
// implementation. It is not a retry order or a peer-advertised preference.
type CarrierProfile string

// Carrier is the complete transport-neutral byte lane returned to Route/Node.
// Transport addresses, QUIC state, fallback and migration stay private.
type Carrier interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	SetDeadline(time.Time) error
	Close() error
}

// ClosedTLSExporter derives secret channel binding bytes after role TLS has
// already authenticated its exact server key. Route retains the result but
// never serializes or forwards it.
type ClosedTLSExporter func(string, []byte, int) ([]byte, error)
