//go:build network_signing_native

package introduction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// The outer controller supplies a fresh genuinely installed stopped generation
// for each selected case. No accepting history, UID or Release is fabricated.
func TestIntroductionHostingNativeOriginalPreparation(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-introduction-initial-native-stand")
	kind := os.Getenv("ARDENTS_INTRODUCTION_HOSTING_CASE")
	if os.Geteuid() != 0 || err != nil || string(marker) != "ardents-introduction-initial-native-v1\n" || (kind != "complete" && kind != "files-created" && kind != "before-close") {
		t.Fatal("invalid environment: fresh marked original Hosting stand and exact case required", err)
	}
	bindingPath := os.Getenv("ARDENTS_INTRODUCTION_INITIAL_BINDING")
	if !validPath(bindingPath) {
		t.Fatal("invalid environment: original initial binding required")
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	b, err := decodeBinding(read(bindingPath))
	if err != nil {
		t.Fatal(err)
	}
	command := os.Getenv("ARDENTS_INTRODUCTION_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_INTRODUCTION_CONTROL_PROGRAM_SHA256")
	raw := read(command)
	sum := sha256.Sum256(raw)
	clear(raw)
	if !filepath.IsAbs(command) || hex.EncodeToString(sum[:]) != pin {
		t.Fatal("invalid environment: pinned same-source canonical Control required")
	}
	initialJournal := b.Request.InstallationRoot + ".introduction-initial-journal"
	initialBefore := read(initialJournal)
	now := time.Now().UTC().Truncate(time.Second)
	policy := hosting.Policy{Provider: "introduction-native-provider", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1024}
	args := []string{"introduction", "prepare-hosting", "--binding", bindingPath, "--provider", policy.Provider, "--start", policy.Start.Format(time.RFC3339), "--end", policy.End.Format(time.RFC3339), "--unit", policy.Unit, "--quantity", "1", "--directions", policy.Directions, "--interfaces", "lo", "--initial-used-bytes", "0", "--low-watermark-bytes", "1024"}
	invoke := func() ([]byte, error) { return exec.CommandContext(t.Context(), command, args...).CombinedOutput() }
	directory, journal, pending := hostingPreparationPaths(b.Request.InstallationRoot)
	originalRoot, err := os.Lstat(b.Request.HostingRoot)
	if err != nil {
		t.Fatal(err)
	}
	if kind == "complete" {
		// A foreign file refuses before the one-use preparation journal is born.
		foreign := filepath.Join(b.Request.HostingRoot, "budget.json")
		foreignBytes := []byte("test-owned foreign budget prefix\n")
		if err := os.WriteFile(foreign, foreignBytes, 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := invoke(); err == nil {
			t.Fatal("foreign nonempty allowance adopted", string(out))
		}
		if !bytes.Equal(read(foreign), foreignBytes) {
			t.Fatal("foreign bytes changed")
		}
		if _, err := os.Lstat(journal); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("foreign root reached preparation effects", err)
		}
		if err := os.Remove(foreign); err != nil {
			t.Fatal(err)
		} // Remove only this test's own non-authoritative leaf.
		if out, err := invoke(); err != nil || !bytes.Contains(out, []byte(`"prepared":true`)) {
			t.Fatal("canonical genuine Hosting preparation failed", err, string(out))
		}
		completed, err := ReadCompleted(t.Context(), bindingPath)
		if err != nil {
			t.Fatal("independent prepared history refused", err)
		}
		copyHandle := completed
		if !completed.original.hostingPrepared {
			t.Fatal("complete Hosting history not observed")
		}
		if err := completed.Check(); err != nil {
			t.Fatal(err)
		}
		// The original reader latches same-byte inode substitution and cannot revive.
		pinPath := filepath.Join(b.Request.HostingRoot, "budget.pin")
		originalPin := read(pinPath)
		retained := filepath.Join(filepath.Dir(b.Request.HostingRoot), "original-allowance-pin")
		if err := os.Rename(pinPath, retained); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pinPath, originalPin, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(pinPath, int(b.UID), int(b.GID)); err != nil {
			t.Fatal(err)
		}
		refusal := completed.Check()
		if refusal == nil {
			t.Fatal("copied pin inode accepted")
		}
		if err := os.Remove(pinPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(retained, pinPath); err != nil {
			t.Fatal(err)
		}
		if copyHandle.Check() == nil || copyHandle.Close() == nil {
			t.Fatal("restoration revived the failed original reader")
		}
		fresh, err := ReadCompleted(t.Context(), bindingPath)
		if err != nil {
			t.Fatal("independent restored original unavailable", err)
		}
		if err := fresh.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("successful original preparation left blocker", err)
		}
	} else {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		fired := false
		err := prepareHosting(ctx, bindingPath, policy, func(stage string, file *os.File) {
			if stage != kind || fired {
				return
			}
			fired = true
			cancel()
			if err := file.Close(); err != nil {
				t.Fatal("actual original descriptor Close", err)
			}
		})
		if !fired || !errors.Is(err, ErrRepairRequired) || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed) {
			t.Fatal("original uncertain/caller/physical failures lost", fired, err)
		}
		if _, err := os.Lstat(pending); err != nil {
			t.Fatal("uncertain preparation erased its blocker", err)
		}
		if h, err := ReadCompleted(t.Context(), bindingPath); err == nil {
			_ = h.Close()
			t.Fatal("blocked prefix accepted as complete")
		}
	}
	journalBefore := read(journal)
	rootAfter, err := os.Lstat(b.Request.HostingRoot)
	if err != nil || !os.SameFile(originalRoot, rootAfter) {
		t.Fatal("original allowance directory replaced", err)
	}
	if out, err := invoke(); err == nil {
		t.Fatal("retained preparation adopted/reinitialized", string(out))
	}
	if !bytes.Equal(journalBefore, read(journal)) || !bytes.Equal(initialBefore, read(initialJournal)) {
		t.Fatal("retry changed retained original histories")
	}
	for _, root := range []string{b.Request.StateRoot, b.Request.ParticipationRoot, b.Request.SpendRoot, b.Request.IntroductionRoot} {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal("Hosting preparation initialized another semantic owner", root, err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "intent.json" && entry.Name() != "completed.json" {
			t.Fatal("foreign preparation inventory", entry.Name())
		}
	}
	// Root-only preparation carries no accepting invocation or domain join.
	state, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-introduction.service", "-p", "ActiveState", "-p", "SubState", "-p", "MainPID").CombinedOutput()
	if err != nil || !strings.Contains(string(state), "MainPID=0\n") || !strings.Contains(string(state), "ActiveState=inactive\n") || !strings.Contains(string(state), "SubState=dead\n") {
		t.Fatal("preparation started a role", err, string(state))
	}
}
