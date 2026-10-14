//go:build linux || windows

package reachability_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
)

// Delaying a foreign observation must not hold the Store state lock. Close,
// however, must retain the original lease until that publication has returned.
func TestStoreForeignObservationDoesNotLockStateAndCloseJoins(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	root := t.TempDir()
	config := reachability.StoreConfig{Root: root, Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	published := make(chan error, 1)
	checks := 0
	go func() {
		_, err := store.Publish(raw, profile, at, func() error {
			checks++
			if checks == 2 {
				close(entered)
				<-resume
			}
			return nil
		})
		published <- err
	}()
	<-entered
	looked := make(chan error, 1)
	go func() { _, _, err := store.Lookup(target, profile, at); looked <- err }()
	select {
	case err := <-looked:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("foreign observation holds Store state lock")
	}
	closed := make(chan error, 1)
	go func() { closed <- store.Close() }()
	select {
	case err := <-closed:
		t.Error("Close returned before original observer joined", err)
	case <-time.After(20 * time.Millisecond):
	}
	if other, err := reachability.OpenStore(config); err == nil {
		t.Error("Close released original lease before publication join")
		_ = other.Close()
	}
	close(resume)
	if err := <-published; err == nil {
		t.Error("publication revived after Close denial")
	}
	if err := <-closed; err != nil {
		t.Error(err)
	}
	if err := store.Close(); err != nil {
		t.Error(err)
	}
	other, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, outcome, err := other.Lookup(target, profile, at); err != nil || outcome != reachability.Stale {
		t.Fatal("denied pre-effect publication persisted", outcome, err)
	}
}

func TestStoreOriginalCustodyLossCannotReviveAfterRestoration(t *testing.T) {
	for _, object := range []string{"root", "records", ".ardents-reachability-store-v3", "record"} {
		t.Run(object, func(t *testing.T) {
			raw, target, network, profile, at := signedDescriptor(3)
			root := filepath.Join(t.TempDir(), "store")
			config := reachability.StoreConfig{Root: root, Network: network}
			store, err := reachability.OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			path := root
			if object != "root" {
				path = filepath.Join(root, object)
			}
			if object == "record" {
				entries, err := os.ReadDir(filepath.Join(root, "records"))
				if err != nil || len(entries) != 1 {
					t.Fatal(entries, err)
				}
				path = filepath.Join(root, "records", entries[0].Name())
			}
			retained := path + ".retained"
			if err := os.Rename(path, retained); err != nil {
				// Windows retained handles may prevent physical renaming. Linux
				// must actually perform every mutation and detect original loss.
				windowsPinnedDirectory := (object == "root" || object == "records") && errors.Is(err, syscall.Errno(5))
				if runtime.GOOS == "windows" && (errors.Is(err, syscall.Errno(32)) || windowsPinnedDirectory) {
					if err := store.Check(); err != nil {
						t.Fatal("blocked rename damaged original", err)
					}
					if got, outcome, err := store.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
						t.Fatal("native prevented mutation damaged original proof", outcome, err)
					}
					return
				}
				t.Fatal(err)
			}
			first := store.Check()
			if first == nil {
				t.Fatal("missing original object accepted")
			}
			if err := os.Rename(retained, path); err != nil {
				t.Fatal(err)
			}
			if err := store.Check(); !errors.Is(err, first) {
				t.Fatal("restored path revived original", err, first)
			}
			if _, _, err := store.Lookup(target, profile, at); !errors.Is(err, first) {
				t.Fatal("failed original returned proof", err)
			}
			if _, err := reachability.OpenStore(config); err == nil {
				t.Fatal("failed original lost exclusive lease")
			}
			if err := store.Close(); err != nil {
				t.Fatal("operational failure replaced physical release result", err)
			}
			fresh, err := reachability.OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			defer fresh.Close()
			if got, outcome, err := fresh.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
				t.Fatal("fresh reopen did not independently verify floor", outcome, err)
			}
		})
	}
}

func TestStoreRequestRefusalDoesNotRetireOriginal(t *testing.T) {
	raw, _, network, profile, at := signedDescriptor(3)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	refusal := errors.New("original request canceled")
	if _, err := store.Publish(raw, profile, at, func() error { return refusal }); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	if outcome, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil || outcome != reachability.Accepted {
		t.Fatal(outcome, err)
	}
}

func TestStoreDelayedPublicationUsesLatestCommittedFloor(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	root := t.TempDir()
	config := reachability.StoreConfig{Root: root, Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	higher := resignDescriptor(raw, func(b []byte) { binary.BigEndian.PutUint64(b[162:170], binary.BigEndian.Uint64(raw[162:170])+1) })
	entered, resume := make(chan struct{}), make(chan struct{})
	delayed := make(chan error, 1)
	checks := 0
	go func() {
		_, err := store.Publish(raw, profile, at, func() error {
			checks++
			if checks == 1 {
				close(entered)
				<-resume
			}
			return nil
		})
		delayed <- err
	}()
	<-entered
	if outcome, err := store.Publish(higher, profile, at, func() error { return nil }); err != nil || outcome != reachability.Accepted {
		close(resume)
		<-delayed
		t.Fatal(outcome, err)
	}
	close(resume)
	if err := <-delayed; err == nil {
		t.Fatal("delayed lower revision overwrote latest floor")
	}
	if got, outcome, err := store.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, higher) {
		t.Fatal("latest floor lost", outcome, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	fresh, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if got, outcome, err := fresh.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, higher) {
		t.Fatal("latest durable floor lost", outcome, err)
	}
}

func TestStoreLateNativeFailureSurvivesOriginalDenial(t *testing.T) {
	raw, target, network, profile, at := signedDescriptor(3)
	root := t.TempDir()
	config := reachability.StoreConfig{Root: root, Network: network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	entered, resume := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	checks := 0
	go func() {
		_, err := store.Publish(raw, profile, at, func() error {
			checks++
			if checks == 3 {
				close(entered)
				<-resume
			}
			return nil
		})
		finished <- err
	}()
	<-entered
	notice := errors.New("original Node retirement notice")
	store.Deny(notice)
	if store.OperationalFailure() != nil {
		t.Fatal("caller denial became a native Store failure")
	}
	marker := filepath.Join(root, ".ardents-reachability-store-v3")
	original, err := os.ReadFile(marker)
	if err != nil {
		close(resume)
		<-finished
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("foreign marker\n"), 0600); err != nil {
		close(resume)
		<-finished
		t.Fatal(err)
	}
	close(resume)
	if err := <-finished; err == nil || errors.Is(err, notice) {
		t.Fatal("original denial masked actual in-flight native failure", err)
	}
	cause := store.Check()
	if cause == nil || cause == notice {
		t.Fatal("native owner failure replaced by caller denial", cause)
	}
	if err := os.WriteFile(marker, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Check(); !errors.Is(err, cause) {
		t.Fatal("restoration revived failed original", err, cause)
	}
	if err := store.Close(); err != nil {
		t.Fatal("operational failure replaced physical release", err)
	}
	if store.OperationalFailure() != cause {
		t.Fatal("joined Close lost original native cause")
	}
	fresh, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if got, outcome, err := fresh.Lookup(target, profile, at); err != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
		t.Fatal("denied ACK discarded committed floor", outcome, err)
	}
}
