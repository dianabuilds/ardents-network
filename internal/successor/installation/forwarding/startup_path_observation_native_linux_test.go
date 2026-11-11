//go:build network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Actual original descriptor/caller loss during native reads. These component
// observations create no Startup, manager, process or domain authority.
func TestForwardingStartupPathsNativeInterruptedObservation(t *testing.T) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		t.Fatal("invalid environment: protected non-root file fixture required")
	}
	for _, cut := range []string{"startup-first-access", "startup-absence", "startup-final-access"} {
		for _, fault := range []string{"closed-file", "caller"} {
			t.Run(cut+"/"+fault, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				s, err := readRuntimeSelection(ctx, os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
				if err != nil {
					t.Fatal("independent original file observer", err)
				}
				closed := false
				defer func() {
					if !closed {
						if err := s.close(); err != nil {
							t.Error(err)
						}
					}
				}()
				fired := false
				var injection error
				s.files.observe = func(phase string, _ *os.File) {
					if phase != cut || fired {
						return
					}
					fired = true
					if fault == "caller" {
						cancel()
					} else {
						injection = s.files.files[filepath.Join(s.files.sealed, programName)].file.Close()
					}
				}
				err = s.observeStartupFiles(func() error {
					return s.observeStartupPaths([]string{filepath.Join(s.files.sealed, "unexpected.service")})
				})
				if !fired || injection != nil {
					t.Fatal("actual native interruption not established", injection)
				}
				if fault == "caller" {
					if err != context.Canceled {
						t.Fatal("original caller cancellation replaced", err)
					}
				} else if !errors.Is(err, os.ErrClosed) || !errors.Is(err, ErrBinding) {
					t.Fatal("lost original descriptor admitted or physical error lost", err)
				}
				first := err
				if next := s.observeStartupFiles(func() error {
					t.Error("denied original observer ran late work")
					return nil
				}); next != first {
					t.Fatal("later observation replaced original refusal", next)
				}
				joined := s.close()
				closed = true
				if !errors.Is(joined, first) || fault == "closed-file" && !errors.Is(joined, os.ErrClosed) {
					t.Fatal("joined Close lost original or physical failure", joined)
				}
			})
		}
	}
}
