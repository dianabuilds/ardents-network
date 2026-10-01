package durable

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func testLimits() Limits {
	return Limits{EpochBytes: 1 << 20, RecordBytes: 32 << 10, ClosedProfileBytes: 64 << 10}
}

func TestRootLeaseAndGenerationSurviveReopen(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(path, testLimits()); err == nil {
		_ = other.Close()
		t.Fatal("second owner acquired the exclusive State root")
	}
	generation := Generation{Name: fmt.Sprintf("%064x", 1), Epoch: []byte("epoch"), Inputs: [][]byte{[]byte("input")}, Activate: true}
	if err := root.CommitState(generation); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	current, values, err := reopened.LoadState()
	if err != nil || current != generation.Name || len(values) != 1 ||
		string(values[0].Epoch) != "epoch" || len(values[0].Inputs) != 1 ||
		string(values[0].Inputs[0]) != "input" {
		t.Fatalf("recovered generation: current=%q values=%+v err=%v", current, values, err)
	}
	different := generation
	different.Epoch = []byte("other")
	if err := reopened.CommitState(different); err == nil {
		t.Fatal("overwrote an immutable State generation")
	}
}

func TestRootCloseRetainsLeaseReleaseFailure(t *testing.T) {
	root := &Root{lease: invalidReleaseLease(t)}
	first := root.Close()
	if first == nil {
		t.Fatal("Close hid the lease release failure")
	}
	if second := root.Close(); second == nil || second.Error() != first.Error() {
		t.Fatalf("repeated Close = %v, want retained %v", second, first)
	}
}

