//go:build linux

package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// State projection is a fixture; listener, Node authentication, framing,
// inner TLS handler, Stop and Drain are the real production path.
func TestClosedForwardingStopDrainsIdleAuthenticatedCarrier(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	certificate, key := rendezvousCertificate(t, 231, "forwarding-stop-server")
	peerCertificate, peerKey := rendezvousCertificate(t, 232, "forwarding-stop-peer")
	fixture.snapshot.ProbeEndpoint = reserveAddress(t)
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = key
	fixture.snapshot.Candidates[0].PublicKey = peerKey
	fixture.config.now = func() time.Time { return fixture.now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: filepath.Join(t.TempDir(), "spends"), Certificate: certificate, ConnectionLimit: 2, DrainTimeout: 2 * time.Second}
	if err := os.MkdirAll(fixture.config.ClosedForwarding.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	server, err := startClosedForwarding(fixture.config, fixture.snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Stop()
	deadline := time.Now().Add(5 * time.Second)
	carrier, err := route.OpenClosedNodeCarrier(t.Context(), route.ClosedNodeCarrierRequest{CarrierProfile: route.ClosedCarrierTCP,
		Endpoint: fixture.snapshot.ProbeEndpoint, Certificate: peerCertificate, ExpectedPeerKey: key, Deadline: deadline})
	if err != nil {
		t.Fatal(err)
	}
	defer carrier.Close()
	if err := carrier.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	receiver := fixture.receiver
	hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding,
		ChannelNonce: [32]byte{9}, Deadline: fixture.now.Add(9 * time.Second)}
	body, err := route.EncodeClosedHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(carrier, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if frame, err := route.ReadClosedLaneFrame(carrier); err != nil || frame.Kind != 5 {
		t.Fatalf("outer accept: %+v %v", frame, err)
	}
	body, err = route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
		Purpose: route.ClosedPurposeForwarding, Deadline: fixture.now.Add(8 * time.Second)}, route.ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(carrier, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	// A completed inner TLS handshake proves the production child handler has
	// started. The peer then leaves both outer and inner HELLO reads idle.
	inner, err := route.OpenClosedRoleTLS(t.Context(), &outerTestInnerConn{outer: carrier, lane: 1}, key, deadline)
	if err != nil {
		t.Fatal(err)
	}
	defer inner.Close()
	server.Stop()
	drain, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if err := server.Drain(drain); err != nil {
		t.Fatalf("stopped forwarding did not drain: %v", err)
	}
	if active, _, _ := server.Usage(); active != 0 {
		t.Fatalf("retained %d accepted carriers after drain", active)
	}
	select {
	case err := <-server.Done:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("accept loop not joined by drain")
	}
}
