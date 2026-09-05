package entry

import "testing"

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
