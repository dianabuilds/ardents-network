//go:build linux

package endpoint

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenJournalAdmissionRefusesMissingRetainedLease(t *testing.T) {
	root := t.TempDir()
	clock := func() time.Time { return time.Unix(1_800_000_000, 0).UTC() }
	first := &endpoint{network: [32]byte{1}, clock: clock}
	first.closedTokenRoot = root
	journal, err := first.tokenJournal()
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	if err := os.Rename(filepath.Join(root, "owner.lock"), filepath.Join(t.TempDir(), "retained-lock")); err != nil {
		t.Fatal(err)
	}
	second := &endpoint{network: first.network, clock: clock}
	second.closedTokenRoot = root
	for attempt := 0; attempt < 2; attempt++ {
		candidate, err := second.tokenJournal()
		if candidate != nil {
			_ = candidate.Close()
		}
		if err == nil || candidate != nil || second.closedTokenJournal != nil {
			t.Fatal("Endpoint retained a second journal after lease loss")
		}
		if _, err := os.Lstat(filepath.Join(root, "owner.lock")); !os.IsNotExist(err) {
			t.Fatalf("Endpoint recreated a missing lease: %v", err)
		}
	}
}