func TestRootReopensFullClosedProfilePopulationAndCleansOwnedStaging(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	var first, last ClosedProfileState
	var firstBytes, lastBytes []byte
	var lastGeneration string
	for index := 0; index < 64; index++ {
		digest := sha256.Sum256([]byte(fmt.Sprintf("generation-%d", index)))
		generation := Generation{
			Name:     hex.EncodeToString(digest[:]),
			Epoch:    []byte(fmt.Sprintf("epoch-%d", index)),
			Inputs:   [][]byte{[]byte(fmt.Sprintf("input-%d", index))},
			Activate: index == 63,
		}
		if err := root.CommitState(generation); err != nil {
			t.Fatalf("commit generation %d: %v", index, err)
		}
		profile := []byte(fmt.Sprintf("signed closed profile %d", index))
		state := ClosedProfileState{
			Generation: digest,
			Epoch:      uint64(index + 1),
			Accepted:   sha256.Sum256(profile),
		}
		if err := root.CommitClosedProfile(state, profile); err != nil {
			t.Fatalf("commit closed profile %d: %v", index, err)
		}
		if index == 0 {
			first, firstBytes = state, profile
		}
		if index == 63 {
			last, lastBytes, lastGeneration = state, profile, generation.Name
		}
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}

	rootStaging := []string{".current-interrupted-one", ".closed-profile-interrupted-one", ".closed-profile-interrupted-two"}
	for _, name := range rootStaging {
		if err := os.WriteFile(filepath.Join(path, name), []byte("interrupted"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	generationsPath := filepath.Join(path, "generations")
	generationStaging := []string{".stage-interrupted-one", ".stage-interrupted-two"}
	for _, name := range generationStaging {
		directory := filepath.Join(generationsPath, name)
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "partial"), []byte("interrupted"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	reopened, err := Open(path, testLimits())
	if err != nil {
		t.Fatalf("reopen full State root: %v", err)
	}
	defer reopened.Close()
	current, generations, err := reopened.LoadState()
	if err != nil || current != lastGeneration || len(generations) != 64 {
		t.Fatalf("recovered State generations: current=%q count=%d err=%v", current, len(generations), err)
	}
	for _, expected := range []struct {
		state   ClosedProfileState
		profile []byte
	}{
		{first, firstBytes},
		{last, lastBytes},
	} {
		state, profile, err := reopened.LoadClosedProfile(expected.state.Generation)
		if err != nil || state != expected.state || !bytes.Equal(profile, expected.profile) {
			t.Fatalf("recovered closed profile: state=%+v bytes=%q err=%v", state, profile, err)
		}
	}
	for _, name := range rootStaging {
		if _, err := os.Lstat(filepath.Join(path, name)); !os.IsNotExist(err) {
			t.Fatalf("root staging %q survived recovery: %v", name, err)
		}
	}
	for _, name := range generationStaging {
		if _, err := os.Lstat(filepath.Join(generationsPath, name)); !os.IsNotExist(err) {
			t.Fatalf("generation staging %q survived recovery: %v", name, err)
		}
	}
}

func TestRootRecoveryPreservesUnrelatedEntriesAndRefusesOverboundRoot(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	keepPath := filepath.Join(path, "unrelated-file")
	if err := os.WriteFile(keepPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	stagingPath := filepath.Join(path, ".closed-profile-interrupted")
	if err := os.WriteFile(stagingPath, []byte("interrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(keepPath); err != nil || string(raw) != "keep" {
		t.Fatalf("unrelated entry after recovery: %q, %v", raw, err)
	}
	if _, err := os.Lstat(stagingPath); !os.IsNotExist(err) {
		t.Fatalf("owned staging survived recovery: %v", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	// Five fixed entries and two files per accepted generation are the
	// permanent root budget. One more unrelated entry must be refused.
	for index := len(entries); index <= 5+2*64; index++ {
		name := fmt.Sprintf("unrelated-%03d", index)
		if err := os.WriteFile(filepath.Join(path, name), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(stagingPath, []byte("interrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if extra, err := Open(path, testLimits()); err == nil {
		_ = extra.Close()
		t.Fatal("reopened an over-bound State root")
	}
	if raw, err := os.ReadFile(stagingPath); err != nil || string(raw) != "interrupted" {
		t.Fatalf("over-bound scan partially cleaned staging: %q, %v", raw, err)
	}
	if raw, err := os.ReadFile(keepPath); err != nil || string(raw) != "keep" {
		t.Fatalf("over-bound scan changed unrelated entry: %q, %v", raw, err)
	}
}

func TestRootRecoveryRefusesDirectoryWithOwnedFilePrefix(t *testing.T) {
	path := t.TempDir()
	root, err := Open(path, testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	stagingFile := filepath.Join(path, ".current-interrupted")
	if err := os.WriteFile(stagingFile, []byte("interrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	unownedDirectory := filepath.Join(path, ".closed-profile-not-a-file")
	if err := os.Mkdir(unownedDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(path, testLimits()); err == nil {
		_ = reopened.Close()
		t.Fatal("removed a directory that cannot be ClosedProfile file staging")
	}
	if raw, err := os.ReadFile(stagingFile); err != nil || string(raw) != "interrupted" {
		t.Fatalf("invalid root was partially cleaned: %q, %v", raw, err)
	}
	if info, err := os.Lstat(unownedDirectory); err != nil || !info.IsDir() {
		t.Fatalf("unexpected directory was removed: %v", err)
	}
}

func TestRootRecoveryRefusesExcessInterruptedStagingBeforeCleanup(t *testing.T) {
	for _, location := range []string{"root", "generations"} {
		t.Run(location, func(t *testing.T) {
			path := t.TempDir()
			root, err := Open(path, testLimits())
			if err != nil {
				t.Fatal(err)
			}
			if err := root.Close(); err != nil {
				t.Fatal(err)
			}
			rootStaging := filepath.Join(path, ".current-interrupted")
			if err := os.WriteFile(rootStaging, []byte("keep until scan succeeds"), 0o600); err != nil {
				t.Fatal(err)
			}
			for index := 0; index < 65; index++ {
				name := fmt.Sprintf(".stage-interrupted-%02d", index)
				if location == "root" {
					name = fmt.Sprintf(".closed-profile-interrupted-%02d", index)
					if err := os.WriteFile(filepath.Join(path, name), []byte("interrupted"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Mkdir(filepath.Join(path, "generations", name), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if reopened, err := Open(path, testLimits()); err == nil {
				_ = reopened.Close()
				t.Fatal("reopened a root beyond its finite staging allowance")
			}
			if raw, err := os.ReadFile(rootStaging); err != nil || string(raw) != "keep until scan succeeds" {
				t.Fatalf("failed scan partially cleaned root staging: %q, %v", raw, err)
			}
		})
	}
}
