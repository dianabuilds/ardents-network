//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// This fixture isolates native file custody. It grants no healthy State,
// process protection, credential, transport or installed admission.
func sourceCustodyFixture(t *testing.T) (string, sourceServePlan) {
	t.Helper()
	parent := t.TempDir()
	paths := make([]string, 4)
	for i, name := range []string{"plan", "certificate", "key", "ca"} {
		dir := filepath.Join(parent, name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		paths[i] = filepath.Join(dir, "input")
		if err := os.WriteFile(paths[i], []byte("bounded original bytes"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan := sourceServePlan{Network: &networkAuthorityPlan{Root: filepath.Join(parent, "state"), NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{2}, ClockObservationFile: filepath.Join(parent, "clock")},
		ProcessProfile: "ardents-source-process-v1", ParticipationRoot: filepath.Join(parent, "participation"), Address: "127.0.0.1:12345", Certificate: paths[1], PrivateKey: paths[2], ClientRoot: paths[3], ClientPins: [][32]byte{{3}}}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], raw, 0600); err != nil {
		t.Fatal(err)
	}
	return paths[0], plan
}

func TestSourceOriginalInputsRetainEveryFileAndAncestor(t *testing.T) {
	for _, name := range []string{"plan", "certificate", "key", "ca"} {
		for _, fault := range []string{"replace", "mutate", "unlink", "access", "ancestor-replace", "ancestor-access"} {
			t.Run(name+"/"+fault, func(t *testing.T) {
				path, plan := sourceCustodyFixture(t)
				selected := map[string]string{"plan": path, "certificate": plan.Certificate, "key": plan.PrivateKey, "ca": plan.ClientRoot}[name]
				inputs, _, err := acquireSourceInputs(t.Context(), path)
				if err != nil {
					t.Fatal(err)
				}
				defer inputs.Close()
				original, err := os.ReadFile(selected)
				if err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "replace":
					if err = os.Rename(selected, selected+".original"); err == nil {
						err = os.WriteFile(selected, original, 0600)
					}
				case "mutate":
					changed := append([]byte(nil), original...)
					changed[len(changed)-1] ^= 1
					err = os.WriteFile(selected, changed, 0600)
				case "unlink":
					err = os.Remove(selected)
				case "access":
					err = os.Chmod(selected, 0400)
				case "ancestor-replace":
					parent := filepath.Dir(selected)
					if err = os.Rename(parent, parent+".original"); err == nil {
						if err = os.Mkdir(parent, 0700); err == nil {
							err = os.WriteFile(selected, original, 0600)
						}
					}
				case "ancestor-access":
					err = os.Chmod(filepath.Dir(selected), 0500)
					defer os.Chmod(filepath.Dir(selected), 0700)
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = inputs.Check(); !errors.Is(err, errSourceInput) {
					t.Fatal("original loss accepted", err)
				}
				// Restore pathname bytes/access; denial remains original and permanent.
				if fault == "ancestor-access" {
					if err := os.Chmod(filepath.Dir(selected), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(selected, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(selected, 0600); err != nil {
					t.Fatal(err)
				}
				if next := inputs.Check(); next != err || inputs.ctx.Err() != context.Canceled {
					t.Fatal("restoration revived original lifetime", next)
				}
			})
		}
	}
}

func TestSourceInputCloseDeniesBeforeJoiningOriginalUses(t *testing.T) {
	path, _ := sourceCustodyFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	inputs, _, err := acquireSourceInputs(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	copied := inputs
	// Hold the native-observation serializer after a genuine original read.
	// Cancellation must not acquire it; Close must retain descriptors until the
	// already admitted use finishes. This is a scheduling control, not State.
	inputs.ioMu.Lock()
	checked := make(chan error, 1)
	go func() { checked <- copied.Check() }()
	cancel()
	closed := make(chan error, 1)
	go func() { closed <- copied.Close() }()
	<-inputs.ctx.Done()
	if err := inputs.Check(); err == nil {
		t.Fatal("canceled original accepted work")
	}
	inputs.ioMu.Unlock()
	if err := <-checked; err == nil {
		t.Fatal("late observation accepted canceled generation")
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if err := inputs.Close(); err != nil {
		t.Fatal("copied Close disagrees", err)
	}
	for _, f := range inputs.files {
		if _, err := f.file.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Fatal("original descriptor survived completed Close", err)
		}
		for _, b := range f.raw {
			if b != 0 {
				t.Fatal("original bytes survived joined Close")
			}
		}
	}
}

func TestSourceConcurrentInputChecksAndCopiedCloseRetainPhysicalFailure(t *testing.T) {
	path, _ := sourceCustodyFixture(t)
	inputs, _, err := acquireSourceInputs(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	copied := inputs
	if err := inputs.files[2].file.Close(); err != nil {
		t.Fatal(err)
	}
	var joined sync.WaitGroup
	for range 8 {
		joined.Go(func() { _ = copied.Check() })
	}
	first := inputs.Close()
	joined.Wait()
	if !errors.Is(first, os.ErrClosed) || copied.Close() != first {
		t.Fatal("original late native failure lost or copied Close changed", first)
	}
}

func TestSourceInputCustodyDoesNotFreezeLiveClockEvidence(t *testing.T) {
	path, plan := sourceCustodyFixture(t)
	inputs, _, err := acquireSourceInputs(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer inputs.Close()
	if err := os.WriteFile(plan.Network.ClockObservationFile, []byte("external live evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(plan.Network.ClockObservationFile, now, now); err != nil {
		t.Fatal(err)
	}
	if err := inputs.Check(); err != nil {
		t.Fatal("fixed input custody took over live clock policy", err)
	}
}

func TestSourceOriginalNativeReadJoinsAfterCancellationAndLateClose(t *testing.T) {
	path, _ := sourceCustodyFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var selected atomic.Pointer[os.File]
	var hold, once atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	inputs, _, err := acquireObservedSourceInputs(ctx, path, func(phase string, f *os.File) {
		if phase == "check" && f == selected.Load() && hold.Load() && once.CompareAndSwap(false, true) {
			close(entered) // Original ReadAt has really returned; retain its use.
			<-release
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	selected.Store(inputs.files[2].file)
	hold.Store(true)
	checked := make(chan error, 1)
	go func() { checked <- inputs.Check() }()
	<-entered
	cancel()
	<-inputs.ctx.Done()
	closed := make(chan error, 1)
	go func() { closed <- inputs.Close() }()
	if err := inputs.Check(); err == nil {
		t.Fatal("short denial waited for held native use")
	}
	for _, f := range inputs.files {
		if _, err := f.file.Stat(); err != nil {
			t.Fatal("input custody released before original native use joined", err)
		}
	}
	// An independent real Close failure arrives after the original cancellation.
	if err := selected.Load().Close(); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-checked; !errors.Is(err, context.Canceled) || !errors.Is(err, errSourceInput) {
		t.Fatal("late independent native observation or original cancellation was lost", err)
	}
	first := <-closed
	if !errors.Is(first, os.ErrClosed) || inputs.Close() != first {
		t.Fatal("late native Close failure erased by original cancellation", first)
	}
}

func TestSourcePartialInputAcquisitionJoinsAndRefusesLateBytes(t *testing.T) {
	for input := range 4 {
		t.Run([]string{"plan", "certificate", "key", "ca"}[input], func(t *testing.T) {
			path, _ := sourceCustodyFixture(t)
			var files []*os.File
			inputs, _, err := acquireObservedSourceInputs(t.Context(), path, func(phase string, f *os.File) {
				if phase != "read" {
					return
				}
				files = append(files, f)
				if len(files) == input+1 {
					if err := f.Close(); err != nil {
						t.Fatal(err)
					}
				}
			})
			if inputs.sourceInputLifetime != nil || !errors.Is(err, errSourceInput) || !errors.Is(err, os.ErrClosed) {
				t.Fatal("partial acquisition adopted late input or dropped close failure", err)
			}
			for _, f := range files {
				if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatal("partial creator left native input unjoined", err)
				}
			}
		})
	}
}
