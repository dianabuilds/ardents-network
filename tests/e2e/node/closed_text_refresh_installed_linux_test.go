//go:build linux && text_worker_installed

package state_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// The positive command cell's 30-second socket wait, 45-second publish, Link
// and initial-read bounds, 330-second refresh observation, 65-second predecessor
// overlap, 45-second second read, 15-second withdrawal and 20-second refusal
// check total 640 seconds. Twelve minutes also retain 80 seconds for Store and
// service completion around those bounded operations. Permission times are whole
// seconds and the endpoint rejects its exact NotAfter, so both independently
// approved Permissions must remain current for the whole episode; a later hour
// cannot renew either holder.
const installedCommandRefreshCoverage = 12 * time.Minute
const installedCommandRefreshObservationWindow = 35 * time.Second

func installedCommandRefreshCoverageError(now time.Time, permissions map[string]credential.Permission) error {
	deadline := now.UTC().Add(installedCommandRefreshCoverage).Truncate(time.Second)
	for _, role := range []string{"reader", "publisher"} {
		permission, ok := permissions[role]
		if !ok || !deadline.Before(permission.NotAfter) {
			return fmt.Errorf("%s Permission ends at %s before bounded scenario deadline %s", role, permission.NotAfter, deadline)
		}
	}
	return nil
}

func requireInstalledCommandRefreshCoverage(t *testing.T, now time.Time, permissions map[string]credential.Permission) {
	t.Helper()
	if err := installedCommandRefreshCoverageError(now, permissions); err != nil {
		t.Fatalf("invalid installed positive refresh prerequisite: %v", err)
	}
}

func TestInstalledCommandRefreshRequiresCoveringPermissions(t *testing.T) {
	expires := time.Date(2026, time.September, 24, 16, 0, 0, 0, time.UTC)
	now := expires.Add(-installedCommandRefreshCoverage - time.Second)
	permissions := map[string]credential.Permission{
		"reader":    {NotAfter: expires},
		"publisher": {NotAfter: expires},
	}
	requireInstalledCommandRefreshCoverage(t, now, permissions)
	if err := installedCommandRefreshCoverageError(expires.Add(-installedCommandRefreshCoverage), permissions); err == nil {
		t.Fatal("positive refresh accepted a Permission that ends at its bounded deadline")
	}
}

func requireInstalledCommandRefreshExpiry(t *testing.T, first reachability.Verified, permissions map[string]credential.Permission) {
	t.Helper()
	refreshAt := first.Descriptor.Private.NotBefore.Add(300 * time.Second)
	for _, role := range []string{"reader", "publisher"} {
		permission, ok := permissions[role]
		if !ok || !time.Now().UTC().Before(permission.NotAfter) || refreshAt.Before(permission.NotAfter) || refreshAt.After(permission.NotAfter.Add(30*time.Second)) {
			t.Fatalf("invalid installed expiry-boundary prerequisite: %s Permission ends at %s, refresh starts at %s", role, permission.NotAfter, refreshAt)
		}
	}
}

type installedCommandJournalEvent struct {
	Timestamp string `json:"__REALTIME_TIMESTAMP"`
	Message   string `json:"MESSAGE"`
}

type installedCommandRefreshFailureEvent struct {
	at    time.Time
	stage string
}

func installedCommandRefreshFailureTime(line string) (installedCommandRefreshFailureEvent, bool, error) {
	var event installedCommandJournalEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return installedCommandRefreshFailureEvent{}, false, err
	}
	if !strings.Contains(event.Message, `"kind":"headless-runtime-publication-refresh-failed"`) {
		return installedCommandRefreshFailureEvent{}, false, nil
	}
	microseconds, err := strconv.ParseInt(event.Timestamp, 10, 64)
	if err != nil {
		return installedCommandRefreshFailureEvent{}, false, fmt.Errorf("decode journal realtime timestamp: %w", err)
	}
	var detail struct {
		Failure string `json:"failure"`
	}
	if err := json.Unmarshal([]byte(event.Message), &detail); err != nil {
		return installedCommandRefreshFailureEvent{}, false, fmt.Errorf("decode refresh failure: %w", err)
	}
	return installedCommandRefreshFailureEvent{at: time.UnixMicro(microseconds).UTC(), stage: detail.Failure}, true, nil
}

