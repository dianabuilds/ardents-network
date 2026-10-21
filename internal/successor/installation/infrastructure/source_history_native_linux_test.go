//go:build linux && network_signing_native

package infrastructure

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Physical history custody and private commitment only. An empty Release
// verifier grants no authorization; the compiled initial installer separately
// consumes genuinely authenticated complete floors and the exact artifact.
func nativeHistoryCustody(t *testing.T, ctx context.Context) (*sourceHistory, *release.Verifier) {
	t.Helper()
	births, path := nativeSourceBirths(t, ctx)
	t.Cleanup(func() { _ = births.close() })
	root := filepath.Join(filepath.Dir(path), "release-history")
	v, err := release.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	})
	h, err := openSourceHistory(ctx, v, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.close() })
	return h, v
}

func TestSourceStoppedNativeHistoryOriginalDirectoryAndVerifier(t *testing.T) {
	t.Run("replacement-before-pins", func(t *testing.T) {
		births, path := nativeSourceBirths(t, t.Context())
		defer births.close()
		root := filepath.Join(filepath.Dir(path), "release-history")
		v, err := release.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		defer v.Close()
		if err := os.Rename(root, root+".original"); err != nil {
			t.Fatal(err)
		}
		// Both genuine native owners have equal empty floors. This physical
		// control grants no authorization; Release's signed complete-history
		// control separately exercises the same substitution after evaluation.
		replacement, err := release.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		defer replacement.Close()
		h, err := openSourceHistory(t.Context(), v, root)
		if h != nil {
			_ = h.close()
			t.Fatal("consumer retained replacement as verifier's original history")
		}
		if !errors.Is(err, release.ErrTrustUnavailable) {
			t.Fatal("pre-pin original verifier mismatch lost", err)
		}
		if err := v.Close(); !errors.Is(err, release.ErrTrustUnavailable) {
			t.Fatal("original verifier failure lost during join", err)
		}
	})
	for _, control := range []string{"commitment", "foreign-verifier", "directory", "access"} {
		t.Run(control, func(t *testing.T) {
			h, v := nativeHistoryCustody(t, t.Context())
			if err := h.check(v); err != nil {
				t.Fatal(err)
			}
			switch control {
			case "commitment":
				binding := sourceBindingVector()
				got, err := h.commitment(v, binding)
				native := h.identities[h.path].Sys().(*syscall.Stat_t)
				// Independent fixed field order and literal domain separator.
				body := fmt.Sprintf(`{"schema":"ardents-source-initial-history-v1","binding_sha256":"%x","history":"%s","device":%d,"inode":%d,"mode":448,"uid":0,"gid":0}`+"\n",
					sha256.Sum256(binding), h.path, uint64(native.Dev), native.Ino)
				if err != nil || got != fmt.Sprintf("%x", sha256.Sum256([]byte(body))) {
					t.Fatal("private original-history commitment differs from independent vector", err)
				}
				return
			case "foreign-verifier":
				other, err := release.Open(filepath.Join(filepath.Dir(h.path), "other-history"))
				if err != nil {
					t.Fatal(err)
				}
				defer other.Close()
				if err := h.check(other); !errors.Is(err, release.ErrTrustUnavailable) {
					t.Fatal("foreign history owner accepted", err)
				}
			case "directory":
				if err := os.Rename(h.path, h.path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(h.path, 0700); err != nil {
					t.Fatal(err)
				}
			case "access":
				if err := os.Chmod(h.path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			first := h.check(v)
			if first == nil || h.check(v) != first || h.close() == nil {
				t.Fatal("changed original history or first failure was lost", first)
			}
		})
	}
}

func TestSourceStoppedNativeHistoryHeldObservationAndClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h, v := nativeHistoryCustody(t, ctx)
	entered, resume := make(chan struct{}), make(chan struct{})
	h.observed = func() { close(entered); <-resume }
	checked := make(chan error, 1)
	go func() { checked <- h.check(v) }()
	<-entered
	cancel()
	if err := h.dirs[h.path].Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-checked:
		t.Fatal("actual held observation returned before join")
	default:
	}
	close(resume)
	if err := <-checked; !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
		t.Fatal("original cancellation or independent native failure lost", err)
	}
	closed := h.close()
	if !errors.Is(closed, context.Canceled) || !errors.Is(closed, os.ErrClosed) || h.close() != closed {
		t.Fatal("late close failure or retained result lost", closed)
	}
}
