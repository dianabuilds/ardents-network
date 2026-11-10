//go:build linux && network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These are independent file observers on the selected native fixture, never
// installed process/manager/Startup authority or a borrowed creator snapshot.
func TestForwardingRuntimeNamespaceNativeRefusalIsRetained(t *testing.T) {
	path := os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING")
	mode := os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT")
	if !validPath(path) || mode != "protected" && mode != "unprotected" {
		t.Fatal("invalid environment: selected native runtime selection required")
	}
	s, err := readRuntimeSelection(t.Context(), path)
	if mode == "unprotected" {
		if s != nil || !errors.Is(err, ErrBinding) {
			if s != nil {
				_ = s.close()
			}
			t.Fatal("writable immutable mount admitted independent observer", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	absent := []string{filepath.Join(s.files.sealed, "unexpected.service")}
	if err := checkStartupNamespace(s, absent); err != nil {
		t.Fatal("actual absent search entry refused", err)
	}
	present := []string{filepath.Join(s.files.sealed, programName)}
	if err := checkStartupNamespace(s, present); !errors.Is(err, ErrBinding) {
		t.Fatal("present entry accepted as absent", err)
	}
	if err := checkStartupNamespace(s, absent); !errors.Is(err, ErrBinding) {
		t.Fatal("later absent entry erased original refusal", err)
	}
	if err := s.close(); !errors.Is(err, ErrBinding) {
		t.Fatal("joined close erased original namespace refusal", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, err = readRuntimeSelection(ctx, path)
	if err != nil {
		t.Fatal("fresh independent observer", err)
	}
	defer s.close()
	// Cancel only after actual original-file I/O, before namespace acceptance.
	s.files.observe = func(phase string, _ *os.File) {
		if phase == "read" {
			cancel()
		}
	}
	if err := checkStartupNamespace(s, absent); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled physical observation accepted namespace", err)
	}
	if err := s.close(); !errors.Is(err, context.Canceled) {
		t.Fatal("joined close lost cancellation", err)
	}
}

func BenchmarkForwardingRuntimeNamespaceNative(b *testing.B) {
	path := os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING")
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		b.Fatal("invalid environment: protected native selection required")
	}
	s, err := readRuntimeSelection(b.Context(), path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := s.close(); err != nil {
			b.Fatal(err)
		}
	}()
	absent := []string{filepath.Join(s.files.sealed, "unexpected.service")}
	for _, name := range []string{"nested", "compound"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if name == "nested" {
					// Original physical bracket, for measurement only. This
					// result is never passed to an installed Startup or effect.
					err = s.check()
					if err == nil {
						_, err = os.Lstat(absent[0])
						if errors.Is(err, os.ErrNotExist) {
							err = s.check()
						} else {
							err = errors.Join(ErrBinding, err)
						}
					}
				} else {
					err = checkStartupNamespace(s, absent)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
