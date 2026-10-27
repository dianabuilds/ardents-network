//go:build linux && network_signing_native

package duty

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// These controls use genuine local producer storage and leases. Supplied
// participation facts grant no signed State, Node runtime or installed join.
func nativeParticipationInspection(t *testing.T) Config {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root participation inspection driver required")
	}
	parent := t.TempDir()
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: private native ext4 TMPDIR required", err)
	}
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("invalid environment: original root-controlled ancestors required", path, err)
		}
		if path == "/" {
			break
		}
	}
	cfg := Config{Root: filepath.Join(parent, "participation"), Clock: time.Now, Create: true}
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Create = false
	return cfg
}

func participationInspectionImages(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string][]byte{}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = raw
	}
	return result
}

func unchangedParticipationInspection(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := participationInspectionImages(t, root)
	if len(before) != len(after) {
		t.Fatal("inspection changed retained participation inventory")
	}
	for name, raw := range before {
		if actual, found := after[name]; !found || !bytes.Equal(raw, actual) {
			t.Fatal("inspection changed retained participation bytes", name)
		}
	}
}

func TestNodeInspectionNativeOriginalHistoryAndLease(t *testing.T) {
	cfg := nativeParticipationInspection(t)
	before := participationInspectionImages(t, cfg.Root)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	inspection, err := OpenNodeInspection(ctx, cfg.Root, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer inspection.Close()
	copy := inspection
	if err := copy.Check(); err != nil {
		t.Fatal(err)
	}
	foreign, err := acquireRootLease(cfg.Root)
	if err == nil {
		_ = foreign.release()
		t.Fatal("inspection lent original producer-history lease")
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copy.Check(); err == nil {
		t.Fatal("closed copy revived")
	}
	unchangedParticipationInspection(t, cfg.Root, before)
	store, err := Open(cfg)
	if err != nil {
		t.Fatal("joined inspection retained original lease", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInspectionNativeCannotExpireOriginalProducer(t *testing.T) {
	cfg := nativeParticipationInspection(t)
	now := time.Now().UTC().Truncate(time.Second)
	cfg.Clock = func() time.Time { return now }
	facts := NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "initiator", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := OpenNodeProducer(t.Context(), cfg, facts)
	if err != nil {
		t.Fatal(err)
	}
	if err := producer.Live(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	if err := producer.Check(); err == nil {
		t.Fatal("expired work still admitted")
	}
	before := participationInspectionImages(t, cfg.Root)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for range 2 {
		inspection, err := OpenNodeInspection(ctx, cfg.Root, 0, 0)
		if err == nil || inspection.original != nil {
			_ = inspection.Close()
			t.Fatal("clock expiry cleared unknown original join")
		}
		unchangedParticipationInspection(t, cfg.Root, before)
	}
	if err := producer.Close(ctx); err != nil {
		t.Fatal("actual original producer removal", err)
	}
	inspection, err := OpenNodeInspection(ctx, cfg.Root, 0, 0)
	if err != nil {
		t.Fatal("genuine original removal not inspectable", err)
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNodeInspectionNativeCannotBorrowSourceExposurePolicy(t *testing.T) {
	cfg := nativeParticipationInspection(t)
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Replace([32]byte{1}, []Duty{{Identity: [32]byte{2}, Family: [32]byte{3}, Class: "direct-source", State: "exposed", NotAfter: time.Now().Add(time.Minute)}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	before := participationInspectionImages(t, cfg.Root)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	source, err := OpenSourceInspection(ctx, cfg.Root, 0, 0)
	if err != nil {
		t.Fatal("Source exposure policy regressed", err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	node, err := OpenNodeInspection(ctx, cfg.Root, 0, 0)
	if err == nil || node.original != nil {
		_ = node.Close()
		t.Fatal("Source exposure became stopped Node history")
	}
	unchangedParticipationInspection(t, cfg.Root, before)
}

func TestNodeInspectionNativeMissingHistoryNeverInitializes(t *testing.T) {
	for _, missing := range []string{rootLockName, rootMarkerName, "current", "watermark"} {
		t.Run(missing, func(t *testing.T) {
			cfg := nativeParticipationInspection(t)
			if err := os.Remove(filepath.Join(cfg.Root, missing)); err != nil {
				t.Fatal(err)
			}
			before := participationInspectionImages(t, cfg.Root)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			for range 2 {
				inspection, err := OpenNodeInspection(ctx, cfg.Root, 0, 0)
				if err == nil || inspection.original != nil {
					_ = inspection.Close()
					t.Fatal("missing history became initialized participation")
				}
				unchangedParticipationInspection(t, cfg.Root, before)
			}
		})
	}
}

func TestNodeInspectionNativeOriginalReadersAndClose(t *testing.T) {
	for _, fault := range []string{"clean", "cancel", "late-close"} {
		t.Run(fault, func(t *testing.T) {
			cfg := nativeParticipationInspection(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			inspection, err := OpenNodeInspection(ctx, cfg.Root, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			entered, resume := make(chan struct{}), make(chan struct{})
			var release sync.Once
			defer release.Do(func() { close(resume) })
			t.Cleanup(func() { _ = inspection.Close() })
			s := inspection.original
			s.observe = func(phase string, _ *os.File) {
				if phase == "read" {
					close(entered)
					<-resume
				}
			}
			checked, closed := make(chan error, 1), make(chan error, 1)
			go func() { checked <- inspection.Check() }()
			select {
			case <-entered:
			case err := <-checked:
				t.Fatal("reader completed before native boundary", err)
			case <-ctx.Done():
				t.Fatal("reader did not reach native boundary", ctx.Err())
			}
			if fault == "cancel" {
				cancel()
			}
			go func() { closed <- inspection.Close() }()
			limit, tick := time.NewTimer(time.Second), time.NewTicker(time.Millisecond)
			defer limit.Stop()
			defer tick.Stop()
			for {
				s.mu.Lock()
				denied := s.closed
				s.mu.Unlock()
				if denied {
					break
				}
				select {
				case <-tick.C:
				case <-limit.C:
					t.Fatal("Close denial waited for native reader")
				}
			}
			if err := inspection.Check(); err == nil {
				t.Fatal("closed inspection admitted fresh check")
			}
			foreign, err := acquireRootLease(cfg.Root)
			if err == nil {
				_ = foreign.release()
				t.Fatal("held reader returned producer-history lease")
			}
			select {
			case err := <-closed:
				t.Fatal("Close completed before actual native join", err)
			default:
			}
			if fault == "late-close" {
				if err := s.files["current"].file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			release.Do(func() { close(resume) })
			readErr, terminal := <-checked, <-closed
			if readErr == nil {
				t.Fatal("denied reader published late success")
			}
			if fault == "clean" && terminal != nil || fault == "cancel" && (!errors.Is(readErr, context.Canceled) || !errors.Is(terminal, context.Canceled)) || fault == "late-close" && !errors.Is(terminal, os.ErrClosed) {
				t.Fatal("original completion lost physical cause or invented failure", readErr, terminal)
			}
			if next := inspection.Close(); next != terminal {
				t.Fatal("copied Close changed retained outcome", next, terminal)
			}
			store, err := Open(cfg)
			if err != nil {
				t.Fatal("lease retained after actual join", err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
