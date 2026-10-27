//go:build linux && network_signing_native

package hosting

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

func nativePeriodInspection(t *testing.T) (string, string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: root Hosting inspection driver required")
	}
	parent := t.TempDir()
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: private root-controlled native ext4 TMPDIR required", err)
	}
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("invalid environment: original Hosting ancestors required", path, err)
		}
		if path == "/" {
			break
		}
	}
	root := filepath.Join(parent, "period")
	now := time.Now().UTC().Truncate(time.Second)
	policy := Policy{Provider: "native-inspection-test", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, InitialUsedBytes: 123, LowWatermarkBytes: 1 << 20}
	if err := Initialize(root, policy); err != nil {
		t.Fatal(err)
	}
	pin, err := os.ReadFile(filepath.Join(root, "budget.pin"))
	if err != nil {
		t.Fatal(err)
	}
	return root, string(pin)
}

func periodInspectionImages(t *testing.T, path string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	images := map[string][]byte{}
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(path, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		images[entry.Name()] = raw
	}
	return images
}

func unchangedPeriodInspection(t *testing.T, path string, before map[string][]byte) {
	t.Helper()
	after := periodInspectionImages(t, path)
	if len(before) != len(after) {
		t.Fatal("inspection changed allowance inventory")
	}
	for name, raw := range before {
		if actual, found := after[name]; !found || !bytes.Equal(raw, actual) {
			t.Fatal("inspection changed retained allowance", name)
		}
	}
}

func TestHostingInspectionNativeOriginalHistoryAndLease(t *testing.T) {
	root, policy := nativePeriodInspection(t)
	before := periodInspectionImages(t, root)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	inspection, err := OpenInspection(ctx, root, policy, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer inspection.Close()
	copy := inspection
	if err := copy.Check(); err != nil {
		t.Fatal(err)
	}
	rootFile, err := openHostingRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFile.Close()
	lease, err := openHostingLease(rootFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		_ = lease.Close()
		t.Fatal("inspection lent original allowance lease")
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copy.Check(); err == nil {
		t.Fatal("closed copied inspection revived")
	}
	unchangedPeriodInspection(t, root, before)
	budget, err := Open(root)
	if err != nil {
		t.Fatal("joined inspection retained original lease", err)
	}
	if err := budget.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHostingInspectionNativeUnresolvedReservationIsNotReturned(t *testing.T) {
	root, policy := nativePeriodInspection(t)
	budget, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC()
	reservation, err := budget.Reserve(ctx, ReservationRequest{Work: Traffic{Tx: 100, Rx: 100}, Termination: Traffic{Tx: 10, Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	before := periodInspectionImages(t, root)
	for range 2 {
		inspection, err := OpenInspection(ctx, root, policy, 0, 0)
		if inspection.original != nil {
			_ = inspection.Close()
		}
		if err == nil {
			t.Fatal("unresolved physical reservation became stopped capacity")
		}
		unchangedPeriodInspection(t, root, before)
	}
	if err := reservation.Release(ctx); err != nil {
		t.Fatal(err)
	}
	inspection, err := OpenInspection(ctx, root, policy, 0, 0)
	if err != nil {
		t.Fatal("actual original return was not inspectable", err)
	}
	if err := inspection.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHostingInspectionNativeRefusesWithoutRepairOrCreation(t *testing.T) {
	for _, fault := range []string{"missing-lock", "missing-state", "missing-pin", "pending", "extra-file", "damaged", "policy", "principal", "state-link"} {
		t.Run(fault, func(t *testing.T) {
			root, policy := nativePeriodInspection(t)
			uid := uint32(0)
			var err error
			switch fault {
			case "missing-lock":
				err = os.Remove(filepath.Join(root, "budget.lock"))
			case "missing-state":
				err = os.Remove(filepath.Join(root, "budget.json"))
			case "missing-pin":
				err = os.Remove(filepath.Join(root, "budget.pin"))
			case "pending", "extra-file":
				name := "unknown"
				if fault == "pending" {
					name = "budget.pending"
				}
				err = os.WriteFile(filepath.Join(root, name), []byte{1}, 0600)
			case "damaged":
				err = os.WriteFile(filepath.Join(root, "budget.json"), []byte("{}"), 0600)
			case "policy":
				policy = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "principal":
				uid = 123
			case "state-link":
				path := filepath.Join(root, "budget.json")
				err = os.Rename(path, path+"-original")
				if err == nil {
					err = os.Symlink(path+"-original", path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before := periodInspectionImages(t, root)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			for range 2 {
				inspection, err := OpenInspection(ctx, root, policy, uid, 0)
				if inspection.original != nil {
					_ = inspection.Close()
				}
				if err == nil {
					t.Fatal("unavailable original history accepted")
				}
				unchangedPeriodInspection(t, root, before)
			}
		})
	}
}

func TestHostingInspectionNativeOriginalJoinAndLateFailure(t *testing.T) {
	for _, fault := range []string{"clean", "cancel", "late-close"} {
		t.Run(fault, func(t *testing.T) {
			root, policy := nativePeriodInspection(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			inspection, err := OpenInspection(ctx, root, policy, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			entered, resume := make(chan struct{}), make(chan struct{})
			var release sync.Once
			defer release.Do(func() { close(resume) })
			t.Cleanup(func() { _ = inspection.Close() })
			s := inspection.original
			s.observe = func(phase string, file *os.File) {
				if phase == "read" && file == s.state.file {
					close(entered)
					<-resume
				}
			}
			checked, closed := make(chan error, 1), make(chan error, 1)
			go func() { checked <- inspection.Check() }()
			select {
			case <-entered:
			case err := <-checked:
				t.Fatal("reader returned before native boundary", err)
			case <-ctx.Done():
				t.Fatal("reader failed to reach native boundary", ctx.Err())
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
					t.Fatal("local Close denial waited for native reader")
				}
			}
			if err := inspection.Check(); err == nil {
				t.Fatal("closed inspection admitted new observation")
			}
			lease, err := os.OpenFile(filepath.Join(root, "budget.lock"), os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
				_ = lease.Close()
				t.Fatal("held reader returned allowance lease")
			}
			if err := lease.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-closed:
				t.Fatal("Close returned before original join", err)
			default:
			}
			if fault == "late-close" {
				if err := s.state.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			release.Do(func() { close(resume) })
			readErr, closeErr := <-checked, <-closed
			if readErr == nil {
				t.Fatal("denied late reader published success")
			}
			switch fault {
			case "clean":
				if closeErr != nil {
					t.Fatal("local denial became false physical failure", closeErr)
				}
			case "cancel":
				if !errors.Is(readErr, context.Canceled) || !errors.Is(closeErr, context.Canceled) {
					t.Fatal("cancellation lost", readErr, closeErr)
				}
			case "late-close":
				if !errors.Is(closeErr, os.ErrClosed) {
					t.Fatal("actual late Close failure lost", closeErr)
				}
			}
			if next := inspection.Close(); next != closeErr {
				t.Fatal("copied Close changed retained result", next, closeErr)
			}
			fresh, finish := context.WithTimeout(t.Context(), time.Second)
			defer finish()
			next, err := OpenInspection(fresh, root, policy, 0, 0)
			if err != nil {
				t.Fatal("original lease not returned after actual joins", err)
			}
			if err := next.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
