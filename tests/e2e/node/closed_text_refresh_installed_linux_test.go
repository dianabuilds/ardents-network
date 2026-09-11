//go:build linux && text_worker_installed

package state_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Observe only the real resolution Store's atomically replaced public record.
// This does not open its exclusive owner or observe Endpoint private keys.
func readInstalledCommandDescriptor(t *testing.T, root string, profile state.ClosedProfileView) reachability.Verified {
	t.Helper()
	directory := filepath.Join(root, "records")
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	var found *reachability.Verified
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
		value, err := reachability.VerifyPrivatePublication(raw[2:], profile.NetworkID, profile.Digest, time.Now().UTC())
		if err != nil {
			t.Fatalf("invalid signed Descriptor observation: %v", err)
		}
		if found != nil {
			t.Fatal("unexpected second Target in isolated resolution Store")
		}
		found = &value
	}
	if found == nil {
		t.Fatal("publication command returned before a stored Descriptor existed")
	}
	return *found
}

func observeInstalledCommandRefresh(t *testing.T, root string, profile state.ClosedProfileView, first reachability.Verified, commandStarted time.Time) {
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
			t.Fatalf("ordinary Endpoint did not refresh its stored Descriptor; endpoint diagnostic:\n%s", installedCommandRefreshFailure(t))
		}
		select {
		case <-tick.C:
		case <-t.Context().Done():
			t.Fatal("refresh observation cancelled")
		}
	}
}
