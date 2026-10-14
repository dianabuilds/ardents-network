//go:build linux

package introduction

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryNativeOriginalLossAndRestorationNeverRevives(t *testing.T) {
	for _, member := range []string{"root", slotLeaseName, slotHistoryName} {
		t.Run(member, func(t *testing.T) {
			history, _, root, binding := newHistoryFixture(t)
			now := time.Now().UTC().Truncate(time.Second)
			if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); err != nil {
				t.Fatal(err)
			}
			path := root
			if member != "root" {
				path = filepath.Join(root, member)
			}
			moved := path + ".held"
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			first := history.Check()
			if first == nil {
				t.Fatal("lost original custody accepted")
			}
			if err := os.Rename(moved, path); err != nil {
				t.Fatal(err)
			}
			if history.Check() != first || history.Failure() != first {
				t.Fatal("restoration revived or replaced original cause")
			}
			if err := history.Claim([32]byte{8}, now.Add(time.Minute), now); err != first {
				t.Fatal("failed original wrote after restoration", err)
			}
			if err := history.Close(); err == nil {
				t.Fatal("Close lost native failure")
			}
			// A separate strict reopen can verify retained bytes and floors; it
			// cannot revive the old lifetime or reclaim its already durable slot.
			reopened, err := OpenHistory(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.Claim([32]byte{7}, now.Add(time.Minute), now); err == nil {
				t.Fatal("separate reopen reset non-reclaim floor")
			}
		})
	}
}

func TestHistoryNativeIdenticalSubstitutionAndRestoredBytesRefuse(t *testing.T) {
	for _, fault := range []string{"identical snapshot", "identical lease", "same inode restored bytes"} {
		t.Run(fault, func(t *testing.T) {
			history, _, root, _ := newHistoryFixture(t)
			path := filepath.Join(root, slotHistoryName)
			if fault == "identical lease" {
				path = filepath.Join(root, slotLeaseName)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if fault == "same inode restored bytes" {
				changed := bytes.Clone(raw)
				changed[0] ^= 1
				if err := os.WriteFile(path, changed, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				temporary := filepath.Join(root, ".foreign")
				if err := os.WriteFile(temporary, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(temporary, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := history.Check(); err == nil {
				t.Fatal("foreign native history accepted identical/restored bytes")
			}
		})
	}
}

func TestRegistryStopDoesNotWaitForOriginalHistoryCommit(t *testing.T) {
	history, _, root, binding := newHistoryFixture(t)
	registry, err := NewRegistry(history)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	capacity, err := registry.Reserve(now)
	if err != nil {
		t.Fatal(err)
	}
	defer capacity.Release()
	entered, release := make(chan struct{}), make(chan struct{})
	files := history.store.(*historyFiles)
	files.replace = func(path string, raw []byte) (*os.File, error) {
		close(entered)
		<-release
		return replaceHistory(path, raw) // actual write/sync/rename/directory-sync
	}
	claimed := make(chan error, 1)
	go func() {
		_, err := registry.Register(capacity, Request{Nonce: [32]byte{1}, Slot: [32]byte{2}, Revision: 1, Expiry: now.Add(time.Minute)}, [32]byte{3}, 1<<20, now.Add(time.Minute), 0, now)
		claimed <- err
	}()
	<-entered
	stopped := make(chan struct{})
	go func() { registry.Stop(nil); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("Stop waited for physical History I/O")
	}
	if err := registry.Check(binding); err == nil {
		t.Fatal("stopped original accepted while write held")
	}
	closed := make(chan error, 1)
	go func() { closed <- registry.Close() }()
	select {
	case <-closed:
		close(release)
		t.Fatal("History released before original writer joined")
	case <-time.After(20 * time.Millisecond):
	}
	if _, err := OpenHistory(root, binding); err == nil {
		close(release)
		t.Fatal("native lease returned before writer join")
	}
	close(release)
	if err := <-claimed; err == nil {
		t.Fatal("late durable write published registration")
	}
	if err := <-closed; err != nil {
		t.Fatal("local denial became native cleanup failure", err)
	}
	reopened, err := OpenHistory(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Claim([32]byte{2}, now.Add(time.Minute), now); err == nil {
		t.Fatal("denial refunded a durable late claim")
	}
}

func TestRegistryOnlyOneOriginalReceiverBorrowAndJoinedNativeLoss(t *testing.T) {
	history, _, root, binding := newHistoryFixture(t)
	registry, err := NewRegistry(history)
	if err != nil {
		t.Fatal(err)
	}
	foreign := binding
	foreign.ReceiverDutyGeneration++
	if err := registry.BindReceiver(foreign); err == nil {
		t.Fatal("foreign receiver acquired original History")
	}
	if err := registry.BindReceiver(binding); err != nil {
		t.Fatal(err)
	}
	if err := registry.BindReceiver(binding); err == nil {
		t.Fatal("original History acquired a second Receiver")
	}
	if err := registry.Check(binding); err != nil {
		t.Fatal("borrow refusal retired healthy owner", err)
	}
	registry.Stop(nil)
	path := filepath.Join(root, slotHistoryName)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	first := registry.Close()
	if first == nil || registry.Failure() == nil || registry.Close() != first {
		t.Fatal("joined close lost native loss after local denial", first)
	}
	if _, err := OpenHistory(root, binding); err == nil {
		t.Fatal("lost snapshot was reconstructed after join")
	}
}

func TestHistoryOriginalReplacementCannotAdoptForeignInodeOrRestoredRoot(t *testing.T) {
	for _, fault := range []string{"snapshot substituted", "root moved and restored"} {
		t.Run(fault, func(t *testing.T) {
			history, _, root, binding := newHistoryFixture(t)
			now := time.Now().UTC().Truncate(time.Second)
			if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); err != nil {
				t.Fatal(err)
			}
			files := history.store.(*historyFiles)
			files.replace = func(path string, raw []byte) (*os.File, error) {
				snapshot, err := replaceHistory(path, raw)
				if err != nil {
					return nil, err
				}
				if fault == "snapshot substituted" {
					foreign := filepath.Join(root, ".foreign-snapshot")
					if err := os.WriteFile(foreign, raw, 0600); err != nil {
						return nil, errors.Join(err, snapshot.Close())
					}
					if err := os.Rename(foreign, filepath.Join(root, slotHistoryName)); err != nil {
						return nil, errors.Join(err, snapshot.Close())
					}
				} else {
					moved := root + ".moved"
					if err := os.Rename(root, moved); err != nil {
						return nil, errors.Join(err, snapshot.Close())
					}
					if err := os.Rename(moved, root); err != nil {
						return nil, errors.Join(err, snapshot.Close())
					}
				}
				return snapshot, nil // only the actual original staged inode
			}
			first := history.Claim([32]byte{8}, now.Add(time.Minute), now.Add(time.Second))
			if first == nil || history.Check() != first || history.Failure() != first {
				t.Fatal("lawful replacement adopted foreign inode or erased root-loss evidence", first)
			}
			if err := history.Close(); !errors.Is(err, first) {
				t.Fatal("Close lost original replacement failure", err)
			}
			reopened, err := OpenHistory(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.Claim([32]byte{8}, now.Add(time.Minute), now.Add(time.Second)); err == nil {
				t.Fatal("separate full verification refunded the visible late claim")
			}
		})
	}
}
