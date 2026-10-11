//go:build linux

package custody

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

func TestServiceNativePartialWriteRefusesBeforePublication(t *testing.T) {
	if os.Getenv("ARDENTS_SERVICE_CUSTODY_PARTIAL_CHILD") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestServiceNativePartialWriteRefusesBeforePublication$", "-test.v")
		cmd.Env = append(os.Environ(), "ARDENTS_SERVICE_CUSTODY_PARTIAL_CHILD=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native partial write: %v\n%s", err, output)
		}
		return
	}
	f := newServiceVaultFixture(t)
	op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	floor, err := os.ReadFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	limited := original
	limited.Cur = 128
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Fatal(err)
	}
	r, issueErr := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	if issueErr == nil || len(r.ServiceResponse) != 0 {
		t.Fatal("partial write acknowledged")
	}
	after, err := os.ReadFile(f.vault.floors)
	if err != nil || !bytes.Equal(floor, after) {
		t.Fatal("partial write advanced floor", err)
	}
	view, err := publication.ParseServiceRequest(op.ServiceRequest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.vault.records, "record-"+serviceSuccessorRecordID(op.RecordID, view.Commitment)+".json")
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial successor published", err)
	}
}

func TestServiceNativeFloorFailureAndExpiredPendingCompletion(t *testing.T) {
	f := newServiceVaultFixture(t)
	op := f.request(f.now, f.now.Add(time.Hour), f.created.RecordID)
	floor, err := os.ReadFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.vault.now = func() time.Time {
		calls++
		if calls == 3 {
			if err := os.Remove(f.vault.floors); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(f.vault.floors, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		return f.now
	}
	r, issueErr := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if issueErr == nil || len(r.ServiceResponse) != 0 {
		t.Fatal("floor failure acknowledged")
	}
	view, err := publication.ParseServiceRequest(op.ServiceRequest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.vault.records, "record-"+serviceSuccessorRecordID(op.RecordID, view.Commitment)+".json")
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("original pending successor absent", err)
	}
	if err := f.vault.Close(); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(2 * time.Hour)
	f.vault, err = Open(VaultConfig{Root: f.root, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	if r, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}}); err == nil || len(r.ServiceResponse) != 0 {
		t.Fatal("invalid floor reopened successfully")
	}
	// Explicit fault-control repair restores only exact retained predecessor
	// evidence. This is never a production floor reconstruction/reset route.
	if err := os.Remove(f.vault.floors); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicPrivate(f.vault.floors, floor); err != nil {
		t.Fatal(err)
	}
	retry, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}})
	if err != nil || len(retry.ServiceResponse) == 0 || retry.Authority.Revision != 1 {
		t.Fatal("exact pending completion refused", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(persisted, after) {
		t.Fatal("pending completion rewrote successor", err)
	}
}
