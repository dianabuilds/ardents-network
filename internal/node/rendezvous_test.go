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
	running, material, config := rendezvousFixture(t)
	deadline := config.NotAfter
	attachment := [32]byte{9}
	initiator, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
		legFor(material, attachment, route.InitiatorRole, deadline))
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.responder, material.serverPublic,
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
	running, material, config := rendezvousFixture(t)
	first, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
		legFor(material, [32]byte{8}, route.InitiatorRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	duplicate, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
		legFor(material, [32]byte{8}, route.InitiatorRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer duplicate.Close()
	responder, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.responder, material.serverPublic,
		legFor(material, [32]byte{8}, route.ResponderRole, config.NotAfter))
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

func TestRendezvousRejectsUnauthorizedOrMismatchedBoundIdentity(t *testing.T) {
	cases := []struct {
		name     string
		role     byte
		identity func(rendezvousMaterials) tls.Certificate
	}{
		{name: "unauthorized", role: route.InitiatorRole, identity: func(material rendezvousMaterials) tls.Certificate {
			certificate, _ := rendezvousCertificate(t, 4, "unrecognized")
			return certificate
		}},
		{name: "binding identity mismatch", role: route.ResponderRole, identity: func(material rendezvousMaterials) tls.Certificate {
			return material.initiator
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			running, material, config := rendezvousFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			err := submitRejectedLeg(ctx, config.ListenAddress, test.identity(material), material.serverPublic,
				legFor(material, [32]byte{7}, test.role, config.NotAfter))
			if err == nil {
				t.Fatal("unauthorized Rendezvous leg was accepted")
			}
			if usage := running.Usage(); usage.WaitingLegs != 0 || usage.ActivePairs != 0 {
				t.Fatalf("unauthorized leg changed Rendezvous state: %+v", usage)
			}
		})
	}
}

func TestRendezvousExpiresUnpairedLeg(t *testing.T) {
	running, material, config := rendezvousFixtureWith(t, 2, 1, 1, 1<<20, 2*time.Second)
	leg, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
		legFor(material, [32]byte{6}, route.InitiatorRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer leg.Close()
	awaitUsage(t, running, 3*time.Second, func(usage RendezvousUsage) bool {
		return usage.Expired == 1 && usage.WaitingLegs == 0 && usage.Connections == 0
	})
}

func TestRendezvousBoundsEachPairedDirection(t *testing.T) {
	running, material, config := rendezvousFixtureWith(t, 2, 2, 1, 4, 5*time.Second)
	attachment := [32]byte{5}
	initiator, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
		legFor(material, attachment, route.InitiatorRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.responder, material.serverPublic,
		legFor(material, attachment, route.ResponderRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	if _, err := initiator.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, 4); string(got) != "1234" {
		t.Fatalf("bounded relay bytes = %q", got)
	}
	awaitUsage(t, running, time.Second, func(usage RendezvousUsage) bool {
		return usage.ActivePairs == 0 && usage.RelayedBytes == 4
	})
}

func TestRendezvousReservesHandshakeWaitingAndPairSlots(t *testing.T) {
	t.Run("handshake", func(t *testing.T) {
		running, _, config := rendezvousFixtureWith(t, 1, 1, 1, 1<<20, 5*time.Second)
		first, err := net.Dial("tcp", config.ListenAddress)
		if err != nil {
			t.Fatal(err)
		}
		defer first.Close()
		second, err := net.Dial("tcp", config.ListenAddress)
		if err != nil {
			t.Fatal(err)
		}
		defer second.Close()
		awaitUsage(t, running, time.Second, func(usage RendezvousUsage) bool {
			return usage.Handshakes == 1 && usage.RefusedBeforeTLS == 1
		})
	})
	t.Run("waiting", func(t *testing.T) {
		running, material, config := rendezvousFixtureWith(t, 2, 1, 1, 1<<20, 5*time.Second)
		first, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
			legFor(material, [32]byte{3}, route.InitiatorRole, config.NotAfter))
		if err != nil {
			t.Fatal(err)
		}
		defer first.Close()
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := submitRejectedLeg(ctx, config.ListenAddress, material.responder, material.serverPublic,
			legFor(material, [32]byte{4}, route.ResponderRole, config.NotAfter)); err == nil {
			t.Fatal("waiting-capacity leg was accepted")
		}
		if usage := running.Usage(); usage.WaitingRefused != 1 || usage.WaitingLegs != 1 {
			t.Fatalf("waiting reservation outcome = %+v", usage)
		}
	})
	t.Run("pair", func(t *testing.T) {
		running, material, config := rendezvousFixtureWith(t, 2, 2, 1, 1<<20, 5*time.Second)
		attachment := [32]byte{2}
		first, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
			legFor(material, attachment, route.InitiatorRole, config.NotAfter))
		if err != nil {
			t.Fatal(err)
		}
		defer first.Close()
		second, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.responder, material.serverPublic,
			legFor(material, attachment, route.ResponderRole, config.NotAfter))
		if err != nil {
			t.Fatal(err)
		}
		defer second.Close()
		extra, err := net.Dial("tcp", config.ListenAddress)
		if err != nil {
			t.Fatal(err)
		}
		defer extra.Close()
		awaitUsage(t, running, time.Second, func(usage RendezvousUsage) bool {
			return usage.ActivePairs == 1 && usage.RefusedBeforeTLS == 1
		})
	})
}

func TestRendezvousRejectsIncompleteConfiguration(t *testing.T) {
	if running, err := StartRendezvous(RendezvousConfig{}); err == nil || running != nil {
		t.Fatalf("incomplete configuration result = (%v, %v)", running, err)
	}
}

type rendezvousMaterials struct {
	server, initiator, responder                   tls.Certificate
	serverPublic, initiatorPublic, responderPublic [32]byte
}

func rendezvousFixture(t *testing.T) (*Rendezvous, rendezvousMaterials, RendezvousConfig) {
	return rendezvousFixtureWith(t, 2, 2, 1, 1<<20, 5*time.Second)
}

func rendezvousFixtureWith(t *testing.T, handshakes, waiting, pairs uint16, pairBytes uint64,
	lifetime time.Duration) (*Rendezvous, rendezvousMaterials, RendezvousConfig) {
	t.Helper()
	material := rendezvousMaterials{}
	material.server, material.serverPublic = rendezvousCertificate(t, 1, "server")
	material.initiator, material.initiatorPublic = rendezvousCertificate(t, 2, "initiator")
	material.responder, material.responderPublic = rendezvousCertificate(t, 3, "responder")
	config := RendezvousConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: material.server,
		NetworkID: [32]byte{1}, EpochDigest: [32]byte{2}, NodeID: [32]byte{3}, NodePublicKey: material.serverPublic,
		Epoch: 4, NotAfter: time.Now().UTC().Truncate(time.Second).Add(lifetime),
		Peers: []RendezvousPeer{{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Role: route.InitiatorRole},
			{NodeID: [32]byte{5}, PublicKey: material.responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: handshakes, WaitingLimit: waiting, PairLimit: pairs, PairByteLimit: pairBytes, DrainTimeout: time.Second}
	running, err := StartRendezvous(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = running.Close() })
	return running, material, config
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

func submitRejectedLeg(ctx context.Context, endpoint string, certificate tls.Certificate, server [32]byte,
	binding route.LegBinding) error {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return err
	}
	defer raw.Close()
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
