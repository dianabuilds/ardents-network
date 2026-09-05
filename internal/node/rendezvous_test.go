package node

import (
	"context"
	"crypto/tls"
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

func TestRendezvousDrainPreservesActivePairInsideLease(t *testing.T) {
	running, material, config := rendezvousFixture(t)
	attachment := [32]byte{10}
	initiator, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.initiator, material.serverPublic,
		legFor(material, attachment, route.InitiatorRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	responder, err := openRendezvousLeg(t.Context(), config.ListenAddress, material.responder, material.serverPublic,
		legFor(material, attachment, route.ResponderRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	awaitUsage(t, running, time.Second, func(usage rendezvousUsage) bool { return usage.ActivePairs == 1 })
	result := make(chan error, 1)
	go func() { result <- running.Drain(t.Context()) }()
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-result:
		t.Fatalf("Rendezvous drain completed before active work: %v", err)
	default:
	}
	if connection, dialErr := net.DialTimeout("tcp", config.ListenAddress, 100*time.Millisecond); dialErr == nil {
		_ = connection.Close()
		t.Fatal("draining Rendezvous accepted a new connection")
	}
	if _, err := initiator.Write([]byte("inside lease")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, len("inside lease")); string(got) != "inside lease" {
		t.Fatalf("draining Rendezvous bytes = %q", got)
	}
	if err := initiator.Close(); err != nil {
		t.Fatal(err)
	}
	if err := responder.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Rendezvous did not finish after active work completed")
	}
}

func TestRendezvousPairsExactAuthenticatedLegsOverQUIC(t *testing.T) {
	running, material, config := rendezvousFixtureForCarrier(t, route.CarrierQUIC)
	attachment := [32]byte{11}
	initiator, err := openRendezvousCarrier(t.Context(), config, material.initiator, material.serverPublic,
		legFor(material, attachment, route.InitiatorRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := openRendezvousCarrier(t.Context(), config, material.responder, material.serverPublic,
		legFor(material, attachment, route.ResponderRole, config.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	if _, err := initiator.Write([]byte("quic product path")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, len("quic product path")); string(got) != "quic product path" {
		t.Fatalf("QUIC relayed bytes = %q", got)
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
		t.Fatal(err)
	}
	if usage := running.Usage(); usage.CompletedPairs != 1 || usage.Connections != 0 {
		t.Fatalf("QUIC Rendezvous terminal usage = %+v", usage)
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
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := responder.Close(); err != nil {
		t.Fatal(err)
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
	awaitUsage(t, running, 3*time.Second, func(usage rendezvousUsage) bool {
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
	awaitUsage(t, running, time.Second, func(usage rendezvousUsage) bool {
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
		awaitUsage(t, running, time.Second, func(usage rendezvousUsage) bool {
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
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		// Attempting the TLS handshake makes the rejected admission observable to
		// the client. RefusedBeforeTLS below proves the listener made that
		// decision before authenticating this leg.
		if err := submitRejectedLeg(ctx, config.ListenAddress, material.initiator, material.serverPublic,
			legFor(material, [32]byte{6}, route.InitiatorRole, config.NotAfter)); err == nil {
			t.Fatal("pair-capacity leg was accepted")
		}
		awaitUsage(t, running, time.Second, func(usage rendezvousUsage) bool {
			return usage.ActivePairs == 1 && usage.RefusedBeforeTLS == 1
		})
	})
}

func TestRendezvousRejectsIncompleteConfiguration(t *testing.T) {
	if running, err := startRendezvous(rendezvousConfig{}); err == nil || running != nil {
		t.Fatalf("incomplete configuration result = (%v, %v)", running, err)
	}
}

func TestRendezvousDutyUsesOnlyStateAssignedPeers(t *testing.T) {
	server, serverPublic := rendezvousCertificate(t, 10, "rendezvous")
	_, initiatorPublic := rendezvousCertificate(t, 11, "initiator")
	_, responderPublic := rendezvousCertificate(t, 12, "responder")
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := dutyFacts{NetworkID: [32]byte{1}, Epoch: 2, Digest: [32]byte{3}, Profile: route.Profile,
		Assignment: "rendezvous", NodeID: [32]byte{4}, NodePublicKey: serverPublic, ProbeEndpoint: "127.0.0.1:4400",
		EpochValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(2 * time.Minute),
		CandidateCount: 2, Candidates: [64]dutyCandidate{
			{NodeID: [32]byte{5}, PublicKey: initiatorPublic, Assignment: "initiator", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)},
			{NodeID: [32]byte{6}, PublicKey: responderPublic, Assignment: "responder", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)},
		}}
	config, err := rendezvousDuty(RendezvousProfile{Certificate: server, HandshakeLimit: 2, WaitingLimit: 2,
		PairLimit: 1, PairByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second}, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if config.ListenAddress != snapshot.ProbeEndpoint || !config.NotAfter.Equal(snapshot.ValidUntil) || len(config.Peers) != 2 ||
		config.Peers[0].Role != route.InitiatorRole || config.Peers[1].Role != route.ResponderRole {
		t.Fatalf("Rendezvous State duty = %+v", config)
	}
	snapshot.Candidates[1].Assignment = "other"
	if _, err := rendezvousDuty(RendezvousProfile{Certificate: server, HandshakeLimit: 2, WaitingLimit: 2,
		PairLimit: 1, PairByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second}, snapshot); err == nil {
		t.Fatal("incomplete State-assigned peers were accepted")
	}
}
