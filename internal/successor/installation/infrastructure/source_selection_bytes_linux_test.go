package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
)

// This native byte reader supplies no selection, history, Release or runtime
// authority. Repeated complete readback must not allocate another artifact-sized
// copy; lineage plus the no-GC descriptor oracle otherwise exhausts its stand.
func TestSourceSelectionReadbackAllocationsDoNotScaleWithArtifact(t *testing.T) {
	raw := bytes.Repeat([]byte("original"), 512<<10)
	path := filepath.Join(t.TempDir(), "program")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	original := &sourceFile{file: file, raw: raw}
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range 16 {
		if err := observeSourceSelectionBytes(t.Context(), original); err != nil {
			t.Fatal(err)
		}
	}
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("sixteen full %d-byte observations allocated %d bytes without GC", len(raw), allocated)
	if allocated > 2<<20 {
		t.Fatal("readback allocated artifact-sized copies", allocated)
	}
}

func TestSourceSelectionReadbackChecksAllBytesEOFAndCancellation(t *testing.T) {
	raw := bytes.Repeat([]byte("unchanged"), 16<<10)
	for _, fault := range []string{"none", "middle", "last", "append", "truncate", "cancel", "closed"} {
		t.Run(fault, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input")
			body := bytes.Clone(raw)
			switch fault {
			case "middle":
				body[len(body)/2] ^= 1
			case "last":
				body[len(body)-1] ^= 1
			case "append":
				body = append(body, 1)
			case "truncate":
				body = body[:len(body)-1]
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if fault == "cancel" {
				cancel()
			}
			if fault == "closed" {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			err = observeSourceSelectionBytes(ctx, &sourceFile{file: file, raw: raw})
			if fault == "none" && err != nil || fault != "none" && err == nil {
				t.Fatal("original byte/EOF check", err)
			}
			if fault == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("original cancellation lost", err)
			}
		})
	}
}
