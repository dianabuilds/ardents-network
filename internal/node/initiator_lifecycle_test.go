package node

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestRunKeepsInitiatorReadyWhileItsEntryLedgerIsHeld(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 141, "lifecycle-initiator")
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Add(time.Minute)
	snapshot := dutyFacts{Generation: "initiator-live", NetworkID: [32]byte{1}, Epoch: 2, Digest: [32]byte{3}, Profile: route.Profile,
		Fresh: true, RecordPresent: true, NodeID: [32]byte{4}, NodePublicKey: public, EpochValidFrom: now.Add(-time.Second),
		RecordValidFrom: now.Add(-time.Second), RecordValidUntil: until, ValidUntil: until, ProbeEndpoint: availableLoopbackEndpoint(t),
		Assignment: "initiator", AssignmentDigest: [32]byte{5}, CandidateCount: 3, Candidates: [64]dutyCandidate{
			{NodeID: [32]byte{4}, PublicKey: public, KeyID: [32]byte{6}, FamilyID: [32]byte{7}, RecordDigest: [32]byte{8}, DomainProofDigest: [32]byte{9}, Endpoint: "127.0.0.1:34104", Capacity: 1, Assignment: "initiator", ValidFrom: now.Add(-time.Second), ValidUntil: until, AssignmentNotAfter: until},
			{NodeID: [32]byte{10}, PublicKey: [32]byte{11}, Endpoint: "127.0.0.1:34110", Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: until},
			{NodeID: [32]byte{12}, PublicKey: [32]byte{13}, Endpoint: "127.0.0.1:34112", Assignment: "destination-resolution", ValidFrom: now.Add(-time.Second), ValidUntil: until},
		}}
	events := make(chan Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan Result, 1)
	errors := make(chan error, 1)
	go func() {
		value, err := Run(ctx, Config{NetworkID: snapshot.NetworkID, NodeID: snapshot.NodeID, IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey),
			Current: func() (DutyView, error) { return snapshot, nil }, Initiator: InitiatorProfile{Certificate: certificate, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 1024, DrainTimeout: time.Second},
			PollInterval: 10 * time.Millisecond, Quarantine: time.Millisecond, LocalRoleStateRoot: t.TempDir(), CheckPlacement: func() error { return nil },
			Emit: func(_ context.Context, event Event) error { events <- event; return nil }})
		result <- value
		errors <- err
	}()
	waitForState(t, events, "READY")
	time.Sleep(40 * time.Millisecond)
	select {
	case terminal := <-result:
		t.Fatalf("Initiator withdrew while Entry ledger was held: %+v", terminal)
	default:
	}
	cancel()
	select {
	case terminal := <-result:
		if terminal.State != "WITHDRAWN" {
			t.Fatalf("Initiator terminal result = %+v", terminal)
		}
	case <-time.After(time.Second):
		t.Fatal("Initiator did not withdraw")
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}
