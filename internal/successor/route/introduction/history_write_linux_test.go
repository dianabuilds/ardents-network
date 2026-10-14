//go:build linux

package introduction

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A separate child changes only its own actual kernel file-size bound. No
// storage result or successful synchronization is supplied by a fixture.
func TestHistoryNativePartialWriteRetainsFailureAndFootprint(t *testing.T) {
	if os.Getenv("ARDENTS_INTRODUCTION_PARTIAL_WRITE_CHILD") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestHistoryNativePartialWriteRetainsFailureAndFootprint$", "-test.v=true")
		command.Env = append(os.Environ(), "ARDENTS_INTRODUCTION_PARTIAL_WRITE_CHILD=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("actual native partial write: %v\n%s", err, output)
		}
		return
	}
	history, _, root, binding := newHistoryFixture(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	prior, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil {
		t.Fatal(err)
	}
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	signal.Ignore(syscall.SIGXFSZ)
	defer signal.Reset(syscall.SIGXFSZ)
	limited := original
	limited.Cur = 1
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limited); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
			t.Error(err)
		}
	}()
	first := history.Claim([32]byte{8}, now.Add(time.Minute), now.Add(time.Second))
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(first, io.ErrShortWrite) && !errors.Is(first, syscall.EFBIG) {
		t.Fatal("actual one-byte write did not fail", first)
	}
	names, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var stage string
	for _, name := range names {
		if strings.HasPrefix(name.Name(), ".introduction-snapshot-") {
			if stage != "" {
				t.Fatal("multiple interrupted stages")
			}
			stage = name.Name()
		}
	}
	partial, err := os.ReadFile(filepath.Join(root, stage))
	if stage == "" || err != nil || !bytes.Equal(partial, []byte{'A'}) {
		t.Fatal("actual partial footprint differs", partial, err)
	}
	if history.Check() != first || history.Failure() != first {
		t.Fatal("restored native limits revived original or changed cause")
	}
	if err := history.Claim([32]byte{9}, now.Add(time.Minute), now.Add(time.Second)); err != first {
		t.Fatal("failed original wrote again", err)
	}
	// The native lease itself remains held, independently of the stage refusal.
	lease, err := os.OpenFile(filepath.Join(root, slotLeaseName), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		_ = lease.Close()
		t.Fatal("failed original lost exclusive lease", err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := history.Close(); !errors.Is(err, first) {
		t.Fatal("joined Close lost original native failure", err)
	}
	if reopened, err := OpenHistory(root, binding); err == nil {
		_ = reopened.Close()
		t.Fatal("partial replacement automatically repaired")
	}
	after, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil || !bytes.Equal(after, prior) {
		t.Fatal("partial replacement changed retained prior floor", err)
	}
	retained, err := os.ReadFile(filepath.Join(root, stage))
	if err != nil || !bytes.Equal(retained, partial) {
		t.Fatal("refused reopen erased partial evidence", err)
	}
}
