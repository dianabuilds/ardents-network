package entry

import (
	"crypto/ed25519"
	"testing"
	"time"
)

// GAP-5-T1 (positive): MinimumReservation=1s with AssignmentNotAfter=now+30s → Accepted.
func TestGAP5VerifyAcceptsWhenAssignmentNotAfterExceedsMinimumReservation(t *testing.T) {
	fixture := newEntryFixture(t)
	verification := fixture.verification()
	verification.MinimumReservation = time.Second
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	_, _, class, err := Verify(raw, verification)
	if err != nil || class != Accepted {
		t.Fatalf("Verify = %q, %v", class, err)
	}
}

// GAP-5-T2 (negative — insufficient): MinimumReservation=1h with AssignmentNotAfter=now+30s → Insufficient.
func TestGAP5VerifyRejectsWhenAssignmentNotAfterBeforeMinimumReservation(t *testing.T) {
	fixture := newEntryFixture(t)
	verification := fixture.verification()
	verification.MinimumReservation = time.Hour
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	_, _, class, err := Verify(raw, verification)
	if err != nil || class != Insufficient {
		t.Fatalf("Verify = %q, %v", class, err)
	}
}

// GAP-5-T3 (backward-compat): MinimumReservation=0 with AssignmentNotAfter=now+1s → Accepted.
func TestGAP5VerifyAcceptsWhenMinimumReservationIsZero(t *testing.T) {
	fixture := newEntryFixture(t)
	candidate := fixture.candidates[0]
	candidate.AssignmentNotAfter = fixture.now.Add(time.Second)
	fixture.view.Candidates[0] = candidate
	verification := fixture.verification()
	verification.MinimumReservation = 0
	raw := fixture.inviteWithWindow(t, candidate, 0, 1, nil, fixture.now.Add(-time.Second), fixture.now.Add(time.Second))
	_, _, class, err := Verify(raw, verification)
	if err != nil || class != Accepted {
		t.Fatalf("Verify = %q, %v", class, err)
	}
}

// GAP-5-T4 (boundary): MinimumReservation=30s with AssignmentNotAfter=now+30s → Insufficient (strict <).
func TestGAP5VerifyRejectsAtBoundaryOfMinimumReservation(t *testing.T) {
	fixture := newEntryFixture(t)
	candidate := fixture.candidates[0]
	candidate.AssignmentNotAfter = fixture.now.Add(30 * time.Second)
	fixture.view.Candidates[0] = candidate
	verification := fixture.verification()
	verification.MinimumReservation = 30 * time.Second
	raw := fixture.inviteWithWindow(t, candidate, 0, 1, nil, fixture.now.Add(-time.Second), fixture.now.Add(20*time.Second))
	_, _, class, err := Verify(raw, verification)
	if err != nil || class != Insufficient {
		t.Fatalf("Verify = %q, %v", class, err)
	}
}

// inviteWithWindow builds an Invite whose notAfter is bounded by the candidate's
// AssignmentNotAfter so the existing time-window check still passes.
func (fixture entryFixture) inviteWithWindow(t *testing.T, candidate Candidate, slot, generation byte, replaces *[32]byte, notBefore, notAfter time.Time) []byte {
	t.Helper()
	body := make([]byte, 0, 256)
	body = appendUint16(body, inviteWireVersion)
	body = append(body, fixture.view.NetworkID[:]...)
	body = appendUint64(body, fixture.view.Epoch)
	body = append(body, fixture.view.Digest[:]...)
	body = append(body, byte(len(profileID)))
	body = append(body, profileID...)
	body = append(body, fixture.recipient[:]...)
	body = append(body, candidate.KeyID[:]...)
	body = append(body, candidate.NodeID[:]...)
	body = append(body, candidate.FamilyID[:]...)
	body = append(body, candidate.RecordDigest[:]...)
	body = append(body, candidate.DomainProofDigest[:]...)
	body = appendUint64(body, uint64(candidate.AssignmentNotAfter.Unix()))
	body = appendUint64(body, uint64(notBefore.Unix()))
	body = appendUint64(body, uint64(notAfter.Unix()))
	body = append(body, generation, slot)
	if replaces == nil {
		body = append(body, 0)
	} else {
		body = append(body, 1)
		body = append(body, replaces[:]...)
	}
	signature := ed25519.Sign(fixture.private[candidate.KeyID], signatureInput(body))
	raw := make([]byte, 0, len(inviteMagic)+2+len(body)+len(signature))
	raw = append(raw, inviteMagic...)
	raw = appendUint16(raw, uint16(len(body)))
	raw = append(raw, body...)
	return append(raw, signature...)
}
