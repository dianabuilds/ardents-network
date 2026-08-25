package node

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestResponderDutyUsesOneStateRendezvousPeer(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 61, "responder-duty")
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := dutyFacts{NetworkID: [32]byte{31}, Epoch: 32, Digest: [32]byte{33}, Profile: route.Profile, NodeID: [32]byte{34}, NodePublicKey: public,
		Assignment: "responder", ProbeEndpoint: "127.0.0.1:30261", EpochValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(30 * time.Second),
		Candidates: [64]dutyCandidate{{NodeID: [32]byte{35}, PublicKey: [32]byte{36}, Endpoint: "127.0.0.1:30262", Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute)}}, CandidateCount: 1}
	profile := ResponderProfile{Certificate: certificate,
		HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 1024, DrainTimeout: time.Second}
	admit := func([]byte, [32]byte, [32]byte, byte, [32]byte, time.Time) (route.EndpointTransitAdmission, error) {
		return route.EndpointTransitAdmission{}, nil
	}
	plan, err := responderDuty(profile, snapshot, admit)
	if err != nil || plan.Rendezvous.NodeID != snapshot.Candidates[0].NodeID || !plan.NotAfter.Equal(snapshot.RecordValidUntil) {
		t.Fatalf("Responder State duty = %+v, %v", plan, err)
	}
	snapshot.Candidates[1] = dutyCandidate{NodeID: [32]byte{37}, PublicKey: [32]byte{38}, Endpoint: "127.0.0.1:30263", Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute)}
	snapshot.CandidateCount = 2
	if _, err := responderDuty(profile, snapshot, admit); err == nil {
		t.Fatal("Responder accepted ambiguous Rendezvous peers")
	}
}
