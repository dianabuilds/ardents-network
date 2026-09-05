package entry

import (
	"testing"
	"time"
)

func TestAdmitterPersistsExactReplayTupleAcrossReopen(t *testing.T) {
	fixture := newEntryFixture(t)
	root := entryRoot(t)
	admitter, err := OpenAdmitter(AdmitterConfig{Root: root, Verification: fixture.verification()})
	if err != nil {
		t.Fatal(err)
	}
	raw := fixture.invite(t, fixture.candidates[0], 0, 1, nil)
	attachment, clientKey := [32]byte{71}, [32]byte{72}
	authorization, err := admitter.AdmitAndConsume(raw, attachment, clientKey, fixture.recipient, fixture.now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admitter.AdmitAndConsume(raw, attachment, clientKey, fixture.recipient, authorization.NotAfter); err == nil {
		t.Fatal("duplicate Entry binding tuple was accepted")
	}
	if _, err := admitter.AdmitAndConsume(raw, attachment, [32]byte{74}, fixture.recipient, authorization.NotAfter); err == nil {
		t.Fatal("replayed attachment with a different client key was accepted")
	}
	if _, err := admitter.AdmitAndConsume(raw, [32]byte{75}, clientKey, [32]byte{76}, authorization.NotAfter); err == nil {
		t.Fatal("copied Invite from another recipient key was accepted")
	}
	if err := admitter.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenAdmitter(AdmitterConfig{Root: root, Verification: fixture.verification()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.AdmitAndConsume(raw, attachment, clientKey, fixture.recipient, authorization.NotAfter); err == nil {
		t.Fatalf("replayed tuple after reopen = %v", err)
	}
	if _, err := reopened.AdmitAndConsume(raw, [32]byte{73}, clientKey, fixture.recipient, authorization.NotAfter); err != nil {
		t.Fatalf("distinct tuple was rejected: %v", err)
	}
}
