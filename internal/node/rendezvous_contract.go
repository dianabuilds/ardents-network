package node

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// RendezvousPeer is one State-authorized adjacent Node identity for one side
// of a Rendezvous attachment. It is not a discovery record or a fallback.
type RendezvousPeer struct {
	NodeID, PublicKey [32]byte
	Role              byte
}

// RendezvousConfig supplies one authenticated duty snapshot to the first
// native Rendezvous listener. State/assignment integration owns producing this
// input; this duty never discovers a peer or selects another profile.
type RendezvousConfig struct {
	ListenAddress                           string
	Certificate                             tls.Certificate
	NetworkID, EpochDigest, NodeID          [32]byte
	NodePublicKey                           [32]byte
	Epoch                                   uint64
	NotAfter                                time.Time
	Peers                                   []RendezvousPeer
	HandshakeLimit, WaitingLimit, PairLimit uint16
	DrainTimeout                            time.Duration
	// Now is a behavior-test seam. A maintained caller leaves it nil.
	Now func() time.Time
}

// RendezvousUsage contains aggregate, non-secret local reservation and
// terminal counters. It deliberately contains no peer addresses, bindings, or
// complete route history.
type RendezvousUsage struct {
	Handshakes, WaitingLegs, ActivePairs, Connections uint16
	CompletedPairs, RefusedBeforeTLS                  uint64
	DuplicateSideRejected, WaitingRefused, Expired    uint64
}

type rendezvousPlan struct {
	RendezvousConfig
	now           func() time.Time
	peersByNode   map[[32]byte]RendezvousPeer
	peersByPublic map[[32]byte]RendezvousPeer
}

func newRendezvousPlan(input RendezvousConfig) (rendezvousPlan, error) {
	if !literalRendezvousEndpoint(input.ListenAddress) || input.NetworkID == [32]byte{} ||
		input.EpochDigest == [32]byte{} || input.NodeID == [32]byte{} || input.NodePublicKey == [32]byte{} ||
		input.Epoch == 0 || input.NotAfter.IsZero() || input.HandshakeLimit == 0 || input.WaitingLimit == 0 ||
		input.PairLimit == 0 || input.HandshakeLimit > 64 || input.WaitingLimit > 64 || input.PairLimit > 64 ||
		input.DrainTimeout <= 0 || input.DrainTimeout > time.Minute {
		return rendezvousPlan{}, errors.New("Rendezvous duty configuration is incomplete or outside its implementation bound")
	}
	if input.Now == nil {
		input.Now = time.Now
	}
	now := func() time.Time { return input.Now().UTC() }
	if !now().Before(input.NotAfter.UTC()) {
		return rendezvousPlan{}, errors.New("Rendezvous duty has expired")
	}
	if !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return rendezvousPlan{}, errors.New("Rendezvous duty expiry must use whole UTC seconds")
	}
	if err := validateRendezvousCertificate(input.Certificate, input.NodePublicKey); err != nil {
		return rendezvousPlan{}, err
	}
	result := rendezvousPlan{RendezvousConfig: input, now: now, peersByNode: make(map[[32]byte]RendezvousPeer, len(input.Peers)),
		peersByPublic: make(map[[32]byte]RendezvousPeer, len(input.Peers))}
	if len(input.Peers) != 2 {
		return rendezvousPlan{}, errors.New("Rendezvous duty requires one Initiator and one Responder peer")
	}
	for _, peer := range input.Peers {
		if peer.NodeID == [32]byte{} || peer.PublicKey == [32]byte{} || peer.NodeID == input.NodeID ||
			(peer.Role != route.InitiatorRole && peer.Role != route.ResponderRole) {
			return rendezvousPlan{}, errors.New("Rendezvous peer is invalid")
		}
		if _, exists := result.peersByNode[peer.NodeID]; exists {
			return rendezvousPlan{}, errors.New("Rendezvous peer Node identity is duplicated")
		}
		if _, exists := result.peersByPublic[peer.PublicKey]; exists {
			return rendezvousPlan{}, errors.New("Rendezvous peer public key is duplicated")
		}
		result.peersByNode[peer.NodeID], result.peersByPublic[peer.PublicKey] = peer, peer
	}
	if result.peersByNodeForRole(route.InitiatorRole).NodeID == [32]byte{} ||
		result.peersByNodeForRole(route.ResponderRole).NodeID == [32]byte{} {
		return rendezvousPlan{}, errors.New("Rendezvous peer sides are incomplete")
	}
	return result, nil
}

func (plan rendezvousPlan) peersByNodeForRole(role byte) RendezvousPeer {
	for _, peer := range plan.peersByNode {
		if peer.Role == role {
			return peer
		}
	}
	return RendezvousPeer{}
}

func literalRendezvousEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}

func validateRendezvousCertificate(certificate tls.Certificate, expected [32]byte) error {
	if certificate.PrivateKey == nil || len(certificate.Certificate) != 1 {
		return errors.New("Rendezvous TLS certificate is invalid")
	}
	leaf := certificate.Leaf
	if leaf == nil {
		var err error
		leaf, err = x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			return errors.New("Rendezvous TLS certificate leaf is invalid")
		}
	}
	public, ok := leaf.PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || string(public) != string(expected[:]) {
		return errors.New("Rendezvous TLS certificate does not match Node public key")
	}
	return nil
}
