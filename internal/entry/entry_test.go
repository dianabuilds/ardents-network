package entry

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// entryRoot creates the owner-only directory for an Entry state root,
// independent of the test process umask.
func entryRoot(t *testing.T) string {
	return entryRootForRecipient(t, testEntryRecipientSeed())
}

func entryRootForRecipient(t *testing.T, seed []byte) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "entry-state")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rootMarkerName), []byte(rootMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, recipientName), seed, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func testEntryRecipientSeed() []byte {
	return bytes.Repeat([]byte{91}, ed25519.SeedSize)
}

func TestImportAndReopenRetainOnlyCurrentStateCandidate(t *testing.T) {
	fixture := newEntryFixture(t)
	owner, err := Open(fixture.config(entryRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	invite := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	result, err := owner.Import(invite)
	if err != nil || result.Class != Accepted {
		t.Fatalf("import = %+v, %v", result, err)
	}
	index, found := owner.state.active(0)
	if !found || owner.state.Records[index].Identity != fixture.candidates[0].NodeID {
		t.Fatalf("imported active record = %+v", owner.state.Records)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(owner.root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	index, found = reopened.state.active(0)
	if !found || reopened.state.Records[index].Identity != fixture.candidates[0].NodeID ||
		reopened.state.Records[index].Status != memberActive {
		t.Fatalf("reopened active record = %+v", reopened.state.Records)
	}
}

func TestImportRejectsInviteWithWrongSignatureOrSurplusBytes(t *testing.T) {
	fixture := newEntryFixture(t)
	owner, err := Open(fixture.config(entryRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	invite := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	invite[len(invite)-1] ^= 1
	result, err := owner.Import(invite)
	if err != nil || result.Class != Invalid {
		t.Fatalf("mutated signature import = %+v, %v", result, err)
	}
	invite = append(fixture.invite(t, fixture.candidates[0], 0, 1, nil), 0)
	result, err = owner.Import(invite)
	if err != nil || result.Class != Invalid {
		t.Fatalf("surplus import = %+v, %v", result, err)
	}
	if _, found := owner.state.active(0); found {
		t.Fatal("rejected Invite became an active Entry record")
	}
}

func TestImportRejectsInviteV1IdentityAndBody(t *testing.T) {
	fixture := newEntryFixture(t)
	owner, err := Open(fixture.config(entryRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	invite := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	legacyIdentity := append([]byte(nil), invite...)
	copy(legacyIdentity, "ardents-entry-invite-v1")
	legacyBody := append([]byte(nil), invite...)
	legacyBody[len(inviteMagic)+2] = 0
	legacyBody[len(inviteMagic)+3] = 1
	for name, raw := range map[string][]byte{"identity": legacyIdentity, "body": legacyBody} {
		t.Run(name, func(t *testing.T) {
			result, importErr := owner.Import(raw)
			if importErr != nil || result.Class == Accepted {
				t.Fatalf("v1 Invite import = %+v, %v", result, importErr)
			}
		})
	}
}

func TestVerifyReturnsOnlyCurrentInitiatorAuthorization(t *testing.T) {
	fixture := newEntryFixture(t)
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	authorization, candidate, class, err := Verify(raw, fixture.verification())
	if err != nil || class != Accepted {
		t.Fatalf("Verify = %+v, %+v, %q, %v", authorization, candidate, class, err)
	}
	if authorization.InviteID == [32]byte{} || authorization.NetworkID != fixture.view.NetworkID ||
		authorization.Digest != fixture.view.Digest || authorization.Epoch != fixture.view.Epoch ||
		authorization.InitiatorNodeID != fixture.candidates[0].NodeID || authorization.RecipientPublicKey != fixture.recipient || !authorization.NotAfter.After(fixture.now) ||
		candidate != fixture.candidates[0] {
		t.Fatalf("unexpected authorization = %+v, candidate = %+v", authorization, candidate)
	}
	mutated := append([]byte(nil), raw...)
	mutated[len(mutated)-1] ^= 1
	if authorization, _, class, err := Verify(mutated, fixture.verification()); err != nil || class != Invalid || authorization != (Authorization{}) {
		t.Fatalf("mutated Verify = %+v, %q, %v", authorization, class, err)
	}
}

// TestVerifyReturnsConflictingRoleWhenConflictCallbackReturnsTrue exercises
// the entry.Verify → Verification.Conflict → ConflictingRole path end-to-end.
// The Conflict callback is a stub that returns (true, nil) to simulate a
// state-level conflict (e.g., a direct-source exposure). The real conflict
// detection logic is tested separately in
// internal/network/duty/source_collision_chain_test.go.
func TestVerifyReturnsConflictingRoleWhenConflictCallbackReturnsTrue(t *testing.T) {
	fixture := newEntryFixture(t)
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	verification := Verification{
		Current:       func() (View, error) { return fixture.view, nil },
		Conflict:      func([32]byte, [32]byte) (bool, error) { return true, nil },
		Clock:         func() time.Time { return fixture.now },
		TimeConfident: func() bool { return true },
	}
	_, _, class, err := Verify(raw, verification)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if class != ConflictingRole {
		t.Fatalf("class = %q, want %q", class, ConflictingRole)
	}
}

func TestReplacementImmediatelyRetiresInactiveGenerationOne(t *testing.T) {
	fixture := newEntryFixture(t)
	owner, err := Open(fixture.config(entryRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	first := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	firstResult, err := owner.Import(first)
	if err != nil || firstResult.Class != Accepted {
		t.Fatalf("first import = %+v, %v", firstResult, err)
	}
	second := fixture.invite(t, fixture.candidates[1], 0, 2, &firstResult.InviteID)
	secondResult, err := owner.Import(second)
	if err != nil || secondResult.Class != Accepted {
		t.Fatalf("replacement import = %+v, %v", secondResult, err)
	}
	if got := len(owner.state.Records); got != 2 || owner.state.Records[0].Status != memberRetired || owner.state.Records[1].Status != memberActive {
		t.Fatalf("replacement durable state = %+v", owner.state.Records)
	}
}

func TestOpenTerminalizesInterruptedAttemptAndSettlesReplacement(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	first, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil))
	if err != nil || first.Class != Accepted {
		t.Fatalf("first import = %+v, %v", first, err)
	}
	// Write one legacy non-terminal attachment journal directly: ADR-0095
	// retired the journal writers while the durable schema, the draining
	// replacement path, and the Open recovery all remain.
	attemptID := [32]byte{96}
	next := owner.state.clone()
	next.Attempt = &attemptRecord{ID: attemptID, Started: fixture.now.UnixNano(),
		Deadline: fixture.now.Add(5 * time.Second).UnixNano()}
	next.Contacts = append(next.Contacts, contactRecord{AttemptID: attemptID, InviteID: first.InviteID,
		Slot: 0, Ordinal: 0, Started: fixture.now.UnixNano()})
	if err := owner.commit(next, false); err != nil {
		t.Fatal(err)
	}
	if result, err := owner.Import(fixture.invite(t, fixture.candidates[1], 0, 2, &first.InviteID)); err != nil || result.Class != Accepted {
		t.Fatalf("replacement import = %+v, %v", result, err)
	}
	if owner.state.Records[0].Status != memberDraining || owner.state.Records[1].Status != memberVerified {
		t.Fatalf("replacement activated during the legacy attempt: %+v", owner.state.Records)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.state.Attempt == nil || reopened.state.Attempt.Terminal != "entry-interrupted" {
		t.Fatalf("reopened attempt = %+v", reopened.state.Attempt)
	}
	if len(reopened.state.Records) != 2 || reopened.state.Records[0].Status != memberRetired ||
		reopened.state.Records[1].Status != memberActive || reopened.state.Records[1].Identity != fixture.candidates[1].NodeID {
		t.Fatalf("interrupted replacement settlement = %+v", reopened.state.Records)
	}
}

type entryFixture struct {
	now        time.Time
	view       View
	candidates []Candidate
	private    map[[32]byte]ed25519.PrivateKey
	recipient  [32]byte
}

func newEntryFixture(t *testing.T) entryFixture {
	t.Helper()
	now := time.Unix(1_750_000_000, 0).UTC()
	recipient := ed25519.NewKeyFromSeed(testEntryRecipientSeed()).Public().(ed25519.PublicKey)
	fixture := entryFixture{now: now, private: map[[32]byte]ed25519.PrivateKey{}}
	copy(fixture.recipient[:], recipient)
	fixture.view = View{NetworkID: [32]byte{1}, Epoch: 7, Digest: [32]byte{2}, Profile: profileID, Fresh: true}
	for index := range 2 {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		candidate := Candidate{NodeID: [32]byte{byte(index + 11)}, KeyID: [32]byte{byte(index + 21)}, FamilyID: [32]byte{byte(index + 31)},
			RecordDigest: [32]byte{byte(index + 41)}, DomainProofDigest: [32]byte{byte(index + 51)}, Endpoint: "127.0.0.1:8" + string(rune('0'+index)),
			Capacity: 1, Domain: "initiator", ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), AssignmentNotAfter: now.Add(time.Hour)}
		copy(candidate.PublicKey[:], public)
		fixture.candidates = append(fixture.candidates, candidate)
		fixture.view.Candidates = append(fixture.view.Candidates, candidate)
		fixture.private[candidate.KeyID] = private
	}
	return fixture
}

func newLiveEntryFixture(t *testing.T) entryFixture {
	t.Helper()
	fixture := newEntryFixture(t)
	fixture.now = time.Now().UTC()
	for index := range fixture.candidates {
		fixture.candidates[index].ValidFrom = fixture.now.Add(-time.Minute)
		fixture.candidates[index].ValidUntil = fixture.now.Add(time.Minute)
		fixture.candidates[index].AssignmentNotAfter = fixture.now.Add(time.Minute)
		fixture.view.Candidates[index] = fixture.candidates[index]
	}
	return fixture
}

func (fixture entryFixture) config(root string) Config {
	return Config{Root: root, Current: func() (View, error) { return fixture.view, nil },
		Conflict: func([32]byte, [32]byte) (bool, error) { return false, nil }, Clock: func() time.Time { return fixture.now }, TimeConfident: func() bool { return true }}
}

func (fixture entryFixture) verification() Verification {
	return Verification{Current: func() (View, error) { return fixture.view, nil },
		Conflict: func([32]byte, [32]byte) (bool, error) { return false, nil }, Clock: func() time.Time { return fixture.now }, TimeConfident: func() bool { return true }}
}

func (fixture entryFixture) invite(t *testing.T, candidate Candidate, slot, generation byte, replaces *[32]byte) []byte {
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
	body = appendUint64(body, uint64(fixture.now.Add(-time.Second).Unix()))
	body = appendUint64(body, uint64(fixture.now.Add(time.Minute).Unix()))
	body = append(body, generation, slot)
	if replaces == nil {
		body = append(body, 0)
	} else {
		body = append(body, 1)
		body = append(body, replaces[:]...)
	}
	signature := ed25519.Sign(fixture.private[candidate.KeyID], signatureInput(body))
	invite := make([]byte, 0, len(inviteMagic)+2+len(body)+len(signature))
	invite = append(invite, inviteMagic...)
	invite = append(invite, byte(len(body)>>8), byte(len(body)))
	invite = append(invite, body...)
	return append(invite, signature...)
}

func TestIssueProducesAStateReferencedInvite(t *testing.T) {
	fixture := newEntryFixture(t)
	candidate := fixture.candidates[0]
	raw, err := Issue(IssueInput{NetworkID: fixture.view.NetworkID, Digest: fixture.view.Digest, RecipientPublicKey: fixture.recipient, Epoch: fixture.view.Epoch,
		Candidate: candidate, NotBefore: fixture.now.Add(-time.Second), NotAfter: fixture.now.Add(time.Second), Slot: 0, Generation: 1},
		fixture.private[candidate.KeyID])
	if err != nil {
		t.Fatal(err)
	}
	authorization, selected, class, err := Verify(raw, fixture.verification())
	if err != nil || class != Accepted || authorization.InitiatorNodeID != candidate.NodeID || selected.KeyID != candidate.KeyID {
		t.Fatalf("issued Invite verification = %+v %+v %s %v", authorization, selected, class, err)
	}
}

func appendUint16(destination []byte, value uint16) []byte {
	return append(destination, byte(value>>8), byte(value))
}

func appendUint64(destination []byte, value uint64) []byte {
	for shift := uint(56); ; shift -= 8 {
		destination = append(destination, byte(value>>shift))
		if shift == 0 {
			return destination
		}
	}
}
