package entry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"
)

func TestImportContactAndReopenUsesOnlyCurrentStateCandidate(t *testing.T) {
	fixture := newEntryFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	invite := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	result, err := owner.Import(invite)
	if err != nil || result.Class != Accepted {
		t.Fatalf("import = %+v, %v", result, err)
	}
	contact, err := owner.Contact()
	if err != nil || contact.Endpoint != fixture.candidates[0].Endpoint || contact.PublicKey != fixture.candidates[0].PublicKey {
		t.Fatalf("contact = %+v, %v", contact, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(owner.root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	contact, err = reopened.Contact()
	if err != nil || contact.NodeID != fixture.candidates[0].NodeID {
		t.Fatalf("reopened contact = %+v, %v", contact, err)
	}
}

func TestImportRejectsInviteWithWrongSignatureOrSurplusBytes(t *testing.T) {
	fixture := newEntryFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
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
	if _, err := owner.Contact(); err == nil {
		t.Fatal("rejected Invite became an Entry contact")
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
		authorization.InitiatorNodeID != fixture.candidates[0].NodeID || !authorization.NotAfter.After(fixture.now) ||
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
	owner, err := Open(fixture.config(t.TempDir()))
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
	contact, err := owner.Contact()
	if err != nil || contact.NodeID != fixture.candidates[1].NodeID {
		t.Fatalf("replacement contact = %+v, %v", contact, err)
	}
	if got := len(owner.state.Records); got != 2 || owner.state.Records[0].Status != memberRetired || owner.state.Records[1].Status != memberActive {
		t.Fatalf("replacement durable state = %+v", owner.state.Records)
	}
}

func TestAcquireRetriesOneCleanFailureAndRecordsTerminalCleanup(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if result, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil || result.Class != Accepted {
		t.Fatalf("import = %+v, %v", result, err)
	}
	starts := 0
	var peer net.Conn
	connection, cleanup, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{99}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			starts++
			if starts == 1 {
				return nil, nil, true, errors.New("injected contact failure")
			}
			client, server := net.Pipe()
			peer = server
			return client, client.Close, true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if starts != 2 || connection == nil {
		t.Fatalf("starts=%d connection=%v", starts, connection)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "opened" || len(owner.state.Contacts) != 2 ||
		owner.state.Contacts[0].Outcome != "failed" || owner.state.Contacts[1].Outcome != "opened" || !owner.state.Contacts[1].Cleanup {
		t.Fatalf("durable attempt = %+v contacts = %+v", owner.state.Attempt, owner.state.Contacts)
	}
}

func TestAcquireStartsDistinctOperationAfterOpenedAttachmentWasCleaned(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	open := func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
		client, server := net.Pipe()
		return client, func() error { return errors.Join(client.Close(), server.Close()) }, true, nil
	}
	first := Attempt{ID: [32]byte{81}, Deadline: fixture.now.Add(5 * time.Second)}
	connection, cleanup, err := owner.Acquire(context.Background(), first, open)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := owner.Acquire(context.Background(), first, open); err == nil {
		t.Fatal("Entry replayed the immediately retained terminal Attempt identity")
	}
	second := Attempt{ID: [32]byte{82}, Deadline: fixture.now.Add(5 * time.Second)}
	connection, cleanup, err = owner.Acquire(context.Background(), second, open)
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if owner.state.Attempt == nil || owner.state.Attempt.ID != second.ID || owner.state.Attempt.Terminal != "opened" ||
		len(owner.state.Contacts) != 1 || owner.state.Contacts[0].AttemptID != second.ID || !owner.state.Contacts[0].Cleanup {
		t.Fatalf("successive Entry attempt = %+v contacts = %+v", owner.state.Attempt, owner.state.Contacts)
	}
}

func TestAcquireFailsClosedWhenOpenerCannotProveCleanup(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	_, _, err = owner.Acquire(context.Background(), Attempt{ID: [32]byte{98}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			return nil, nil, false, errors.New("injected incomplete cleanup")
		})
	if err == nil || owner.state.Attempt == nil || owner.state.Attempt.Terminal != "entry-local-denial" ||
		len(owner.state.Contacts) != 1 || owner.state.Contacts[0].Cleanup {
		t.Fatalf("unclean failure err=%v attempt=%+v contacts=%+v", err, owner.state.Attempt, owner.state.Contacts)
	}
}

func TestReplacementDrainsUntilLiveAttemptSettles(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	first, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil))
	if err != nil || first.Class != Accepted {
		t.Fatalf("first import = %+v, %v", first, err)
	}
	_, _, ordinal, _, err := owner.beginAttempt(Attempt{ID: [32]byte{97}, Deadline: fixture.now.Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.Import(fixture.invite(t, fixture.candidates[1], 0, 2, &first.InviteID))
	if err != nil || second.Class != Accepted {
		t.Fatalf("replacement import = %+v, %v", second, err)
	}
	if owner.state.Records[0].Status != memberDraining || owner.state.Records[1].Status != memberVerified {
		t.Fatalf("replacement activated early: %+v", owner.state.Records)
	}
	if err := owner.finishContact(ordinal, false, true); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := owner.nextContact(); err == nil {
		t.Fatal("settled old attempt unexpectedly found another eligible contact")
	}
	contact, err := owner.Contact()
	if err != nil || contact.NodeID != fixture.candidates[1].NodeID {
		t.Fatalf("settled replacement contact = %+v, %v", contact, err)
	}
}

func TestOpenTerminalizesInterruptedAttemptAndSettlesReplacement(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	first, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil))
	if err != nil || first.Class != Accepted {
		t.Fatalf("first import = %+v, %v", first, err)
	}
	if _, _, _, _, err := owner.beginAttempt(Attempt{ID: [32]byte{96}, Deadline: fixture.now.Add(5 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if result, err := owner.Import(fixture.invite(t, fixture.candidates[1], 0, 2, &first.InviteID)); err != nil || result.Class != Accepted {
		t.Fatalf("replacement import = %+v, %v", result, err)
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
	contact, err := reopened.Contact()
	if err != nil || contact.NodeID != fixture.candidates[1].NodeID {
		t.Fatalf("interrupted replacement contact = %+v, %v", contact, err)
	}
}

func TestAcquiredCarrierStopsAfterTimeConfidenceLoss(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	confident := true
	config := fixture.config(t.TempDir())
	config.TimeConfident = func() bool { return confident }
	owner, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	defer server.Close()
	connection, cleanup, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{95}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			return client, client.Close, true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	confident = false
	if _, err := connection.Write([]byte("forbidden")); err == nil || err.Error() != "entry carrier is no longer eligible" {
		t.Fatalf("write after confidence loss = %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
}

type entryFixture struct {
	now        time.Time
	view       View
	candidates []Candidate
	private    map[[32]byte]ed25519.PrivateKey
}

func newEntryFixture(t *testing.T) entryFixture {
	t.Helper()
	now := time.Unix(1_750_000_000, 0).UTC()
	fixture := entryFixture{now: now, private: map[[32]byte]ed25519.PrivateKey{}}
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
	body = appendUint16(body, 1)
	body = append(body, fixture.view.NetworkID[:]...)
	body = appendUint64(body, fixture.view.Epoch)
	body = append(body, fixture.view.Digest[:]...)
	body = append(body, byte(len(profileID)))
	body = append(body, profileID...)
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
	raw, err := Issue(IssueInput{NetworkID: fixture.view.NetworkID, Digest: fixture.view.Digest, Epoch: fixture.view.Epoch,
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
