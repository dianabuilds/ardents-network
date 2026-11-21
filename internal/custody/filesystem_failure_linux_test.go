//go:build linux

package custody

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// The child changes its own kernel write limit after genuine encrypted birth.
// No successful filesystem operation is supplied by a storage fake.
func TestAdmissionNativePartialWriteRefusesBeforePublication(t *testing.T) {
	if os.Getenv("ARDENTS_CUSTODY_WRITE_FAILURE_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestAdmissionNativePartialWriteRefusesBeforePublication$", "-test.v")
		cmd.Env = append(os.Environ(), "ARDENTS_CUSTODY_WRITE_FAILURE_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native write failure: %v\n%s", err, output)
		}
		return
	}
	f := newAllocationVaultFixture(t)
	op := f.request(t, 1)
	floor, err := readSmallFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	limited := original
	limited.Cur = 128 // a real partial encrypted temporary, never a complete envelope
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Fatal(err)
	}
	r, issueErr := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	if issueErr == nil || len(r.AdmissionPermission) != 0 {
		t.Fatal("partial write acknowledged")
	}
	after, err := readSmallFile(f.vault.floors)
	if err != nil || !bytes.Equal(floor, after) {
		t.Fatal("partial write changed floor")
	}
	path, _ := admissionLedgerPath(f.vault.root, f.created.RecordID)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("partial successor published")
	}
	if err := f.vault.Close(); err != nil {
		t.Fatal(err)
	}
	f.vault, err = Open(VaultConfig{Root: f.vault.root, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if err != nil || retry.Authority.Revision != 1 {
		t.Fatalf("uncommitted write retry: %v", err)
	}
}

func TestAdmissionNativeFloorFailureRetainsOriginalRefusal(t *testing.T) {
	f := newAllocationVaultFixture(t)
	op := f.request(t, 1)
	floor, err := readSmallFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.vault.now = func() time.Time {
		calls++
		if calls == 4 {
			if err := os.Remove(f.vault.floors); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(f.vault.floors, 0700); err != nil {
				t.Fatal(err)
			}
		}
		return f.now
	}
	refused, issueErr := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if issueErr == nil || len(refused.AdmissionPermission) != 0 {
		t.Fatal("failed floor acknowledged")
	}
	path, _ := admissionLedgerPath(f.vault.root, f.created.RecordID)
	raw, err := readEnvelopeFile(path)
	if err != nil {
		t.Fatalf("successor not written before floor refusal: %v", err)
	}
	if _, err := inspectEnvelope(raw); err != nil {
		t.Fatal(err)
	}
	if err := f.vault.Close(); err != nil {
		t.Fatal(err)
	}
	f.vault, err = Open(VaultConfig{Root: f.vault.root, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	if r, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}}); err == nil || len(r.AdmissionPermission) != 0 {
		t.Fatal("invalid floor reopened successfully")
	}
	// Explicit test repair reinstates the exact retained floor, never a fresh
	// history. A new invocation must authenticate and finish the one successor.
	if err := os.Remove(f.vault.floors); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicPrivate(f.vault.floors, floor); err != nil {
		t.Fatal(err)
	}
	retried, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if err != nil || retried.Authority.Revision != 1 {
		t.Fatalf("authenticated successor recovery: %v", err)
	}
	after, err := readEnvelopeFile(path)
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("recovery reallocated instead of retaining successor")
	}
	if issueErr == nil {
		t.Fatal("later recovery erased original failure")
	}
}
