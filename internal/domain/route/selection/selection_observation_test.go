package selection

import (
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

func TestSelectionObservesNetworkOutsideBothStateLocks(t *testing.T) {
	root := ownedRootFixture(t)
	view := installationView(t, 1)
	var owner *Owner
	var observations int
	opened, err := Open(Config{
		EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3,
		Current: func() (network.RuntimeView, error) {
			if owner != nil {
				observations++
				if !owner.mu.TryLock() {
					return network.RuntimeView{}, errors.New("observer holds role state lock")
				}
				owner.mu.Unlock()
				if !owner.entries.mu.TryLock() {
					return network.RuntimeView{}, errors.New("observer holds Entry state lock")
				}
				owner.entries.mu.Unlock()
			}
			return view, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	owner = opened
	defer owner.Close()
	if _, err := owner.Select(); err != nil || observations != 4 {
		t.Fatalf("selection failed across the four observation boundaries: %d observations, %v", observations, err)
	}
}

func TestSelectionRefusesSuccessorNetworkGenerationMidSelection(t *testing.T) {
	for _, tc := range []struct {
		name               string
		changeAt           int
		interiorGeneration uint64
	}{
		{name: "before Interior write", changeAt: 2, interiorGeneration: 1},
		{name: "after Interior write", changeAt: 3, interiorGeneration: 2},
		{name: "during final Entry observation", changeAt: 4, interiorGeneration: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := ownedRootFixture(t)
			original := installationGenerationView(t, 1, 2)
			successor := installationGenerationView(t, 1, 9)
			var started bool
			var observations int
			owner, err := Open(Config{
				EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3,
				Current: func() (network.RuntimeView, error) {
					if started {
						observations++
						if observations >= tc.changeAt {
							return successor, nil
						}
					}
					return original, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			started = true
			leg, err := owner.Select()
			if err == nil || leg.Profile != (network.ProfileBinding{}) {
				t.Fatal("a changed generation supplied a leg for the original attempt")
			}
			if owner.entries.state.Generation != 2 || owner.interior.record.Generation != tc.interiorGeneration {
				t.Fatalf("generation change altered the effect boundary: Entry %d, Interior %d",
					owner.entries.state.Generation, owner.interior.record.Generation)
			}
			// A separate attempt can still use the retained history with a fresh,
			// coherent observation. The failed attempt grants no transport.
			wantEntry := owner.entries.state.Sets[closedAdjacentIndex(3)]
			leg, err = owner.Select()
			if err != nil || leg.Profile != successor.ProfileBinding() || leg.Entry.NodeID != wantEntry.Members[0].NodeID ||
				owner.entries.state.Generation != 2 || owner.interior.record.Generation != 2 {
				t.Fatal("fresh coherent attempt did not retain the selected history", err)
			}
		})
	}
}

func TestSelectionFinalObservationCannotIssueExpiredInterior(t *testing.T) {
	root := ownedRootFixture(t)
	original := installationView(t, 1)
	afterInteriorExpiry := installationGenerationViewAt(t, 1, 2, 30*time.Minute)
	var started bool
	var observations int
	owner, err := Open(Config{
		EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3,
		Current: func() (network.RuntimeView, error) {
			if started {
				observations++
				if observations == 4 {
					return afterInteriorExpiry, nil
				}
			}
			return original, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	started = true
	if leg, err := owner.Select(); err == nil || !leg.NotAfter.IsZero() || observations != 4 {
		t.Fatal("final trusted time did not refuse the expired Interior", observations, err)
	}
	if owner.interior.record.Generation != 2 || !owner.interior.record.Set.NotAfter.Equal(afterInteriorExpiry.ObservedAt()) {
		t.Fatal("refusal changed the original Interior generation or deadline")
	}
}

func TestSelectionCloseJoinsObservationAndPreservesCommittedFloors(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		held                                int
		entryGeneration, interiorGeneration uint64
	}{
		{name: "before Entry write", held: 1, entryGeneration: 1, interiorGeneration: 1},
		{name: "before Interior write", held: 2, entryGeneration: 2, interiorGeneration: 1},
		{name: "after both writes", held: 4, entryGeneration: 2, interiorGeneration: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := ownedRootFixture(t)
			view := installationView(t, 1)
			reached, resume := make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(resume) })
			var observing bool
			var calls int
			config := Config{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), Domain: 3,
				Current: func() (network.RuntimeView, error) {
					if observing {
						calls++
						if calls == tc.held {
							close(reached)
							<-resume
						}
					}
					return view, nil
				},
			}
			owner, err := Open(config)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			defer unblock()
			observing = true
			refusal := make(chan error, 1)
			go func() {
				leg, err := owner.Select()
				if leg.EntryMember != (network.Member{}) || leg.InteriorMember != (network.Member{}) {
					err = nil
				}
				refusal <- err
			}()
			select {
			case <-reached:
			case <-time.After(5 * time.Second):
				t.Fatal("selection did not enter the held observation")
			}
			// The channel establishes that the preceding writes have completed.
			wantEntry := owner.entries.state
			wantInterior := owner.interior.record
			if wantEntry.Generation != tc.entryGeneration || wantInterior.Generation != tc.interiorGeneration {
				t.Fatalf("held at the wrong durable boundary: Entry %d, Interior %d", wantEntry.Generation, wantInterior.Generation)
			}
			closed := make(chan error, 1)
			go func() { closed <- owner.Close() }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if owner.mu.TryLock() {
					sealed := owner.closed
					owner.mu.Unlock()
					if sealed {
						break
					}
				}
				if time.Now().After(deadline) {
					t.Fatal("Close did not seal while Network was held")
				}
				runtime.Gosched()
			}
			select {
			case err := <-closed:
				t.Fatalf("Close returned before the observer joined: %v", err)
			default:
			}
			if lease, err := acquireRetainedRootLease(config.InteriorRoot); err == nil {
				_ = lease.release()
				t.Fatal("Interior lease released before the observer joined")
			}
			unblock()
			select {
			case err := <-refusal:
				if err == nil {
					t.Fatal("sealed selection returned an accepting leg")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("selection did not join")
			}
			select {
			case err := <-closed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Close did not join")
			}
			observing = false
			reopened, err := OpenRetainedInstallation(InstallationConfig{EntryRoot: config.EntryRoot, Current: config.Current})
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			borrower, err := reopened.Borrow(RoleConfig{InteriorRoot: config.InteriorRoot, Domain: config.Domain})
			if err != nil {
				t.Fatal(err)
			}
			defer borrower.Close()
			if reopened.entries.state != wantEntry || borrower.interior.record != wantInterior {
				t.Fatal("refused selection changed the retained pair, deadline or generation")
			}
		})
	}
}
