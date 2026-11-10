//go:build network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The original protected file fixture supplies actual bytes and descriptors,
// never a running Startup, manager, process, Guard or State authority.
func TestForwardingStartupQueuedFileObservationNative(t *testing.T) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		t.Fatal("invalid environment: protected non-root file fixture required")
	}
	for _, loseDescriptor := range []bool{false, true} {
		name := "caller"
		if loseDescriptor {
			name = "caller-and-original-descriptor"
		}
		t.Run(name, func(t *testing.T) {
			parent, cancel := context.WithCancel(t.Context())
			ctx, stop := context.WithCancel(parent)
			defer cancel()
			defer stop()
			selection, err := readRuntimeSelection(ctx, os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
			if err != nil {
				t.Fatal(err)
			}
			s := &startup{ctx: ctx, parent: parent, cancel: stop, ready: true, selection: selection, closeDone: make(chan struct{})}
			h := &Startup{original: s}
			entered, release := make(chan struct{}), make(chan struct{})
			var held, released sync.Once
			releaseRead := func() { released.Do(func() { close(release) }) }
			defer func() {
				releaseRead()
				cancel()
				if err := h.Close(); err != nil {
					t.Log("original queue/file Close retained:", err)
				}
			}()
			selection.files.observe = func(stage string, _ *os.File) {
				if stage == "startup-first-access" {
					held.Do(func() { close(entered); <-release })
				}
			}
			var observations atomic.Int32
			observe := func() error {
				observations.Add(1)
				return selection.observeStartupFiles(func() error {
					return selection.observeStartupPaths([]string{filepath.Join(selection.files.sealed, "unexpected.service")})
				})
			}
			checks := make([]chan error, 3)
			for i := range checks {
				checks[i] = make(chan error, 1)
				go func() { checks[i] <- s.checkObservation(observe) }()
				if i == 0 {
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("actual original access observation not reached")
					}
				}
			}
			waitQueuedStartupNative(t, "two requests admitted behind actual I/O", func() bool {
				s.mu.Lock()
				defer s.mu.Unlock()
				return len(s.checks) == 2
			})
			if loseDescriptor {
				program := selection.files.files[filepath.Join(selection.files.sealed, programName)]
				if err := program.file.Close(); err != nil {
					t.Fatal("original descriptor loss control", err)
				}
			}
			cancel()
			closed := make(chan error, 1)
			go func() { closed <- h.Close() }()
			waitQueuedStartupNative(t, "Close denied original admission", func() bool {
				s.mu.Lock()
				defer s.mu.Unlock()
				return s.closed
			})
			select {
			case err := <-closed:
				t.Fatal("Close released held original file custody", err)
			default:
			}
			for _, done := range checks {
				select {
				case err := <-done:
					t.Fatal("waiter escaped actual held observation", err)
				default:
				}
			}
			releaseRead()
			for _, done := range checks {
				if err := <-done; err != context.Canceled {
					t.Fatal("original caller identity lost", err)
				}
			}
			joined := <-closed
			copied := *h
			if !errors.Is(joined, context.Canceled) || loseDescriptor && !errors.Is(joined, os.ErrClosed) || copied.Close() != joined {
				t.Fatal("original joined causes lost", joined)
			}
			if err := s.checkObservation(observe); err == nil || observations.Load() != 1 {
				t.Fatal("late work revived the original file observer", observations.Load(), err)
			}
		})
	}
}

func waitQueuedStartupNative(t *testing.T, boundary string, reached func() bool) {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for !reached() {
		if time.Now().After(end) {
			t.Fatal("native observation boundary unavailable:", boundary)
		}
		runtime.Gosched()
	}
}

// This is a finite model of serial calls and three simultaneously offered file
// observations. It measures neither actual Node timers nor installed pressure.
func BenchmarkForwardingStartupQueuedFilesNative(b *testing.B) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		b.Fatal("invalid environment: protected non-root file fixture required")
	}
	selection, err := readRuntimeSelection(b.Context(), os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := selection.close(); err != nil {
			b.Fatal(err)
		}
	}()
	absent := []string{filepath.Join(selection.files.sealed, "unexpected.service")}
	for _, mode := range []string{"serial-direct", "serial-queued", "burst-direct", "burst-queued"} {
		b.Run(mode, func(b *testing.B) {
			ctx, cancel := context.WithCancel(b.Context())
			defer cancel()
			s := &startup{ctx: ctx, parent: b.Context(), cancel: cancel, ready: true}
			var observations atomic.Int64
			observe := func() error {
				observations.Add(1)
				return selection.observeStartupFiles(func() error {
					for range 3 {
						if err := selection.observeStartupPaths(absent); err != nil {
							return err
						}
					}
					return nil
				})
			}
			check := func() error {
				if mode == "serial-queued" || mode == "burst-queued" {
					return s.checkObservation(observe)
				}
				// Exact previous per-call admission/I/O/retained-failure order,
				// over the same actual file observation used by the queue.
				if err := s.begin(); err != nil {
					return err
				}
				defer s.uses.Done()
				s.ioMu.Lock()
				defer s.ioMu.Unlock()
				if err := observe(); err != nil {
					return s.record(err)
				}
				return s.local()
			}
			b.ReportAllocs()
			for b.Loop() {
				if mode == "serial-direct" || mode == "serial-queued" {
					if err := check(); err != nil {
						b.Fatal(err)
					}
					continue
				}
				gate := make(chan struct{})
				results := make(chan error, 3)
				var joined sync.WaitGroup
				joined.Add(3)
				for range 3 {
					go func() { defer joined.Done(); <-gate; results <- check() }()
				}
				close(gate)
				joined.Wait()
				close(results)
				for err := range results {
					if err != nil {
						b.Fatal(err)
					}
				}
			}
			s.uses.Wait()
			b.ReportMetric(float64(observations.Load())/float64(b.N), "rounds/op")
		})
	}
}
