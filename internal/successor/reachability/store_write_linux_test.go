//go:build linux

package reachability_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// The separate child changes only its own native file-size limit. No write,
// synchronization, successful publication or reopened floor is simulated.
func TestStoreNativePartialWriteRetainsOriginalFailureAndPriorFloor(t *testing.T) {
	if os.Getenv("ARDENTS_REACHABILITY_PARTIAL_WRITE_CHILD") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestStoreNativePartialWriteRetainsOriginalFailureAndPriorFloor$", "-test.v=true")
		command.Env = append(os.Environ(), "ARDENTS_REACHABILITY_PARTIAL_WRITE_CHILD=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("actual native partial write: %v\n%s", err, output)
		}
		return
	}
	raw, target, network, profile, at := signedDescriptor(3)
	config := reachability.StoreConfig{Root: t.TempDir(), Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	check := func() error {
		_, err := reachability.VerifyPublish(raw, network, profile, at)
		return err
	}
	if outcome, err := store.Publish(raw, profile, at, check); err != nil || outcome != reachability.Accepted {
		t.Fatal(outcome, err)
	}
	prior, outcome, err := store.Lookup(target, profile, at)
	if err != nil || outcome != reachability.Accepted || !bytes.Equal(prior, raw) {
		t.Fatal("genuine initial commit", outcome, err)
	}
	higher := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], 10) })
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
	publication, writeErr := store.Publish(higher, profile, at, check)
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Fatal(err)
	}
	if publication != reachability.Invalid || !errors.Is(writeErr, io.ErrShortWrite) && !errors.Is(writeErr, syscall.EFBIG) {
		t.Fatal("partial native write did not refuse", publication, writeErr)
	}
	stage := filepath.Join(config.Root, "records", ".record-stage")
	partial, err := os.ReadFile(stage)
	if err != nil || !bytes.Equal(partial, []byte{3}) {
		t.Fatal("actual partial stage footprint differs", partial, err)
	}
	first := store.Check()
	if first == nil || !errors.Is(first, io.ErrShortWrite) && !errors.Is(first, syscall.EFBIG) {
		t.Fatal("original native cause lost", first)
	}
	if store.Check() != first {
		t.Fatal("restored kernel conditions changed original failure")
	}
	if got, outcome, err := store.Lookup(target, profile, at); err == nil || outcome != reachability.Invalid || len(got) != 0 {
		t.Fatal("failed owner returned predecessor", outcome, err)
	}
	if outcome, err := store.Publish(higher, profile, at, check); err != first || outcome != reachability.Invalid {
		t.Fatal("restored writer revived original", outcome, err)
	}
	if duplicate, err := reachability.OpenStore(config); err == nil {
		_ = duplicate.Close()
		t.Fatal("failed original released lease before Close")
	}
	if err := store.Close(); err != nil || store.Close() != err {
		t.Fatal("operational failure counterfeited physical release", err)
	}
	fresh, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal("independent validated interrupted-stage recovery", err)
	}
	defer fresh.Close()
	if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("validated recovery retained own interrupted stage", err)
	}
	got, outcome, err := fresh.Lookup(target, profile, at)
	if err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
		t.Fatal("partial successor changed original committed floor", outcome, err)
	}
	if outcome, err := fresh.Publish(higher, profile, at, check); err != nil || outcome != reachability.Accepted {
		t.Fatal("fresh explicit owner unavailable after full validation", outcome, err)
	}
	if store.Check() != first {
		t.Fatal("fresh owner revived retired original")
	}
}
