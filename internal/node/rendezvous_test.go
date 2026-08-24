package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRendezvousPairsExactAuthenticatedLegsAndDrains(t *testing.T) {
	running, material := rendezvousFixture(t)
	deadline := running.plan.NotAfter
	attachment := [32]byte{9}
	initiator, err := openRendezvousLeg(t.Context(), running.Address(), material.initiator, material.serverPublic,
		legFor(material, attachment, route.InitiatorRole, deadline))
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := openRendezvousLeg(t.Context(), running.Address(), material.responder, material.serverPublic,
		legFor(material, attachment, route.ResponderRole, deadline))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()

	if _, err := initiator.Write([]byte("from initiator")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, len("from initiator")); string(got) != "from initiator" {
		t.Fatalf("responder bytes = %q", got)
	}
	if _, err := responder.Write([]byte("from responder")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, initiator, len("from responder")); string(got) != "from responder" {
		t.Fatalf("initiator bytes = %q", got)
	}
	if err := initiator.Close(); err != nil {
		t.Fatal(err)
	}
	if err := responder.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := running.Drain(ctx); err != nil {
		t.Fatalf("drain Rendezvous: %v", err)
	}
	usage := running.Usage()
	if usage.Handshakes != 0 || usage.WaitingLegs != 0 || usage.ActivePairs != 0 || usage.Connections != 0 || usage.CompletedPairs != 1 {
		t.Fatalf("Rendezvous terminal usage = %+v", usage)
	}
}

func TestRendezvousRejectsDuplicateSideWithoutDisplacingWaitingLeg(t *testing.T) {
	running, material := rendezvousFixture(t)
	first, err := openRendezvousLeg(t.Context(), running.Address(), material.initiator, material.serverPublic,
		legFor(material, [32]byte{8}, route.InitiatorRole, running.plan.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	duplicate, err := openRendezvousLeg(t.Context(), running.Address(), material.initiator, material.serverPublic,
		legFor(material, [32]byte{8}, route.InitiatorRole, running.plan.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer duplicate.Close()
	responder, err := openRendezvousLeg(t.Context(), running.Address(), material.responder, material.serverPublic,
		legFor(material, [32]byte{8}, route.ResponderRole, running.plan.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	if _, err := first.Write([]byte("retained")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, len("retained")); string(got) != "retained" {
		t.Fatalf("retained leg bytes = %q", got)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := running.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if usage := running.Usage(); usage.DuplicateSideRejected != 1 || usage.Connections != 0 {
		t.Fatalf("duplicate-side outcome = %+v", usage)
	}
}

type rendezvousMaterials struct {
	server, initiator, responder                   tls.Certificate
	serverPublic, initiatorPublic, responderPublic [32]byte
}

func rendezvousFixture(t *testing.T) (*Rendezvous, rendezvousMaterials) {
	t.Helper()
	material := rendezvousMaterials{}
	material.server, material.serverPublic = rendezvousCertificate(t, 1, "server")
	material.initiator, material.initiatorPublic = rendezvousCertificate(t, 2, "initiator")
	material.responder, material.responderPublic = rendezvousCertificate(t, 3, "responder")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := newRendezvousPlan(RendezvousConfig{ListenAddress: listener.Addr().String(), Certificate: material.server,
		NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3}, NodePublicKey: material.serverPublic,
		Epoch: 4, NotAfter: time.Now().UTC().Truncate(time.Second).Add(5 * time.Second),
		Peers: []RendezvousPeer{{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Role: route.InitiatorRole},
			{NodeID: [32]byte{5}, PublicKey: material.responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, DrainTimeout: time.Second})
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	running := startRendezvous(plan, listener)
	t.Cleanup(func() { _ = running.Close() })
	return running, material
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
	connection := tls.Client(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
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
		}})
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
		return nil, errors.New("Rendezvous reciprocal LegBinding is invalid")
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return connection, nil
}

func readExact(t *testing.T, reader io.Reader, length int) []byte {
	t.Helper()
	result := make([]byte, length)
	if _, err := io.ReadFull(reader, result); err != nil {
		t.Fatal(err)
	}
	return result
}
