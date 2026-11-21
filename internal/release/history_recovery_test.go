package release

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This subprocess terminates without Close or deferred cleanup at actual
// publication boundaries. It tests process interruption, not power loss.
func TestHistoryProcessInterruption(t *testing.T) {
	if phase := os.Getenv("ARDENTS_RELEASE_CRASH_TEST_PHASE"); phase != "" {
		root := os.Getenv("ARDENTS_RELEASE_CRASH_TEST_ROOT")
		h := freshSignedHistory(t)
		v, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		if d := v.Evaluate(context.Background(), h.input); d.Outcome != OutcomeReleaseAccepted {
			t.Fatal(d.Err())
		}
		h.resignOnline(t, h.keys, 2)
		v.store.flush = func(path string) error {
			match := phase == "stage" && strings.HasPrefix(filepath.Base(path), ".stage-") || phase == "generation" && path == filepath.Join(root, "generations") || (phase == "pointer" || phase == "flushed") && path == root
			if match && phase != "flushed" {
				os.Exit(77)
			}
			err := syncDirectory(path)
			if err == nil && match {
				os.Exit(77)
			}
			return err
		}
		_ = v.Evaluate(context.Background(), h.input)
		t.Fatal("publication interruption point was not reached")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"stage", "generation", "pointer", "flushed"} {
		t.Run(phase, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "history")
			cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestHistoryProcessInterruption$", "-test.count=1")
			cmd.Env = append(os.Environ(), "ARDENTS_RELEASE_CRASH_TEST_PHASE="+phase, "ARDENTS_RELEASE_CRASH_TEST_ROOT="+root)
			output, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 77 {
				t.Fatalf("crash boundary not established: %v %s", err, output)
			}
			v, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := v.Close(); err != nil {
					t.Error(err)
				}
			})
			f, err := v.CurrentFloors(context.Background())
			want := int64(1)
			if phase == "pointer" || phase == "flushed" {
				want = 2
			}
			if err != nil || f.RootVersion != 1 || f.TimestampVersion != want || f.SnapshotVersion != want || f.TargetsVersion != want {
				t.Fatalf("interruption changed or split retained floors: %+v %v", f, err)
			}
		})
	}
}

func TestRecoverExactWriterResidueRetainsCommittedHistory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if d := v.Evaluate(context.Background(), publicVector(t)); d.Outcome != OutcomeReleaseAccepted {
		t.Fatal(d.Err())
	}
	f, err := v.CurrentFloors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "generations", ".stage-12345")
	if err = os.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(stage, "state.bin"), []byte("role=root version=1 digest=abc"), 0600); err != nil {
		t.Fatal(err)
	}
	pointer := filepath.Join(root, ".current-67890")
	if err = os.WriteFile(pointer, []byte("012345abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	v, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	retained, err := v.CurrentFloors(context.Background())
	if err != nil || !floorSetEqual(f, retained) {
		t.Fatal("writer residue changed committed history")
	}
	for _, p := range []string{stage, pointer} {
		if _, err = os.Lstat(p); !os.IsNotExist(err) {
			t.Fatal("exact own residue not recovered")
		}
	}
}
func TestMalformedResidueRefusesWithoutDeletingOtherResidue(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if d := v.Evaluate(context.Background(), publicVector(t)); d.Outcome != OutcomeReleaseAccepted {
		t.Fatal(d.Err())
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(root, ".current-1")
	bad := filepath.Join(root, ".current-2")
	if err = os.WriteFile(valid, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(bad, []byte("foreign-pointer"), 0600); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(root); err == nil {
		_ = reopened.Close()
		t.Fatal("foreign residue accepted")
	}
	for _, path := range []string{valid, bad} {
		if _, err = os.Lstat(path); err != nil {
			t.Fatal("refusal deleted residue")
		}
	}
}
func TestInterruptedFloorGrammarPrefixes(t *testing.T) {
	payload := []byte("role=root version=1 digest=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n")
	for i := 0; i <= len(payload); i++ {
		if !floorPayloadPrefix(payload[:i]) {
			t.Fatalf("writer prefix refused at%d", i)
		}
	}
	for _, bad := range []string{"role=root version=0", "role=root version=01", "role=root version=1 digest=g", "role=root version=1 strange", "role=targets version=1"} {
		if floorPayloadPrefix([]byte(bad)) {
			t.Fatalf("malformed prefix accepted: %s", bad)
		}
	}
}
