package node

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// InitiatorPeer is the single State-authorized next Rendezvous Node for an
// Initiator duty. It is neither discovery input nor a fallback list.
type InitiatorPeer struct {
	NodeID, PublicKey [32]byte
	Endpoint          string
}

// InitiatorConfig supplies one authenticated State snapshot and the narrow
// Entry admission port for an Initiator transit duty.
type InitiatorConfig struct {
	ListenAddress                  string
	Certificate                    tls.Certificate
	NetworkID, EpochDigest, NodeID [32]byte
	NodePublicKey                  [32]byte
	Epoch                          uint64
	NotAfter                       time.Time
	Rendezvous                     InitiatorPeer
	Admit                          route.EntryBindingAdmitter
	HandshakeLimit, RelayLimit     uint16
	RelayByteLimit                 uint64
	DrainTimeout                   time.Duration
}

// InitiatorUsage contains aggregate local reservations and terminal transit
// counters. It exposes neither targets nor peer addresses.
type InitiatorUsage struct {
	Handshakes, ActiveRelays, Connections uint16
	CompletedRelays, RefusedBeforeTLS     uint64
	SetupRefused, RelayedBytes            uint64
}

type initiatorPlan struct {
	InitiatorConfig
	now func() time.Time
}

func newInitiatorPlan(input InitiatorConfig) (initiatorPlan, error) {
	if !literalNodeEndpoint(input.ListenAddress) || input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.NodeID == [32]byte{} || input.NodePublicKey == [32]byte{} || input.Epoch == 0 || input.NotAfter.IsZero() ||
		input.Rendezvous.NodeID == [32]byte{} || input.Rendezvous.PublicKey == [32]byte{} ||
		input.Rendezvous.NodeID == input.NodeID || !literalNodeEndpoint(input.Rendezvous.Endpoint) || input.Admit == nil ||
		input.HandshakeLimit == 0 || input.RelayLimit == 0 || input.RelayByteLimit == 0 || input.RelayByteLimit > uint64(1<<63-1) ||
		input.DrainTimeout <= 0 || input.DrainTimeout > time.Minute {
		return initiatorPlan{}, errors.New("Initiator duty configuration is incomplete or outside its implementation bound")
	}
	if !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) || !time.Now().UTC().Before(input.NotAfter) {
		return initiatorPlan{}, errors.New("Initiator duty has expired or has an invalid expiry")
	}
	if err := validateNodeCertificate(input.Certificate, input.NodePublicKey); err != nil {
		return initiatorPlan{}, err
	}
	return initiatorPlan{InitiatorConfig: input, now: func() time.Time { return time.Now().UTC() }}, nil
}
