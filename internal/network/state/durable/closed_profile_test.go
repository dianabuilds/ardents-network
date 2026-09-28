package durable

import (
	"crypto/sha256"
	"os"
	"path/filepath"
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

func TestClosedProfileOrphanRequiresExactRetry(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	generation := sha256.Sum256([]byte("generation"))
	first := []byte("one signed closed profile")
	profilePath := filepath.Join(rootPath, closedProfileBytesName(generation))
	if err := os.WriteFile(profilePath, first, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	root, err = Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if state, raw, err := root.LoadClosedProfile(generation); err != nil || state != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("uncommitted profile became visible: state=%+v, raw=%q, err=%v", state, raw, err)
	}
	different := []byte("different signed closed profile")
	other := ClosedProfileState{Generation: generation, Epoch: 7, Accepted: sha256.Sum256(different)}
	if err := root.CommitClosedProfile(other, different); err == nil {
		t.Fatal("replaced immutable orphan with a different profile")
	}
	if state, raw, err := root.LoadClosedProfile(generation); err != nil || state != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("failed different retry changed acceptance: state=%+v, raw=%q, err=%v", state, raw, err)
	}
	accepted := ClosedProfileState{Generation: generation, Epoch: 7, Accepted: sha256.Sum256(first)}
	if err := root.CommitClosedProfile(accepted, first); err != nil {
		t.Fatalf("exact retry: %v", err)
	}
	if state, raw, err := root.LoadClosedProfile(generation); err != nil || state != accepted || string(raw) != string(first) {
		t.Fatalf("accepted exact retry: state=%+v, raw=%q, err=%v", state, raw, err)
	}
}
