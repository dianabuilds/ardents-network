package durable

import (
	"bytes"
	"crypto/sha256"
	"errors"
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

func TestClosedProfileExactRetryRequiresFileSyncBeforeStateRecord(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	profile := []byte("complete but unsynced signed profile")
	state := ClosedProfileState{Generation: sha256.Sum256([]byte("unsynced generation")), Epoch: 7, Accepted: sha256.Sum256(profile)}
	path := filepath.Join(rootPath, closedProfileBytesName(state.Generation))
	if err := os.WriteFile(path, profile, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("byte-only profile became accepted: state=%+v raw=%q err=%v", got, raw, err)
	}
	failure := errors.New("injected profile file sync failure")
	fileSynced, directorySynced := false, false
	ops := standardClosedProfileByteOps()
	ops.syncExisting = func(got string) error {
		fileSynced = true
		if got != path {
			t.Fatalf("synced profile path %q, want %q", got, path)
		}
		return failure
	}
	ops.syncDirectory = func(string) error {
		directorySynced = true
		return nil
	}
	err = root.commitClosedProfileWithOps(state, profile, ops)
	if !errors.Is(err, failure) || !fileSynced || directorySynced {
		t.Fatalf("exact retry after unsynced orphan: err=%v file sync=%t directory sync=%t", err, fileSynced, directorySynced)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("failed sync published accepted profile: state=%+v raw=%q err=%v", got, raw, err)
	}
}

func TestClosedProfileBytePhaseFailureLeavesNoFinalFileOrAcceptedRecord(t *testing.T) {
	for _, phase := range []string{"write", "file sync", "close"} {
		t.Run(phase, func(t *testing.T) {
			rootPath := t.TempDir()
			root, err := Open(rootPath, testLimits())
			if err != nil {
				t.Fatal(err)
			}
			profile := []byte("signed profile whose first publication fails")
			state := ClosedProfileState{Generation: sha256.Sum256([]byte(phase)), Epoch: 7, Accepted: sha256.Sum256(profile)}
			failure := errors.New("injected " + phase + " failure")
			ops := standardClosedProfileByteOps()
			switch phase {
			case "write":
				ops.write = func(file *os.File, raw []byte) (int, error) {
					written, err := file.Write(raw[:len(raw)/2])
					return written, errors.Join(err, failure)
				}
			case "file sync":
				ops.syncFile = func(*os.File) error { return failure }
			case "close":
				ops.closeFile = func(file *os.File) error { return errors.Join(file.Close(), failure) }
			}
			if err := root.commitClosedProfileWithOps(state, profile, ops); !errors.Is(err, failure) {
				t.Fatalf("%s failure = %v", phase, err)
			}
			if _, err := os.Stat(filepath.Join(rootPath, closedProfileBytesName(state.Generation))); !os.IsNotExist(err) {
				t.Fatalf("%s left final profile file: %v", phase, err)
			}
			if _, err := os.Stat(filepath.Join(rootPath, closedProfileStateName(state.Generation))); !os.IsNotExist(err) {
				t.Fatalf("%s published accepted state: %v", phase, err)
			}
			if err := root.Close(); err != nil {
				t.Fatal(err)
			}
			root, err = Open(rootPath, testLimits())
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
				t.Fatalf("%s became accepted after reopen: state=%+v raw=%q err=%v", phase, got, raw, err)
			}
			if err := root.CommitClosedProfile(state, profile); err != nil {
				t.Fatalf("exact retry after %s: %v", phase, err)
			}
			if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != state || !bytes.Equal(raw, profile) {
				t.Fatalf("accepted retry after %s: state=%+v raw=%q err=%v", phase, got, raw, err)
			}
		})
	}
}

