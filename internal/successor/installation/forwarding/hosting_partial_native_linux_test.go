//go:build linux && network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// Each selected cut requires its own genuinely fresh completed installation.
// Neither the mode nor the observer can supply successful preparation or Start.
func TestForwardingHostingNativePartialRetainsOriginalPrefix(t *testing.T) {
	mode := os.Getenv("ARDENTS_FORWARDING_HOSTING_PARTIAL")
	if mode != "files-cancel" && mode != "files-cancel-close" && mode != "complete-cancel" && mode != "complete-cancel-close" {
		t.Fatal("invalid environment: exact fresh partial Hosting preparation case required")
	}
	parent, finish := context.WithTimeout(t.Context(), 30*time.Second)
	defer finish()
	completed := nativeCompleted(t, parent)
	b := completed.original.binding
	original := completed.original.files.dirs[b.Request.HostingRoot].info
	if err := completed.Close(); err != nil {
		t.Fatal(err)
	}
	path := nativeCompletedBinding(t)
	directory, journal, pending := hostingPreparationPaths(b.Request.InstallationRoot)
	ctx, cancel := context.WithCancel(parent)
	now := time.Now().UTC().Truncate(time.Second)
	policy := hosting.Policy{Provider: "private partial preparation fixture", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, InitialUsedBytes: 100, LowWatermarkBytes: 1024}
	wanted := "files-created"
	if strings.HasPrefix(mode, "complete-") {
		wanted = "before-close"
	}
	entered := make(chan *os.File, 1)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	done := make(chan error, 1)
	joined := false
	defer func() {
		cancel()
		unblock()
		if !joined {
			<-done
		}
	}()
	go func() {
		done <- prepareHosting(ctx, path, policy, func(stage string, file *os.File) {
			if stage == wanted {
				entered <- file
				<-release
			}
		})
	}()
	var held *os.File
	select {
	case held = <-entered:
	case result := <-done:
		joined = true
		t.Fatal("original preparation returned before selected cut", result)
	case <-parent.Done():
		t.Fatal("selected native cut not reached", parent.Err())
	}
	info, err := held.Stat()
	if err != nil || !os.SameFile(original, info) || !nodeAccess(info, 0700, b.UID, b.GID, true) {
		t.Fatal("cut did not retain original allowance directory", err)
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", false)
	probeCompletedLock(t, journal, false)
	if _, err := os.Lstat(pending); err != nil {
		t.Fatal("original pending blocker absent at selected cut", err)
	}
	journalRaw, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	wantedCount := 11
	wantedUID := uint32(0)
	if wanted == "before-close" {
		wantedCount = 16
		wantedUID = b.UID
	}
	if bytes.Count(journalRaw, []byte{'\n'}) != wantedCount {
		t.Fatal("wrong physical preparation frontier", wantedCount, bytes.Count(journalRaw, []byte{'\n'}))
	}
	for _, name := range []string{"budget.pin", "budget.lock", "budget.json"} {
		current, err := os.Lstat(filepath.Join(b.Request.HostingRoot, name))
		if err != nil || !nodeAccess(current, 0600, wantedUID, wantedUID, false) {
			t.Fatal("actual semantic files not retained at selected frontier", name, err)
		}
	}
	if wanted == "before-close" {
		if _, err := os.Lstat(filepath.Join(directory, "completed.json")); err != nil {
			t.Fatal("complete-looking result not actually written", err)
		}
	} else if _, err := os.Lstat(filepath.Join(directory, "completed.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("early cut acquired completion", err)
	}
	cancel()
	select {
	case result := <-done:
		joined = true
		t.Fatal("cancellation returned before original physical boundary joined", result)
	case <-time.After(30 * time.Millisecond):
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", false)
	probeCompletedLock(t, journal, false)
	if strings.HasSuffix(mode, "-close") {
		// Closing this exact original fd causes a real later Close error. No
		// injected error value can stand in for that physical result.
		if err := held.Close(); err != nil {
			t.Fatal(err)
		}
	}
	unblock()
	result := <-done
	joined = true
	if !errors.Is(result, context.Canceled) || !errors.Is(result, ErrRepairRequired) {
		t.Fatal("original cancellation or blocking result lost", result)
	}
	if strings.HasSuffix(mode, "-close") && !errors.Is(result, os.ErrClosed) {
		t.Fatal("actual late descriptor-close error lost", result)
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", true)
	probeCompletedLock(t, journal, true)
	if _, err := os.Lstat(pending); err != nil {
		t.Fatal("failed preparation published completion", err)
	}
	if after, err := os.Lstat(b.Request.HostingRoot); err != nil || !os.SameFile(original, after) {
		t.Fatal("partial preparation replaced original directory", err)
	}
	// Root Start uses this exact independent reader before any Start effect.
	// Even a fully written 16-record prefix cannot consume a pending blocker.
	reader, err := ReadCompleted(parent, path)
	if err != nil {
		t.Fatal(err)
	}
	readErr := reader.original.readHostingPreparation(path)
	if !errors.Is(readErr, ErrRepairRequired) {
		_ = reader.Close()
		t.Fatal("partial Hosting prefix acquired root-start proof", readErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	before := hostingPartialInventory(t, b.Request.HostingRoot, directory, journal, pending)
	if err := PrepareHosting(parent, path, policy); err == nil {
		t.Fatal("fresh root owner reset partial preparation")
	}
	command := os.Getenv("ARDENTS_FORWARDING_CONTROL_PROGRAM")
	if !validPath(command) {
		t.Fatal("invalid environment: pinned canonical Control required")
	}
	args := []string{"forwarding", "prepare-hosting", "--binding", path, "--provider", policy.Provider, "--start", policy.Start.Format(time.RFC3339), "--end", policy.End.Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "100", "--low-watermark-bytes", "1024"}
	if output, err := exec.CommandContext(parent, command, args...).CombinedOutput(); err == nil || bytes.Contains(output, []byte(`"prepared":true`)) {
		t.Fatal("canonical retry admitted partial allowance", err, string(output))
	}
	after := hostingPartialInventory(t, b.Request.HostingRoot, directory, journal, pending)
	if len(before) != len(after) {
		t.Fatal("refused retry changed partial inventory")
	}
	for name, stamp := range before {
		if after[name] != stamp {
			t.Fatal("refused retry changed original prefix", name)
		}
	}
	probeCompletedLock(t, b.Request.InstallationRoot+".forwarding-initial-journal", true)
	probeCompletedLock(t, journal, true)
}

type hostingPartialStamp struct {
	device, inode  uint64
	mode, uid, gid uint32
	sha            string
}

func hostingPartialInventory(t *testing.T, budget, directory, journal, pending string) map[string]hostingPartialStamp {
	t.Helper()
	paths := []string{budget, directory, journal, pending}
	for _, parent := range []string{budget, directory} {
		entries, err := os.ReadDir(parent)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			paths = append(paths, filepath.Join(parent, entry.Name()))
		}
	}
	result := map[string]hostingPartialStamp{}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatal("native stamp unavailable")
		}
		stamp := hostingPartialStamp{device: uint64(native.Dev), inode: native.Ino, mode: uint32(info.Mode().Perm()), uid: native.Uid, gid: native.Gid}
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			stamp.sha = digest(raw)
		} else if !info.IsDir() {
			t.Fatal("unknown partial file type")
		}
		result[path] = stamp
	}
	return result
}
