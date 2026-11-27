//go:build linux && (source_installed_native || forwarding_platform_native || descriptor_platform_native || join_platform_native)

package nodeinstalled_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type installedInvocation struct {
	unit, id, operation string
}

func observeInstalledInvocation(t *testing.T, unit, operation string) installedInvocation {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "show", unit, "-p", "InvocationID", "--value").Output()
	id := strings.TrimSpace(string(out))
	decoded, parseErr := hex.DecodeString(id)
	if err != nil || parseErr != nil || len(decoded) != 16 || id == strings.Repeat("0", 32) {
		t.Fatal("original installed invocation unavailable", unit, err, parseErr)
	}
	return installedInvocation{unit: unit, id: id, operation: operation}
}

func (original installedInvocation) requireCurrent(t *testing.T) {
	t.Helper()
	if current := observeInstalledInvocation(t, original.unit, original.operation); current != original {
		t.Fatal("installed invocation changed", original.unit, original.id, current.id)
	}
}

type installedJournalEntry struct {
	Invocation string `json:"_SYSTEMD_INVOCATION_ID"`
	Unit       string `json:"_SYSTEMD_UNIT"`
	Message    string `json:"MESSAGE"`
}

type installedRoleEvent struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome"`
}

func (original installedInvocation) lastRoleEvent(raw []byte) (installedRoleEvent, error) {
	var last installedRoleEvent
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var entry installedJournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return installedRoleEvent{}, err
		}
		if entry.Invocation != original.id || entry.Unit != original.unit {
			continue
		}
		var event installedRoleEvent
		if err := json.Unmarshal([]byte(entry.Message), &event); err != nil {
			if strings.HasPrefix(strings.TrimSpace(entry.Message), "{") {
				return installedRoleEvent{}, err
			}
			continue
		}
		if event.Operation == original.operation || event.Operation == "" && (event.Phase != "" || event.Outcome != "") {
			last = event
		}
	}
	return last, nil
}

func (original installedInvocation) journal(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "journalctl", "--no-pager", "-o", "json", "--output-fields=MESSAGE,_SYSTEMD_UNIT,_SYSTEMD_INVOCATION_ID",
		"_SYSTEMD_UNIT="+original.unit, "_SYSTEMD_INVOCATION_ID="+original.id).Output()
}

func (original installedInvocation) requireJoined(raw []byte) error {
	event, err := original.lastRoleEvent(raw)
	if err != nil {
		return err
	}
	if event.Operation != original.operation || event.Phase != "joined" || event.Outcome != "canceled" {
		return fmt.Errorf("original joined cancellation report unavailable: %+v", event)
	}
	return nil
}

func (original installedInvocation) stop(t *testing.T, phase string) {
	t.Helper()
	original.requireCurrent(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "stop", original.unit).CombinedOutput(); err != nil {
		t.Fatal(phase, "installed unit stop", original.unit, err, string(out))
	}
	result, err := exec.CommandContext(ctx, "systemctl", "show", original.unit, "-p", "Result", "--value").Output()
	if err != nil || strings.TrimSpace(string(result)) != "success" {
		t.Fatal(phase, "clean stop manager result", original.unit, string(result), err)
	}
	raw := original.syncJournal(t, ctx)
	if err := original.requireJoined(raw); err != nil {
		t.Fatal(phase, original.unit, original.id, err)
	}
}

// Known crash is a causal negative control with real earlier clean stops in the
// same unit journal and only a listening event in the crashed invocation.
func (original installedInvocation) requireCrashWithoutTerminal(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	raw := original.syncJournal(t, ctx)
	event, err := original.lastRoleEvent(raw)
	if err != nil || event.Operation != original.operation || event.Phase != "listening" {
		t.Fatal("crash did not retain only the original listening event", original.unit, original.id, event, err)
	}
	if original.requireJoined(raw) == nil {
		t.Fatal("SIGKILL accepted as original joined cancellation", original.unit, original.id)
	}
}

func (original installedInvocation) syncJournal(t *testing.T, ctx context.Context) []byte {
	t.Helper()
	if out, err := exec.CommandContext(ctx, "journalctl", "--sync").CombinedOutput(); err != nil {
		t.Fatal("installed journal sync", err, string(out))
	}
	raw, err := original.journal(ctx)
	if err != nil {
		t.Fatal("original invocation journal", original.unit, original.id, err)
	}
	return raw
}

func TestInstalledInvocationRequiresOriginalCompleteTerminal(t *testing.T) {
	original := installedInvocation{unit: "ardents-descriptor.service", id: "aabb", operation: "node.descriptor"}
	entry := func(id, unit, message string) string {
		raw, err := json.Marshal(installedJournalEntry{Invocation: id, Unit: unit, Message: message})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw) + "\n"
	}
	joined := `{"operation":"node.descriptor","phase":"joined","outcome":"canceled"}`
	old := entry("ccdd", original.unit, joined)
	current := entry(original.id, original.unit, joined)
	for _, journal := range []string{
		old,
		old + entry(original.id, original.unit, `{"operation":"node.descriptor","phase":"joined"}`),
		old + entry(original.id, original.unit, `{"phase":"joined","outcome":"canceled"}`),
		old + entry(original.id, original.unit, `{"operation":"node.descriptor","phase":"retiring","outcome":"completion-unavailable"}`),
		old + entry(original.id, "other.service", joined),
		old + current + entry(original.id, original.unit, `{"operation":"node.descriptor","phase":"joined"}`),
	} {
		if err := original.requireJoined([]byte(journal)); err == nil {
			t.Fatal("accepted stale, incomplete or foreign terminal", journal)
		}
	}
	if err := original.requireJoined([]byte(old + current)); err != nil {
		t.Fatal("refused original joined cancellation", err)
	}
}

// Listening only gates scheduling. Each installed scenario still has to complete
// its authenticated operation against this same invocation.
func (original installedInvocation) waitListening(t *testing.T, phase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := original.journal(ctx)
		if err != nil {
			t.Fatal(phase, "original invocation journal", original.unit, err)
		}
		event, err := original.lastRoleEvent(raw)
		if err != nil {
			t.Fatal(phase, "original invocation event", original.unit, err)
		}
		if event.Operation == original.operation && event.Phase == "listening" && event.Outcome == "original-duty" {
			original.requireCurrent(t)
			return
		}
		if event.Phase != "" {
			t.Fatal(phase, "original invocation did not reach listening", original.unit, event)
		}
		select {
		case <-ctx.Done():
			t.Fatal(phase, "timed out waiting for original invocation", original.unit, ctx.Err())
		case <-ticker.C:
		}
	}
}
