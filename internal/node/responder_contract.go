package node

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// ResponderPeer is the sole State-authorized Rendezvous neighbour for a
// Publisher-side C-2 attachment. It is neither discovery nor a fallback set.
type responderPeer struct {
	NodeID, PublicKey [32]byte
	Endpoint          string
	CarrierProfile    route.CarrierProfile
}

// responderConfig supplies one authenticated State projection and a narrow
// C-2 first-hop admission port. It has no Service key, Introduction plaintext,
// browser destination, or peer-selection authority.
type responderConfig struct {
	ListenAddress                  string
	Certificate                    tls.Certificate
	NetworkID, EpochDigest, NodeID [32]byte
	NodePublicKey                  [32]byte
	Epoch                          uint64
	NotAfter                       time.Time
	rendezvous                     responderPeer
	Admit                          route.EndpointTransitBindingAdmitter
	HandshakeLimit, RelayLimit     uint16
	RelayByteLimit                 uint64
	AdmissionTimeout               time.Duration
	DrainTimeout                   time.Duration
}

type responderUsage struct {
	Handshakes, ActiveRelays, Connections uint16
	CompletedRelays, RefusedBeforeTLS     uint64
	RelayRefused, RelayedBytes            uint64
}

type responderPlan struct {
	responderConfig
	now func() time.Time
}

func newResponderPlan(input responderConfig) (responderPlan, error) {
	if !literalNodeEndpoint(input.ListenAddress) || input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.NodeID == [32]byte{} || input.NodePublicKey == [32]byte{} || input.Epoch == 0 || input.NotAfter.IsZero() ||
		input.rendezvous.NodeID == [32]byte{} || input.rendezvous.PublicKey == [32]byte{} || input.rendezvous.NodeID == input.NodeID ||
		!literalNodeEndpoint(input.rendezvous.Endpoint) || !supportedCarrier(input.rendezvous.CarrierProfile) || input.Admit == nil || input.HandshakeLimit == 0 || input.RelayLimit == 0 ||
		input.RelayByteLimit == 0 || input.RelayByteLimit > uint64(1<<63-1) || !validAdmissionTimeout(input.AdmissionTimeout) || input.DrainTimeout <= 0 || input.DrainTimeout > time.Minute ||
		!input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) || !time.Now().UTC().Before(input.NotAfter) {
		return responderPlan{}, errors.New("responder duty configuration is incomplete or outside its implementation bound")
	}
	if err := validateNodeCertificate(input.Certificate, input.NodePublicKey); err != nil {
		return responderPlan{}, err
	}
	return responderPlan{responderConfig: input, now: func() time.Time { return time.Now().UTC() }}, nil
}
