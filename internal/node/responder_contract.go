package node

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// ResponderPeer is the sole State-authorized Rendezvous neighbour for a
// Publisher-side C-2 attachment. It is neither discovery nor a fallback set.
type ResponderPeer struct {
	NodeID, PublicKey [32]byte
	Endpoint          string
	CarrierProfile    route.CarrierProfile
}

// ResponderConfig supplies one authenticated State snapshot and a narrow
// C-2 first-hop admission port. It has no Service key, Introduction plaintext,
// browser destination, or peer-selection authority.
type ResponderConfig struct {
	ListenAddress                  string
	Certificate                    tls.Certificate
	NetworkID, EpochDigest, NodeID [32]byte
	NodePublicKey                  [32]byte
	Epoch                          uint64
	NotAfter                       time.Time
	Rendezvous                     ResponderPeer
	Admit                          route.EndpointTransitBindingAdmitter
	HandshakeLimit, RelayLimit     uint16
	RelayByteLimit                 uint64
	AdmissionTimeout               time.Duration
	DrainTimeout                   time.Duration
}

type ResponderUsage struct {
	Handshakes, ActiveRelays, Connections uint16
	CompletedRelays, RefusedBeforeTLS     uint64
	RelayRefused, RelayedBytes            uint64
}

type responderPlan struct {
	ResponderConfig
	now func() time.Time
}

func newResponderPlan(input ResponderConfig) (responderPlan, error) {
	if !literalNodeEndpoint(input.ListenAddress) || input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.NodeID == [32]byte{} || input.NodePublicKey == [32]byte{} || input.Epoch == 0 || input.NotAfter.IsZero() ||
		input.Rendezvous.NodeID == [32]byte{} || input.Rendezvous.PublicKey == [32]byte{} || input.Rendezvous.NodeID == input.NodeID ||
		!literalNodeEndpoint(input.Rendezvous.Endpoint) || !supportedCarrier(input.Rendezvous.CarrierProfile) || input.Admit == nil || input.HandshakeLimit == 0 || input.RelayLimit == 0 ||
		input.RelayByteLimit == 0 || input.RelayByteLimit > uint64(1<<63-1) || !validAdmissionTimeout(input.AdmissionTimeout) || input.DrainTimeout <= 0 || input.DrainTimeout > time.Minute ||
		!input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) || !time.Now().UTC().Before(input.NotAfter) {
		return responderPlan{}, errors.New("Responder duty configuration is incomplete or outside its implementation bound")
	}
	if err := validateNodeCertificate(input.Certificate, input.NodePublicKey); err != nil {
		return responderPlan{}, err
	}
	return responderPlan{ResponderConfig: input, now: func() time.Time { return time.Now().UTC() }}, nil
}
