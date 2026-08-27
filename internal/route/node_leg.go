package route

import (
	"context"
	"crypto/tls"
	"errors"
	"time"
)

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

// NodeLegRequest is one State-pinned adjacent Node leg. It is usable only by
// a transit duty that already owns its peer and Carrier Profile selection. It
// contains no Service, Application, fallback, or retry policy.
type NodeLegRequest struct {
	CarrierProfile  CarrierProfile
	Endpoint        string
	Certificate     tls.Certificate
	ExpectedPeerKey [32]byte
	Binding         LegBinding
	Deadline        time.Time
}

type nodeCarrierResult struct {
	lane  Carrier
	state tls.ConnectionState
	abort func() error
}

type nodeCarrierAdapter func(context.Context, NodeLegRequest) (nodeCarrierResult, error)

// OpenNodeLeg opens exactly the requested Carrier Profile, authenticates the
// State-pinned peer, and confirms the reciprocal LegBinding. It never selects,
// races, retries, or downgrades to another profile.
func OpenNodeLeg(ctx context.Context, input NodeLegRequest) (Carrier, error) {
	adapter, err := preflightNodeCarrier(ctx, input)
	if err != nil {
		return nil, err
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	result, err := adapter(attempt, input)
	if err != nil {
		return nil, classifyCarrierFailure(err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = result.abort()
		}
	}()
	if result.state.Version != tls.VersionTLS13 || result.state.NegotiatedProtocol != Profile {
		return nil, carrierFailure(CarrierFailureUnauthorized, errors.New("native Node Carrier TLS state is invalid"))
	}
	if err := exactPeer(input.ExpectedPeerKey)(result.state); err != nil {
		return nil, carrierFailure(CarrierFailureUnauthorized, err)
	}
	if err := ConfirmNodeLegBinding(result.lane, input.Binding); err != nil {
		return nil, carrierFailure(CarrierFailureUnauthorized, err)
	}
	if err := result.lane.SetDeadline(time.Time{}); err != nil {
		return nil, classifyCarrierFailure(err)
	}
	keep = true
	return authenticatedCarrier(result.lane), nil
}

func preflightNodeCarrier(ctx context.Context, input NodeLegRequest) (nodeCarrierAdapter, error) {
	if ctx == nil || !literalEndpoint(input.Endpoint) || input.Certificate.PrivateKey == nil || input.ExpectedPeerKey == [32]byte{} ||
		input.Deadline.IsZero() || validLegBinding(input.Binding) != nil ||
		input.Binding.NotAfter.After(input.Deadline) {
		return nil, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier request is invalid"))
	}
	if !time.Now().Before(input.Deadline) {
		return nil, carrierFailure(CarrierFailureStale, errors.New("native Node Carrier request is stale"))
	}
	switch input.CarrierProfile {
	case CarrierTCP:
		return openTCPNodeCarrier, nil
	case CarrierQUIC:
		return openQUICNodeCarrier, nil
	default:
		return nil, carrierFailure(CarrierFailureIncompatible, errors.New("native Node Carrier Profile is unsupported"))
	}
}
