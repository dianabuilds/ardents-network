package node

import (
	"context"
	"crypto/ed25519"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRunServesStateAssignedRendezvousThenWithdraws(t *testing.T) {
	server, serverPublic := rendezvousCertificate(t, 21, "rendezvous")
	initiator, initiatorPublic := rendezvousCertificate(t, 22, "initiator")
	responder, responderPublic := rendezvousCertificate(t, 23, "responder")
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := dutyFacts{Generation: "generation-r", NetworkID: [32]byte{1}, Epoch: 4, Digest: [32]byte{2},
		EpochValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute), Profile: route.Profile, Fresh: true,
		RecordPresent: true, NodeID: [32]byte{3}, NodePublicKey: serverPublic, RecordValidFrom: now.Add(-time.Minute),
		RecordValidUntil: now.Add(time.Minute), DeclaredFamily: "rendezvous-family", ProbeEndpoint: reserveAddress(t), CarrierProfile: string(route.CarrierTCP),
		Assignment: "rendezvous", AssignmentDigest: [32]byte{4}, CandidateCount: 2, Candidates: [64]dutyCandidate{
			{NodeID: [32]byte{4}, PublicKey: initiatorPublic, Assignment: "initiator", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)},
			{NodeID: [32]byte{5}, PublicKey: responderPublic, Assignment: "responder", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Minute)},
		}}
	var lock sync.RWMutex
	events := make(chan Event, 16)
	config := Config{NetworkID: snapshot.NetworkID, NodeID: snapshot.NodeID, IdentityKey: server.PrivateKey.(ed25519.PrivateKey),
		Current: func() (DutyView, error) { lock.RLock(); defer lock.RUnlock(); return snapshot, nil },
		Rendezvous: RendezvousProfile{Certificate: server, HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1,
			PairByteLimit: 1 << 20, AdmissionTimeout: time.Second, DrainTimeout: time.Second}, PollInterval: 10 * time.Millisecond,
		Quarantine: time.Millisecond, LocalRoleStateRoot: localRoleStateRoot(t), CheckPlacement: func() error { return nil },
		Emit: func(_ context.Context, event Event) error { events <- event; return nil }}
	results := make(chan Result, 1)
	errors := make(chan error, 1)
	go func() { result, err := Run(context.Background(), config); results <- result; errors <- err }()
	ready := waitForStateEvent(t, events, "READY")
	if ready.CarrierProfile != string(route.CarrierTCP) {
		t.Fatalf("Rendezvous READY Carrier Profile = %q", ready.CarrierProfile)
	}
	attachment := [32]byte{9}
	first, err := openRendezvousLeg(t.Context(), snapshot.ProbeEndpoint, initiator, serverPublic,
		legFor(rendezvousMaterials{serverPublic: serverPublic}, attachment, route.InitiatorRole, snapshot.ValidUntil))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := openRendezvousLeg(t.Context(), snapshot.ProbeEndpoint, responder, serverPublic,
		legFor(rendezvousMaterials{serverPublic: serverPublic}, attachment, route.ResponderRole, snapshot.ValidUntil))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := first.Write([]byte("State-assigned rendezvous")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, second, len("State-assigned rendezvous")); string(got) != "State-assigned rendezvous" {
		t.Fatalf("Rendezvous bytes = %q", got)
	}
	lock.Lock()
	snapshot.Fresh = false
	lock.Unlock()
	draining := waitForStateEvent(t, events, "DRAINING")
	if draining.CarrierProfile != string(route.CarrierTCP) {
		t.Fatalf("Rendezvous DRAINING Carrier Profile = %q", draining.CarrierProfile)
	}
	select {
	case result := <-results:
		if result.State != "WITHDRAWN" || result.CarrierProfile != string(route.CarrierTCP) {
			t.Fatalf("Rendezvous terminal result = %+v", result)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("State withdrawal did not terminate Rendezvous")
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}
