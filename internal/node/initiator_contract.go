package node

import (
	"crypto/tls"
	"errors"
	"net/url"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// InitiatorPeer is one exact State-authorized adjacent Node fact. It is neither
// discovery input nor a fallback list.
type InitiatorPeer struct {
	NodeID, PublicKey [32]byte
	Endpoint          string
}

// ResolutionGateway is the separate State-authorized OHTTP Gateway fact for
// the finite private lookup operation. Its HTTPS URL is never supplied by the
// Endpoint setup; the Initiator alone obtains it from State.
type ResolutionGateway struct {
	NodeID, PublicKey [32]byte
	URL               string
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
	ResolutionGateway              ResolutionGateway
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
		input.Rendezvous.NodeID == input.NodeID || !literalNodeEndpoint(input.Rendezvous.Endpoint) || !validOptionalInitiatorPeer(input.ResolutionGateway, input.NodeID) || input.Admit == nil ||
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

func validOptionalInitiatorPeer(peer ResolutionGateway, local [32]byte) bool {
	empty := peer.NodeID == [32]byte{} && peer.PublicKey == [32]byte{} && peer.URL == ""
	if empty {
		return true
	}
	parsed, err := url.Parse(peer.URL)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" &&
		peer.NodeID != [32]byte{} && peer.PublicKey != [32]byte{} && peer.NodeID != local
}