type installedCommandRefreshFailureWindow uint8

const (
	installedCommandRefreshFailureBeforeExpiry installedCommandRefreshFailureWindow = iota
	installedCommandRefreshFailureAtExpiry
	installedCommandRefreshFailureAfterWindow
)

func classifyInstalledCommandRefreshFailure(occurred, expires time.Time) installedCommandRefreshFailureWindow {
	if occurred.Before(expires) {
		return installedCommandRefreshFailureBeforeExpiry
	}
	if occurred.After(expires.Add(installedCommandRefreshObservationWindow)) {
		return installedCommandRefreshFailureAfterWindow
	}
	return installedCommandRefreshFailureAtExpiry
}

func waitInstalledCommandRefreshExpiry(t *testing.T, invocation string, permissions map[string]credential.Permission) {
	t.Helper()
	publisher, ok := permissions["publisher"]
	if !ok {
		t.Fatal("invalid installed expiry-boundary prerequisite: publisher Permission missing")
	}
	deadline := publisher.NotAfter.Add(installedCommandRefreshObservationWindow)
	for {
		journal := string(installedCommandTool(t, "journalctl", "--no-pager", "-o", "json", "_SYSTEMD_INVOCATION_ID="+invocation))
		for _, line := range strings.Split(journal, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			failure, matching, err := installedCommandRefreshFailureTime(line)
			if err != nil {
				t.Fatalf("invalid Endpoint journal event: %v", err)
			}
			if !matching {
				continue
			}
			if failure.stage != "rotation-source-prefix-opening-entry-token-take-permission" {
				t.Fatalf("refresh failed unexpectedly before expiry observation: stage=%s at %s", failure.stage, failure.at)
			}
			switch classifyInstalledCommandRefreshFailure(failure.at, publisher.NotAfter) {
			case installedCommandRefreshFailureBeforeExpiry:
				t.Fatalf("refresh failed before publisher Permission expiry at %s", failure.at)
			case installedCommandRefreshFailureAfterWindow:
				t.Fatalf("scheduled refresh failure occurred outside Permission observation window: %s", failure.at)
			default:
				return
			}
		}
		if !time.Now().UTC().Before(deadline) {
			t.Fatalf("expired Permission did not report fail-closed token refusal by %s", deadline)
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-t.Context().Done():
			timer.Stop()
			t.Fatal(t.Context().Err())
		case <-timer.C:
		}
	}
}

func TestInstalledCommandRefreshFailureTimeRequiresEventTimestamp(t *testing.T) {
	expires := time.Date(2026, time.September, 24, 22, 0, 0, 0, time.UTC)
	message := `{"kind":"headless-runtime-publication-refresh-failed","failure":"rotation-source-prefix-opening-entry-token-take-permission"}`
	for _, event := range []struct {
		name   string
		at     time.Time
		window installedCommandRefreshFailureWindow
	}{
		{name: "before expiry", at: expires.Add(-time.Microsecond), window: installedCommandRefreshFailureBeforeExpiry},
		{name: "at expiry", at: expires, window: installedCommandRefreshFailureAtExpiry},
		{name: "observation deadline", at: expires.Add(installedCommandRefreshObservationWindow), window: installedCommandRefreshFailureAtExpiry},
		{name: "after observation deadline", at: expires.Add(installedCommandRefreshObservationWindow + time.Microsecond), window: installedCommandRefreshFailureAfterWindow},
	} {
		t.Run(event.name, func(t *testing.T) {
			line, err := json.Marshal(installedCommandJournalEvent{Timestamp: strconv.FormatInt(event.at.UnixMicro(), 10), Message: message})
			if err != nil {
				t.Fatal(err)
			}
			failure, matching, err := installedCommandRefreshFailureTime(string(line))
			if err != nil || !matching || !failure.at.Equal(event.at) || failure.stage != "rotation-source-prefix-opening-entry-token-take-permission" {
				t.Fatalf("decode event: %s / %s / %t / %v", failure.at, failure.stage, matching, err)
			}
			if window := classifyInstalledCommandRefreshFailure(failure.at, expires); window != event.window {
				t.Fatalf("event window = %d, want %d", window, event.window)
			}
		})
	}
}