func TestClosedProfileExistingByteRetryRequiresDirectorySync(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	profile := []byte("complete signed profile orphan")
	state := ClosedProfileState{Generation: sha256.Sum256([]byte("directory sync generation")), Epoch: 7, Accepted: sha256.Sum256(profile)}
	if err := os.WriteFile(filepath.Join(rootPath, closedProfileBytesName(state.Generation)), profile, 0o600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected profile directory sync failure")
	ops := standardClosedProfileByteOps()
	fileSynced := false
	ops.syncExisting = func(path string) error {
		fileSynced = true
		return syncClosedProfileFile(path)
	}
	ops.syncDirectory = func(path string) error {
		if path != rootPath {
			t.Fatalf("synced directory %q, want %q", path, rootPath)
		}
		return failure
	}
	if err := root.commitClosedProfileWithOps(state, profile, ops); !errors.Is(err, failure) || !fileSynced {
		t.Fatalf("existing-byte directory sync failure: err=%v file sync=%t", err, fileSynced)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("directory sync failure published accepted profile: state=%+v raw=%q err=%v", got, raw, err)
	}
	if err := root.CommitClosedProfile(state, profile); err != nil {
		t.Fatalf("exact retry: %v", err)
	}
}

func TestClosedProfileNewByteDirectorySyncFailureRequiresExactRetry(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	profile := []byte("synced bytes before failed parent sync")
	state := ClosedProfileState{Generation: sha256.Sum256([]byte("new-byte directory failure")), Epoch: 7, Accepted: sha256.Sum256(profile)}
	failure := errors.New("injected first profile directory sync failure")
	ops := standardClosedProfileByteOps()
	ops.syncDirectory = func(path string) error {
		if path != rootPath {
			t.Fatalf("synced directory %q, want %q", path, rootPath)
		}
		return failure
	}
	if err := root.commitClosedProfileWithOps(state, profile, ops); !errors.Is(err, failure) {
		t.Fatalf("new-byte directory sync failure = %v", err)
	}
	path := filepath.Join(rootPath, closedProfileBytesName(state.Generation))
	if raw, err := os.ReadFile(path); err != nil || !bytes.Equal(raw, profile) {
		t.Fatalf("published bytes after failed directory sync: raw=%q err=%v", raw, err)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("byte-only profile became accepted: state=%+v raw=%q err=%v", got, raw, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	root, err = Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("reopen accepted byte-only profile: state=%+v raw=%q err=%v", got, raw, err)
	}
	if err := root.CommitClosedProfile(state, profile); err != nil {
		t.Fatalf("exact retry after parent sync failure: %v", err)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != state || !bytes.Equal(raw, profile) {
		t.Fatalf("accepted exact retry: state=%+v raw=%q err=%v", got, raw, err)
	}
}

func TestClosedProfilePartialFinalBytesRemainUnavailable(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	profile := []byte("complete signed profile")
	state := ClosedProfileState{Generation: sha256.Sum256([]byte("partial final generation")), Epoch: 7, Accepted: sha256.Sum256(profile)}
	path := filepath.Join(rootPath, closedProfileBytesName(state.Generation))
	partial := profile[:len(profile)/2]
	if err := os.WriteFile(path, partial, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.CommitClosedProfile(state, profile); err == nil {
		t.Fatal("partial final profile bytes were silently replaced")
	}
	if raw, err := os.ReadFile(path); err != nil || !bytes.Equal(raw, partial) {
		t.Fatalf("partial final evidence changed: raw=%q err=%v", raw, err)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("partial final profile became accepted: state=%+v raw=%q err=%v", got, raw, err)
	}
}

func TestClosedProfilePublicationNeverReplacesInterveningFinalFile(t *testing.T) {
	rootPath := t.TempDir()
	root, err := Open(rootPath, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	profile := []byte("new signed profile")
	state := ClosedProfileState{Generation: sha256.Sum256([]byte("intervening final generation")), Epoch: 7, Accepted: sha256.Sum256(profile)}
	path := filepath.Join(rootPath, closedProfileBytesName(state.Generation))
	intervening := []byte("pre-existing immutable bytes")
	ops := standardClosedProfileByteOps()
	ops.closeFile = func(file *os.File) error {
		if err := file.Close(); err != nil {
			return err
		}
		return os.WriteFile(path, intervening, 0o600)
	}
	if err := root.commitClosedProfileWithOps(state, profile, ops); err == nil {
		t.Fatal("publication replaced a final file created after the initial absence check")
	}
	if raw, err := os.ReadFile(path); err != nil || !bytes.Equal(raw, intervening) {
		t.Fatalf("intervening immutable bytes changed: raw=%q err=%v", raw, err)
	}
	if got, raw, err := root.LoadClosedProfile(state.Generation); err != nil || got != (ClosedProfileState{}) || raw != nil {
		t.Fatalf("intervening final file became accepted: state=%+v raw=%q err=%v", got, raw, err)
	}
}

func TestClosedProfileStateSyncFailureMarksVisibleRecordUncertain(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "accepted"
		if conflict {
			name = "conflict"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			state := ClosedProfileState{
				Generation: sha256.Sum256([]byte("generation")),
				Epoch:      7,
				Accepted:   sha256.Sum256([]byte("accepted profile")),
			}
			if conflict {
				state.Conflict = sha256.Sum256([]byte("second signed profile"))
			}
			name := closedProfileStateName(state.Generation)
			raw := encodeClosedProfileState(state)
			syncFailure := errors.New("injected directory sync failure")
			err := replaceClosedProfileStateWithSync(root, name, raw, func(path string) error {
				if path != root {
					t.Fatalf("synced directory %q, want %q", path, root)
				}
				visible, readErr := os.ReadFile(filepath.Join(root, name))
				if readErr != nil || !bytes.Equal(visible, raw) {
					t.Fatalf("state record after rename: read=%v bytes=%x", readErr, visible)
				}
				return syncFailure
			})
			if !errors.Is(err, ErrClosedProfileStateSyncUncertain) || !errors.Is(err, syncFailure) {
				t.Fatalf("post-rename sync failure = %v", err)
			}
			visible, err := os.ReadFile(filepath.Join(root, name))
			if err != nil || !bytes.Equal(visible, raw) {
				t.Fatalf("visible state record: read=%v bytes=%x", err, visible)
			}
			decoded, err := decodeClosedProfileState(visible)
			if err != nil || decoded != state {
				t.Fatalf("visible state identity: %+v, %v", decoded, err)
			}
		})
	}
}

func TestClosedProfileStateFailureBeforeRenameIsNotSyncUncertain(t *testing.T) {
	missingRoot := filepath.Join(t.TempDir(), "missing")
	syncCalled := false
	err := replaceClosedProfileStateWithSync(missingRoot, "state", []byte("state"), func(string) error {
		syncCalled = true
		return nil
	})
	if err == nil || errors.Is(err, ErrClosedProfileStateSyncUncertain) || syncCalled {
		t.Fatalf("pre-rename failure = %v, sync called=%t", err, syncCalled)
	}
}
