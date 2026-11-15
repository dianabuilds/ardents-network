//go:build linux && network_signing_native

package introduction

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

func slotInspectionFixture(t *testing.T) (string, Binding, []byte) {
	t.Helper()
	if os.Geteuid() != 0 || os.Getenv("ARDENTS_INTRODUCTION_INSPECTION_NATIVE") != "1" {
		t.Fatal("invalid environment: explicit root/ext4 slot inspection stand required")
	}
	parent, err := os.MkdirTemp("/var/lib", "i559-inspection-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	b := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := spending.Open(filepath.Join(parent, "spend"), spending.Binding{NetworkID: b.NetworkID, ProfileDigest: b.ProfileDigest, ReceiverNodeID: b.ReceiverNodeID, ReceiverDutyGeneration: b.ReceiverDutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	fact, err := ledger.TakeFreshRoot()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "slots")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	h, err := InitializeHistory(root, b, fact)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	now := time.Now().UTC().Truncate(time.Second)
	if err := h.Claim([32]byte{9}, now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(h.Close(), ledger.Close()); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, 988, 988)
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil {
		t.Fatal(err)
	}
	return root, b, raw
}

func TestIntroductionInspectionNativeRetainsOriginalFloorsAndLease(t *testing.T) {
	root, b, raw := slotInspectionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	before, err := os.Stat(filepath.Join(root, slotHistoryName))
	if err != nil {
		t.Fatal(err)
	}
	h, err := OpenInspection(ctx, root, b, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if other, err := OpenHistory(root, b); err == nil {
		other.Close()
		t.Fatal("inspection lost exclusive runtime lease")
	}
	if other, err := OpenInspection(ctx, root, b, 988, 988); err == nil {
		other.Close()
		t.Fatal("second inspection acquired original lease")
	}
	if err := errors.Join(h.Check(), h.Close()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filepath.Join(root, slotHistoryName))
	actual, readErr := os.ReadFile(filepath.Join(root, slotHistoryName))
	if err != nil || readErr != nil || !sameHistoryFile(before, after) || !bytes.Equal(raw, actual) {
		t.Fatal("read-only inspection mutated original floors", err, readErr)
	}
	reopened, err := OpenHistory(root, b)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Claim([32]byte{9}, time.Now().Add(time.Minute), time.Now()); err == nil {
		t.Fatal("inspection erased non-reclaim floor")
	}
}

func TestIntroductionInspectionNativeRefusesMissingForeignAndUncertain(t *testing.T) {
	for _, kind := range []string{"missing-snapshot", "missing-lease", "foreign-binding", "extra", "mode", "uid", "hardlink", "truncated", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			root, b, raw := slotInspectionFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			path := filepath.Join(root, slotHistoryName)
			var err error
			switch kind {
			case "missing-snapshot":
				err = os.Remove(path)
			case "missing-lease":
				err = os.Remove(filepath.Join(root, slotLeaseName))
			case "foreign-binding":
				b.ProfileDigest[0]++
			case "extra":
				err = os.WriteFile(filepath.Join(root, "uncertain-stage"), []byte("retained"), 0600)
			case "mode":
				err = os.Chmod(path, 0644)
			case "uid":
				err = os.Chown(path, 989, 989)
			case "hardlink":
				err = os.Link(path, filepath.Join(filepath.Dir(root), "foreign-link"))
			case "truncated":
				err = os.WriteFile(path, raw[:len(raw)-1], 0600)
			case "canceled":
				cancel()
			}
			if err != nil {
				t.Fatal(err)
			}
			if h, err := OpenInspection(ctx, root, b, 988, 988); err == nil {
				h.Close()
				t.Fatal("invalid retained history accepted")
			}
			if kind == "missing-snapshot" || kind == "missing-lease" {
				if kind == "missing-lease" {
					path = filepath.Join(root, slotLeaseName)
				}
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("inspection recreated missing history", err)
				}
			}
		})
	}
}

func TestIntroductionInspectionNativeRestorationCannotReviveOriginal(t *testing.T) {
	root, b, raw := slotInspectionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	h, err := OpenInspection(ctx, root, b, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	path := filepath.Join(root, slotHistoryName)
	if err := os.WriteFile(path, raw[:len(raw)-1], 0600); err != nil {
		t.Fatal(err)
	}
	if h.Check() == nil {
		t.Fatal("original accepted changed snapshot")
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if h.Check() == nil || h.Close() == nil {
		t.Fatal("restoration erased original failure")
	}
	fresh, err := OpenInspection(ctx, root, b, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIntroductionInspectionNativeCloseJoinsCanceledReader(t *testing.T) {
	root, b, _ := slotInspectionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	h, err := OpenInspection(ctx, root, b, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	h.original.observe = func(stage string, _ *os.File) {
		if stage == "read" {
			select {
			case <-entered:
			default:
				close(entered)
			}
			<-release
		}
	}
	checked, closed := make(chan error, 1), make(chan error, 1)
	go func() { checked <- h.Check() }()
	<-entered
	cancel()
	go func() { closed <- h.Close() }()
	until := time.Now().Add(time.Second)
	for {
		h.original.mu.Lock()
		denied := h.original.closed
		h.original.mu.Unlock()
		if denied {
			break
		}
		if time.Now().After(until) {
			t.Fatal("Close failed to seal reads")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case err := <-closed:
		t.Fatal("Close escaped held original read", err)
	default:
	}
	if other, err := OpenHistory(root, b); err == nil {
		other.Close()
		t.Fatal("lease returned before original reader joined")
	}
	if h.Check() == nil {
		t.Fatal("late reader accepted")
	}
	unblock()
	if err := <-checked; !errors.Is(err, context.Canceled) {
		t.Fatal("canceled original read accepted", err)
	}
	first := <-closed
	if !errors.Is(first, context.Canceled) || h.Close() != first {
		t.Fatal("terminal cancellation not retained", first)
	}
}

func TestIntroductionInspectionNativeOriginalInodeAndRootMovement(t *testing.T) {
	for _, moveRoot := range []bool{false, true} {
		root, b, raw := slotInspectionFixture(t)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		h, err := OpenInspection(ctx, root, b, 988, 988)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		path := filepath.Join(root, slotHistoryName)
		if moveRoot {
			path = root
		}
		saved := filepath.Join(filepath.Dir(root), "original-moved")
		if err := os.Rename(path, saved); err != nil {
			t.Fatal(err)
		}
		if moveRoot {
			if err := os.Rename(saved, path); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chown(path, 988, 988); err != nil {
				t.Fatal(err)
			}
		}
		if h.Check() == nil || h.Close() == nil {
			t.Fatal("foreign inode or root restoration revived original")
		}
		cancel()
	}
}

func TestIntroductionInspectionNativePhysicalCloseFailureRetained(t *testing.T) {
	root, b, _ := slotInspectionFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	h, err := OpenInspection(ctx, root, b, 988, 988)
	if err != nil {
		t.Fatal(err)
	}
	h.original.observe = func(stage string, file *os.File) {
		if stage == "before-close" {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		}
	}
	first := h.Close()
	if first == nil || h.Close() != first {
		t.Fatal("native close failure erased", first)
	}
}