func TestInstalledCommandRefreshFailureParsesUnexpectedStage(t *testing.T) {
	line := `{"__REALTIME_TIMESTAMP":"1790287200000000","MESSAGE":"{\"kind\":\"headless-runtime-publication-refresh-failed\",\"failure\":\"registration-ended-protocol\"}"}`
	failure, matching, err := installedCommandRefreshFailureTime(line)
	if err != nil || !matching || failure.stage != "registration-ended-protocol" {
		t.Fatalf("unexpected stage = %q, matching=%t, err=%v", failure.stage, matching, err)
	}
}

// Observe only the real resolution Store's atomically replaced public record.
// This does not open its exclusive owner or observe Endpoint private keys.
func readInstalledCommandDescriptor(t *testing.T, root string, profile state.ClosedProfileView) reachability.Verified {
	t.Helper()
	raw := readInstalledCommandDescriptorRecord(t, root)
	value, err := reachability.VerifyPrivatePublication(raw[2:], profile.NetworkID, profile.Digest, time.Now().UTC())
	if err != nil {
		t.Fatalf("invalid signed Descriptor observation: %v", err)
	}
	return value
}

// This preserves the exact durable Store record for before/after comparisons
// even when its signed Descriptor is correctly no longer current.
func readInstalledCommandDescriptorRecord(t *testing.T, root string) []byte {
	t.Helper()
	directory := filepath.Join(root, "records")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var found []byte
	for _, entry := range entries {
		if len(entry.Name()) != 64 {
			continue
		}
		if !entry.Type().IsRegular() {
			t.Fatal("unexpected Descriptor record type")
		}
		file, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(file, reachability.MaximumPrivateDescriptorSize+3))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(raw) > reachability.MaximumPrivateDescriptorSize+2 || len(raw) < 3 {
			t.Fatalf("invalid persisted Descriptor observation: %v / %v", readErr, closeErr)
		}
		// The current Store's two-byte prefix records its version and conflict flags. A conflict
		// cannot count as successful refresh, even when the remaining proof verifies.
		if raw[0] != 2 || raw[1] != 0 {
			t.Fatal("conflicting publication observed")
		}
		if found != nil {
			t.Fatal("unexpected second Target in isolated resolution Store")
		}
		found = bytes.Clone(raw)
	}
	if found == nil {
		t.Fatal("publication command returned before a stored Descriptor existed")
	}
	return found
}

func observeInstalledCommandRefresh(t *testing.T, root string, profile state.ClosedProfileView, first reachability.Verified, commandStarted time.Time, invocation string) {
	t.Helper()
	original := first.Descriptor.Private
	// Thirty seconds is an observation/command-completion allowance, not an
	// extension to the selected 300-second schedule or predecessor validity.
	deadline := original.NotBefore.Add(330 * time.Second)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		current := readInstalledCommandDescriptor(t, root, profile)
		next := current.Descriptor.Private
		if next.Revision != original.Revision {
			if next.Revision != original.Revision+1 || next.Slot == original.Slot || next.RecipientKey == original.RecipientKey ||
				!bytes.Equal(first.Current.Record, current.Current.Record) || current.Descriptor.Target != first.Descriptor.Target {
				t.Fatal("refresh did not preserve publication with a fresh slot/key and next revision")
			}
			now := time.Now().UTC()
			if now.Before(commandStarted.Truncate(time.Second).Add(300*time.Second)) || now.After(deadline) {
				t.Fatal("refresh observed outside its schedule window")
			}
			t.Logf("verified real command refresh after %s", now.Sub(original.NotBefore))
			// A subsequent real read occurs after the predecessor overlap. This does
			// not independently inspect the old recipient's key erasure or ACK time.
			timer := time.NewTimer(65 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				return
			case <-t.Context().Done():
				t.Fatal("refresh overlap observation cancelled")
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("ordinary Endpoint did not refresh its stored Descriptor; endpoint diagnostic:\n%s", installedCommandRefreshFailure(t, invocation))
		}
		select {
		case <-tick.C:
		case <-t.Context().Done():
			t.Fatal("refresh observation cancelled")
		}
	}
}
