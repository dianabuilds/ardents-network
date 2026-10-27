//go:build linux && network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func TestForwardingHostingNativePrepareOriginalAllowanceAndNoReset(t *testing.T) {
	if path := os.Getenv("ARDENTS_FORWARDING_HOSTING_VERIFY"); path != "" {
		if !validPath(path) || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
			t.Fatal("invalid environment: actual Node principal required")
		}
		owner, err := hosting.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		view, err := owner.Observe(t.Context())
		closeErr := owner.Close()
		if err != nil || closeErr != nil || view.UsedBytes < 100 || view.ReservedBytes != 0 {
			t.Fatal("original Node allowance unavailable", view, err, closeErr)
		}
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	c := nativeCompleted(t, ctx)
	b := c.original.binding
	original := c.original.files.dirs[b.Request.HostingRoot].info
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	command := os.Getenv("ARDENTS_FORWARDING_CONTROL_PROGRAM")
	if !validPath(command) {
		t.Fatal("invalid environment: pinned canonical Control required")
	}
	now := time.Now().UTC().Truncate(time.Second)
	args := []string{"forwarding", "prepare-hosting", "--binding", nativeCompletedBinding(t), "--provider", "private component fixture", "--start", now.Add(-time.Minute).Format(time.RFC3339), "--end", now.Add(time.Hour).Format(time.RFC3339), "--unit", "GiB", "--quantity", "1", "--directions", "tx+rx", "--interfaces", "lo", "--initial-used-bytes", "100", "--low-watermark-bytes", "1024"}
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil || string(output) != "{\"operation\":\"forwarding.prepare-hosting\",\"prepared\":true,\"runtime_authority\":false}\n" {
		t.Fatal("canonical one-use preparation failed", err, string(output))
	}
	after, err := os.Lstat(b.Request.HostingRoot)
	if err != nil || !os.SameFile(original, after) || !nodeAccess(after, 0700, b.UID, b.GID, true) {
		t.Fatal("original allowance directory changed", err)
	}
	c, err = ReadCompleted(ctx, nativeCompletedBinding(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.original.readHostingPreparation(nativeCompletedBinding(t)); err != nil {
		_ = c.Close()
		t.Fatal("independent original preparation refused", err)
	}
	directory, journal, _ := hostingPreparationPaths(b.Request.InstallationRoot)
	probeCompletedLock(t, journal, false)
	intentRaw := c.original.files.files[filepath.Join(directory, "intent.json")].body
	resultRaw := c.original.files.files[filepath.Join(directory, "completed.json")].body
	journalRaw := c.original.files.files[journal].body
	lines := bytes.Split(journalRaw[:len(journalRaw)-1], []byte{'\n'})
	if len(lines) != 16 {
		t.Fatal("independent preparation transcript count differs", len(lines))
	}
	for index := range lines {
		missing := append(bytes.Join(append(append([][]byte(nil), lines[:index]...), lines[index+1:]...), []byte{'\n'}), '\n')
		if _, err := decodeHostingRecords(missing, b, intentRaw, resultRaw); err == nil {
			t.Fatal("incomplete preparation admitted", index)
		}
		if index+1 < len(lines) {
			changed := make([][]byte, len(lines))
			for n := range lines {
				changed[n] = bytes.Clone(lines[n])
			}
			changed[index], changed[index+1] = changed[index+1], changed[index]
			for n := range changed {
				var row initialRecord
				if json.Unmarshal(changed[n], &row) != nil {
					t.Fatal("actual record unreadable")
				}
				row.Sequence = n
				changed[n], _ = json.Marshal(row)
			}
			if _, err := decodeHostingRecords(append(bytes.Join(changed, []byte{'\n'}), '\n'), b, intentRaw, resultRaw); err == nil {
				t.Fatal("reordered preparation admitted", index)
			}
		}
	}
	if err := c.Check(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	probeCompletedLock(t, journal, true)
	before, err := os.ReadFile(filepath.Join(b.Request.HostingRoot, "budget.json"))
	if err != nil {
		t.Fatal(err)
	}
	output, err = exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err == nil {
		t.Fatal("canonical replay reset allowance", string(output))
	}
	unchanged, err := os.ReadFile(filepath.Join(b.Request.HostingRoot, "budget.json"))
	if err != nil || !bytes.Equal(before, unchanged) {
		t.Fatal("refused replay changed allowance", err)
	}
	// A separate actual Node process proves file delivery; it receives no root
	// lease, permission, installed executable, Startup or live budget handle.
	actor, err := os.MkdirTemp("/var/lib", "ardents-hosting-actor-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(actor)
	if err := os.Chmod(actor, 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	actorPath := filepath.Join(actor, "hosting.test")
	file, err := os.OpenFile(actorPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0555)
	if err != nil {
		_ = input.Close()
		t.Fatal(err)
	}
	_, copyErr := io.Copy(file, io.LimitReader(input, 128<<20))
	if err := errors.Join(copyErr, file.Sync(), file.Close(), input.Close()); err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(ctx, actorPath, "-test.run=^TestForwardingHostingNativePrepareOriginalAllowanceAndNoReset$", "-test.count=1", "-test.timeout=10s")
	child.Env = []string{"PATH=/usr/bin:/bin", "GOMAXPROCS=1", "GOMEMLIMIT=536870912", "ARDENTS_FORWARDING_HOSTING_VERIFY=" + b.Request.HostingRoot}
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: b.UID, Gid: b.GID, Groups: []uint32{}}}
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatal("actual Node retained open failed", err, string(output))
	}
	c, err = ReadCompleted(ctx, nativeCompletedBinding(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.original.readHostingPreparation(nativeCompletedBinding(t)); err != nil {
		_ = c.Close()
		t.Fatal("Node use invalidated original preparation", err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
