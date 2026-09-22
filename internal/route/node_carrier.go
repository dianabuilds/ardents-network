package route

import "time"

// CarrierProfile is one exact State-authorized adjacent-leg transport
// implementation. It is not a retry order or a peer-advertised preference.
type CarrierProfile string

const (
	CarrierTCP  CarrierProfile = "ardents-carrier-tcp-tls-v1"
	CarrierQUIC CarrierProfile = "ardents-carrier-quic-v1"
)

// Carrier is the complete transport-neutral byte lane returned to Route/Node.
// Transport addresses, QUIC state, fallback and migration stay private.
type Carrier interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	SetDeadline(time.Time) error
	Close() error
}
