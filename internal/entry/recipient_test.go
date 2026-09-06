package entry

import (
	"bytes"
	"testing"
)

func TestRecipientIdentityPersistsAcrossReopen(t *testing.T) {
	fixture := newEntryFixture(t)
	root := entryRoot(t)
	first, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	public, err := first.RecipientPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := first.RecipientCertificate()
	if err != nil || certificate.Leaf == nil || certificate.PrivateKey == nil {
		t.Fatalf("recipient certificate = %+v, %v", certificate, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	again, err := reopened.RecipientPublicKey()
	if err != nil || again != public {
		t.Fatalf("reopened recipient key = %x, %v; want %x", again, err, public)
	}
}

func TestRootRecipientLookupDoesNotChangeInviteJournal(t *testing.T) {
	fixture := newEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	recipient, err := owner.RecipientPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	fixture.recipient = recipient
	result, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil))
	if err != nil || result.Class != Accepted {
		t.Fatalf("import = %+v, %v", result, err)
	}
	before := owner.state.clone()
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := RecipientPublicKey(root); err != nil {
		t.Fatal(err)
	}
	after, _, err := loadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if after.Generation != before.Generation || len(after.Records) != 1 || after.Records[0].Status != memberActive ||
		!bytes.Equal(after.Records[0].Invite, before.Records[0].Invite) {
		t.Fatalf("recipient lookup changed Invite journal: before=%+v after=%+v", before, after)
	}
}
