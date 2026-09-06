package entry

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestImportRejectsForeignRecipientBeforeReplacement(t *testing.T) {
	fixture := newEntryFixture(t)
	firstRoot := entryRoot(t)
	firstOwner, err := Open(fixture.config(firstRoot))
	if err != nil {
		t.Fatal(err)
	}
	defer firstOwner.Close()
	firstRecipient, err := firstOwner.RecipientPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	fixture.recipient = firstRecipient
	first, err := firstOwner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil))
	if err != nil || first.Class != Accepted {
		t.Fatalf("first import = %+v, %v", first, err)
	}

	secondOwner, err := Open(fixture.config(entryRootForRecipient(t, bytes.Repeat([]byte{92}, ed25519.SeedSize))))
	if err != nil {
		t.Fatal(err)
	}
	foreignRecipient, err := secondOwner.RecipientPublicKey()
	if closeErr := secondOwner.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	fixture.recipient = foreignRecipient
	foreign := fixture.invite(t, fixture.candidates[1], 0, 2, &first.InviteID)
	result, err := firstOwner.Import(foreign)
	if err != nil || result.Class != WrongRecipient {
		t.Fatalf("foreign replacement = %+v, %v", result, err)
	}

	contact, err := firstOwner.Contact()
	if err != nil || contact.NodeID != fixture.candidates[0].NodeID {
		t.Fatalf("foreign replacement changed contact = %+v, %v", contact, err)
	}
	fixture.recipient = firstRecipient
	correct := fixture.invite(t, fixture.candidates[1], 0, 2, &first.InviteID)
	result, err = firstOwner.Import(correct)
	if err != nil || result.Class != Accepted {
		t.Fatalf("correct replacement = %+v, %v", result, err)
	}
}

func TestOpenRetiresPersistedForeignRecipientInvite(t *testing.T) {
	fixture := newEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	foreignOwner, err := Open(fixture.config(entryRootForRecipient(t, bytes.Repeat([]byte{92}, ed25519.SeedSize))))
	if err != nil {
		t.Fatal(err)
	}
	foreignRecipient, err := foreignOwner.RecipientPublicKey()
	if closeErr := foreignOwner.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	fixture.recipient = foreignRecipient
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	decoded, _, class, err := validateInvite(raw, fixture.verification())
	if err != nil || class != Accepted {
		t.Fatalf("foreign Invite fixture = %+v, %q, %v", decoded, class, err)
	}
	next := owner.state.clone()
	next.Records = append(next.Records, memberRecord{InviteID: decoded.id, Identity: decoded.nodeID, Family: decoded.familyID,
		Slot: decoded.slot, Generation: decoded.slotGeneration, Status: memberActive, Invite: raw})
	if err := owner.commit(next, true); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Contact(); err == nil {
		t.Fatal("reopened foreign Invite became a Contact")
	}
	result, err := reopened.Import(raw)
	if err != nil || result.Class != WrongRecipient {
		t.Fatalf("reopened foreign Invite = %+v, %v", result, err)
	}
	if len(reopened.state.Records) != 1 || reopened.state.Records[0].Status != memberRetired || reopened.state.Records[0].Invite != nil {
		t.Fatalf("reopened foreign Invite did not retain a retired replay witness: %+v", reopened.state.Records)
	}
}

func TestOpenRestoresDrainingPredecessorAfterRetiringForeignReplacement(t *testing.T) {
	fixture := newEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	localRecipient, err := owner.RecipientPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	fixture.recipient = localRecipient
	first, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil))
	if err != nil || first.Class != Accepted {
		t.Fatalf("first import = %+v, %v", first, err)
	}
	if _, _, _, _, err := owner.beginAttempt(Attempt{ID: [32]byte{1}, Deadline: fixture.now.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}

	foreignOwner, err := Open(fixture.config(entryRootForRecipient(t, bytes.Repeat([]byte{92}, ed25519.SeedSize))))
	if err != nil {
		t.Fatal(err)
	}
	foreignRecipient, err := foreignOwner.RecipientPublicKey()
	if closeErr := foreignOwner.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	fixture.recipient = foreignRecipient
	foreign := fixture.invite(t, fixture.candidates[1], 0, 2, &first.InviteID)
	decoded, _, class, err := validateInvite(foreign, fixture.verification())
	if err != nil || class != Accepted {
		t.Fatalf("foreign replacement fixture = %+v, %q, %v", decoded, class, err)
	}
	next := owner.state.clone()
	active, found := next.active(0)
	if !found || next.Attempt == nil {
		t.Fatalf("missing active predecessor or attempt: %+v", next)
	}
	next.Records[active].Status = memberDraining
	next.Records = append(next.Records, memberRecord{InviteID: decoded.id, Identity: decoded.nodeID, Family: decoded.familyID,
		Slot: decoded.slot, Generation: decoded.slotGeneration, Status: memberVerified, Invite: foreign})
	next.Attempt.Terminal, next.Attempt.Ended = "entry-interrupted", next.Attempt.Started
	if err := owner.commit(next, true); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	contact, err := reopened.Contact()
	if err != nil || contact.NodeID != fixture.candidates[0].NodeID {
		t.Fatalf("recovered predecessor contact = %+v, %v", contact, err)
	}
	if len(reopened.state.Records) != 2 || reopened.state.Records[0].Status != memberActive || reopened.state.Records[1].Status != memberRetired {
		t.Fatalf("recovered replacement state = %+v", reopened.state.Records)
	}
}
