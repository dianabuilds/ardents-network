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
type rendezvousPeer struct {
	NodeID, PublicKey [32]byte
	Role              byte
}

// rendezvousConfig supplies one authenticated duty projection to the first
// native Rendezvous listener. State/assignment integration owns producing this
// input; this duty never discovers a peer or selects another profile.
type rendezvousConfig struct {
	ListenAddress                           string
	CarrierProfile                          route.CarrierProfile
	Certificate                             tls.Certificate
	NetworkID, EpochDigest, NodeID          [32]byte
	NodePublicKey                           [32]byte
	Epoch                                   uint64
	NotAfter                                time.Time
	Peers                                   []rendezvousPeer
	HandshakeLimit, WaitingLimit, PairLimit uint16
	PairByteLimit                           uint64
	AdmissionTimeout                        time.Duration
	DrainTimeout                            time.Duration
}

// RendezvousUsage contains aggregate, non-secret local reservation and
// terminal counters. It deliberately contains no peer addresses, bindings, or
// complete route history.
type rendezvousUsage struct {
	Handshakes, WaitingLegs, ActivePairs, Connections uint16
	CompletedPairs, RefusedBeforeTLS                  uint64
	DuplicateSideRejected, WaitingRefused, Expired    uint64
	RelayedBytes                                      uint64
}

type rendezvousPlan struct {
	rendezvousConfig
	now           func() time.Time
	peersByNode   map[[32]byte]rendezvousPeer
	peersByPublic map[[32]byte]rendezvousPeer
}

func newRendezvousPlan(input rendezvousConfig) (rendezvousPlan, error) {
	if !literalNodeEndpoint(input.ListenAddress) || !supportedCarrier(input.CarrierProfile) || input.NetworkID == [32]byte{} ||
		input.EpochDigest == [32]byte{} || input.NodeID == [32]byte{} || input.NodePublicKey == [32]byte{} ||
		input.Epoch == 0 || input.NotAfter.IsZero() || input.HandshakeLimit == 0 || input.WaitingLimit == 0 ||
		input.PairLimit == 0 || input.PairByteLimit == 0 || input.PairByteLimit > uint64(1<<63-1) ||
		!validAdmissionTimeout(input.AdmissionTimeout) || input.DrainTimeout <= 0 || input.DrainTimeout > time.Minute {
		return rendezvousPlan{}, errors.New("Rendezvous duty configuration is incomplete or outside its implementation bound")
	}
	now := func() time.Time { return time.Now().UTC() }
	if !now().Before(input.NotAfter.UTC()) {
		return rendezvousPlan{}, errors.New("Rendezvous duty has expired")
	}
	if !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return rendezvousPlan{}, errors.New("Rendezvous duty expiry must use whole UTC seconds")
	}
	if err := validateNodeCertificate(input.Certificate, input.NodePublicKey); err != nil {
		return rendezvousPlan{}, err
	}
	result := rendezvousPlan{rendezvousConfig: input, now: now, peersByNode: make(map[[32]byte]rendezvousPeer, len(input.Peers)),
		peersByPublic: make(map[[32]byte]rendezvousPeer, len(input.Peers))}
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

func (plan rendezvousPlan) peersByNodeForRole(role byte) rendezvousPeer {
	for _, peer := range plan.peersByNode {
		if peer.Role == role {
			return peer
		}
	}
	return rendezvousPeer{}
}

func literalNodeEndpoint(endpoint string) bool {
	host, port, err := net.SplitHostPort(endpoint)
	number, portErr := strconv.Atoi(port)
	return err == nil && net.ParseIP(host) != nil && portErr == nil && number >= 1 && number <= 65535
}

func supportedCarrier(profile route.CarrierProfile) bool {
	return profile == route.CarrierTCP || profile == route.CarrierQUIC
}

func validateNodeCertificate(certificate tls.Certificate, expected [32]byte) error {
	if certificate.PrivateKey == nil || len(certificate.Certificate) != 1 {
		return errors.New("node TLS certificate is invalid")
	}
	leaf := certificate.Leaf
	if leaf == nil {
		var err error
		leaf, err = x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			return errors.New("node TLS certificate leaf is invalid")
		}
	}
	public, ok := leaf.PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || string(public) != string(expected[:]) {
		return errors.New("node TLS certificate does not match Node public key")
	}
	return nil
}
