package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestStateEntryAdmitterUsesSeparateLedgerRoot(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Add(time.Minute)
	var fixedPublic [32]byte
	copy(fixedPublic[:], public)
	candidate := dutyCandidate{NodeID: [32]byte{1}, PublicKey: fixedPublic, KeyID: [32]byte{2}, FamilyID: [32]byte{3},
		RecordDigest: [32]byte{4}, DomainProofDigest: [32]byte{5}, Endpoint: "127.0.0.1:34001", Capacity: 1, Assignment: "initiator",
		ValidFrom: now.Add(-time.Second), ValidUntil: until, AssignmentNotAfter: until}
	snapshot := dutyFacts{NetworkID: [32]byte{6}, Digest: [32]byte{7}, Epoch: 8, Profile: route.Profile, Fresh: true,
		Assignment: "initiator", ValidUntil: until, CandidateCount: 1, Candidates: [64]dutyCandidate{candidate}}
	admit, closeAdmitter, err := openStateEntryAdmitter(t.TempDir(), snapshot, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer closeAdmitter()
	raw, err := entry.Issue(entry.IssueInput{NetworkID: snapshot.NetworkID, Digest: snapshot.Digest, Epoch: snapshot.Epoch,
		Candidate: entry.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, KeyID: candidate.KeyID, FamilyID: candidate.FamilyID,
			RecordDigest: candidate.RecordDigest, DomainProofDigest: candidate.DomainProofDigest, Endpoint: candidate.Endpoint, Capacity: candidate.Capacity,
			Domain: candidate.Assignment, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil, AssignmentNotAfter: candidate.AssignmentNotAfter},
		NotBefore: now.Add(-time.Second), NotAfter: until, Slot: 0, Generation: 1}, private)
	if err != nil {
		t.Fatal(err)
	}
	got, err := admit(raw, [32]byte{9}, [32]byte{10}, until)
	if err != nil || got.InitiatorNodeID != candidate.NodeID || got.NotAfter != until {
		t.Fatalf("State Entry admission = %+v, %v", got, err)
	}
}
