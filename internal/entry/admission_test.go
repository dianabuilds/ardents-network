package entry

import "testing"

func TestAdmitterPersistsExactReplayTupleAcrossReopen(t *testing.T) {
	fixture := newEntryFixture(t)
	root := t.TempDir()
	admitter, err := OpenAdmitter(AdmitterConfig{Root: root, Verification: fixture.verification()})
	if err != nil {
		t.Fatal(err)
	}
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	attachment, clientKey := [32]byte{71}, [32]byte{72}
	authorization, err := admitter.Admit(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitter.AdmitAndConsume(raw, attachment, clientKey, authorization.NotAfter); err != nil {
		t.Fatal(err)
	}
	if _, err := admitter.AdmitAndConsume(raw, attachment, clientKey, authorization.NotAfter); err == nil {
		t.Fatal("duplicate Entry binding tuple was accepted")
	}
	if err := admitter.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAdmitter(AdmitterConfig{Root: root, Verification: fixture.verification()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	authorization, err = reopened.Admit(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.AdmitAndConsume(raw, attachment, clientKey, authorization.NotAfter); err == nil {
		t.Fatalf("replayed tuple after reopen = %v", err)
	}
	if _, err := reopened.AdmitAndConsume(raw, [32]byte{73}, clientKey, authorization.NotAfter); err != nil {
		t.Fatalf("distinct tuple was rejected: %v", err)
	}
}
