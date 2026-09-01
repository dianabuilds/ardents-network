package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type rendezvousMaterials struct {
	server, initiator, responder                   tls.Certificate
	serverPublic, initiatorPublic, responderPublic [32]byte
}

func rendezvousFixture(t *testing.T) (*Rendezvous, rendezvousMaterials, rendezvousConfig) {
	return rendezvousFixtureWith(t, 2, 2, 1, 1<<20, 5*time.Second)
}

func rendezvousFixtureForCarrier(t *testing.T, profile route.CarrierProfile) (*Rendezvous, rendezvousMaterials, rendezvousConfig) {
	return rendezvousFixtureForCarrierWith(t, profile, 2, 2, 1, 1<<20, 5*time.Second)
}

func rendezvousFixtureWith(t *testing.T, handshakes, waiting, pairs uint16, pairBytes uint64,
	lifetime time.Duration) (*Rendezvous, rendezvousMaterials, rendezvousConfig) {
	return rendezvousFixtureForCarrierWith(t, route.CarrierTCP, handshakes, waiting, pairs, pairBytes, lifetime)
}

func rendezvousFixtureForCarrierWith(t *testing.T, profile route.CarrierProfile, handshakes, waiting, pairs uint16,
	pairBytes uint64, lifetime time.Duration) (*Rendezvous, rendezvousMaterials, rendezvousConfig) {
	t.Helper()
	material := rendezvousMaterials{}
	material.server, material.serverPublic = rendezvousCertificate(t, 1, "server")
	material.initiator, material.initiatorPublic = rendezvousCertificate(t, 2, "initiator")
	material.responder, material.responderPublic = rendezvousCertificate(t, 3, "responder")
	endpoint := availableLoopbackEndpoint(t)
	if profile == route.CarrierQUIC {
		endpoint = availableLoopbackUDPEndpoint(t)
	}
	config := rendezvousConfig{ListenAddress: endpoint, CarrierProfile: profile, Certificate: material.server,
		NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3}, NodePublicKey: material.serverPublic,
		Epoch: 4, NotAfter: time.Now().UTC().Truncate(time.Second).Add(lifetime),
		Peers: []RendezvousPeer{{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Role: route.InitiatorRole},
			{NodeID: [32]byte{5}, PublicKey: material.responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: handshakes, WaitingLimit: waiting, PairLimit: pairs, PairByteLimit: pairBytes, AdmissionTimeout: time.Second, DrainTimeout: time.Second}
	running, err := startRendezvous(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = running.Close() })
	return running, material, config
}

func availableLoopbackUDPEndpoint(t *testing.T) string {
	t.Helper()
	connection, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := connection.LocalAddr().String()
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func availableLoopbackEndpoint(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func rendezvousCertificate(t *testing.T, serial int64, name string) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var identifier [32]byte
	copy(identifier[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, identifier
}

func legFor(material rendezvousMaterials, attachment [32]byte, role byte, deadline time.Time) route.LegBinding {
	nodeID := [32]byte{4}
	if role == route.ResponderRole {
		nodeID = [32]byte{5}
	}
	return route.LegBinding{NetworkID: [32]byte{1}, Epoch: 4, Digest: [32]byte{2}, AttachmentID: attachment,
		SenderRole: role, PeerRole: route.RendezvousRole, SenderNodeID: nodeID, PeerNodeID: [32]byte{3}, NotAfter: deadline}
}

func openRendezvousLeg(ctx context.Context, endpoint string, certificate tls.Certificate, server [32]byte,
	binding route.LegBinding) (*tls.Conn, error) {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, err
	}
	connection := tls.Client(raw, rendezvousClientTLS(certificate, server))
	if err := connection.SetDeadline(binding.NotAfter); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := connection.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := route.WriteNodeLegBinding(connection, binding); err != nil {
		_ = raw.Close()
		return nil, err
	}
	peer, err := route.ReadNodeLegBinding(connection)
	if err != nil || binding.VerifyReciprocal(peer) != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("Rendezvous reciprocal LegBinding is invalid: read=%v verify=%v local=%+v peer=%+v", err, binding.VerifyReciprocal(peer), binding, peer)
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return connection, nil
}

func openRendezvousCarrier(ctx context.Context, config rendezvousConfig, certificate tls.Certificate, server [32]byte,
	binding route.LegBinding) (route.Carrier, error) {
	return route.OpenNodeLeg(ctx, route.NodeLegRequest{CarrierProfile: config.CarrierProfile, Endpoint: config.ListenAddress,
		Certificate: certificate, ExpectedPeerKey: server, Binding: binding, Deadline: binding.NotAfter})
}

func submitRejectedLeg(ctx context.Context, endpoint string, certificate tls.Certificate, server [32]byte,
	binding route.LegBinding) error {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return err
	}
	defer raw.Close()
	connection := tls.Client(raw, rendezvousClientTLS(certificate, server))
	if err := connection.SetDeadline(binding.NotAfter); err != nil {
		return err
	}
	if err := connection.HandshakeContext(ctx); err != nil {
		return err
	}
	if err := route.WriteNodeLegBinding(connection, binding); err != nil {
		return err
	}
	_, err = route.ReadNodeLegBinding(connection)
	return err
}

func rendezvousClientTLS(certificate tls.Certificate, server [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, InsecureSkipVerify: true, SessionTicketsDisabled: true,
		NextProtos: []string{route.Profile}, VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return errors.New("missing server certificate")
			}
			public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
			if !ok || string(public) != string(server[:]) {
				return errors.New("wrong server certificate")
			}
			return nil
		}}
}

func awaitUsage(t *testing.T, running *Rendezvous, timeout time.Duration, accepted func(RendezvousUsage) bool) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		usage := running.Usage()
		if accepted(usage) {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("Rendezvous did not reach expected state: %+v", usage)
		case <-tick.C:
		}
	}
}

func readExact(t *testing.T, reader io.Reader, length int) []byte {
	t.Helper()
	result := make([]byte, length)
	if _, err := io.ReadFull(reader, result); err != nil {
		t.Fatal(err)
	}
	return result
}
