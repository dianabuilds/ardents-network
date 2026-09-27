package durable

import (
	"crypto/sha256"
	"testing"
)

func TestClosedProfileStorePersistsConflictAcrossReopen(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	profile := []byte("one signed closed profile")
	accepted := sha256.Sum256(profile)
	state := ClosedProfileState{Generation: sha256.Sum256([]byte("generation")), Epoch: 7, Accepted: accepted}
	if err := root.CommitClosedProfile(state, profile); err != nil {
		t.Fatal(err)
	}
	state.Conflict = sha256.Sum256([]byte("different signed profile"))
	if err := root.CommitClosedProfile(state, profile); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, raw, err := reopened.LoadClosedProfile(state.Generation)
	if err != nil || got != state || string(raw) != string(profile) {
		t.Fatalf("reopen closed profile = %+v, %q, %v", got, raw, err)
	}
	nextProfile := []byte("next epoch signed closed profile")
	next := ClosedProfileState{Generation: sha256.Sum256([]byte("next generation")), Epoch: 8, Accepted: sha256.Sum256(nextProfile)}
	if err := reopened.CommitClosedProfile(next, nextProfile); err != nil {
		t.Fatalf("persist successor profile: %v", err)
	}
	if got, raw, err := reopened.LoadClosedProfile(next.Generation); err != nil || got != next || string(raw) != string(nextProfile) {
		t.Fatalf("load successor profile = %+v, %q, %v", got, raw, err)
	}
}
