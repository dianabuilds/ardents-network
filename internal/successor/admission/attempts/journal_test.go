//go:build linux

package attempts

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tokenJournalFixture(t *testing.T) (*Journal, *time.Time, Attempt, []byte) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	journal, err := Open(t.TempDir(), [32]byte{1}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	record := Attempt{Profile: [32]byte{2}, Receiver: [32]byte{3}, Duty: 4, Window: now.Truncate(time.Hour), Class: 2, Nonce: [32]byte{5}}
	t.Cleanup(func() { _ = journal.Close() })
	return journal, &now, record, bytes.Repeat([]byte{6}, 354)
}

func TestTextTokenJournalPersistsBurnWithoutSecretsOrReplay(t *testing.T) {
	journal, _, record, token := tokenJournalFixture(t)
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(journal.root, "attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != journalHeader+attemptSize || bytes.Contains(raw, token) {
		t.Fatal("journal shape or secret retention wrong")
	}
	if other, err := Open(journal.root, journal.network, journal.clock); err == nil {
		_ = other.Close()
		t.Fatal("concurrent journal ownership")
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(journal.root, journal.network, journal.clock)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record.Nonce[0]++
	if err := reopened.Mark(token, record); err == nil {
		t.Fatal("restart revived potentially spent token")
	}
	if len(reopened.records) != 1 {
		t.Fatal("replay appended another receipt")
	}
}

func TestTextTokenJournalRefusesMissingPartialOrForeignRetainedState(t *testing.T) {
	for _, fault := range []string{"missing", "partial", "foreign", "clock"} {
		t.Run(fault, func(t *testing.T) {
			journal, now, record, token := tokenJournalFixture(t)
			if err := journal.Mark(token, record); err != nil {
				t.Fatal(err)
			}
			if err := journal.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(journal.root, "attempts")
			switch fault {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "partial":
				file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, err = file.Write([]byte{1})
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				journal.network[0]++
			case "clock":
				*now = now.Add(-time.Second)
			}
			if reopened, err := Open(journal.root, journal.network, journal.clock); err == nil {
				_ = reopened.Close()
				t.Fatal("ambiguous state reset")
			}
		})
	}
}

func TestTextTokenJournalPoisonsOwnerOnReplacedFile(t *testing.T) {
	journal, _, record, token := tokenJournalFixture(t)
	path := filepath.Join(journal.root, "attempts")
	if err := os.Rename(path, path+".replaced"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, journal.header(journal.floor), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := journal.Mark(token, record); err == nil {
		t.Fatal("replacement accepted")
	}
	if len(journal.records) != 0 || journal.failure == nil {
		t.Fatal("failed mark consumed no durable authority but owner stayed usable")
	}
	token[0]++
	if err := journal.Mark(token, record); err == nil {
		t.Fatal("failed owner recovered silently")
	}
	if err := journal.Close(); err == nil {
		t.Fatal("cleanup erased failure")
	}
}

func TestTextTokenJournalPrunesOnlyAfterWindowCleanupMargin(t *testing.T) {
	journal, now, record, token := tokenJournalFixture(t)
	defer journal.Close()
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	*now = record.Window.Add(time.Hour + 59*time.Second)
	record.Window = record.Window.Add(time.Hour)
	token[0]++
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	if len(journal.records) != 2 {
		t.Fatal("pruned before cleanup margin")
	}
	*now = now.Add(time.Second)
	token[0]++
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	if len(journal.records) != 2 {
		t.Fatal("expired receipt not pruned")
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(journal.root, journal.network, journal.clock)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.records) != 2 || reopened.floor != now.Truncate(time.Second) {
		t.Fatal("compaction lost retained receipts or time floor")
	}
}

// The earliest-expiry fence must retain exactly what a full scan retains and
// keep earliestWindow equal to the minimum retained window (audit AD-4).
func TestTextTokenJournalPruneFenceMatchesFullScan(t *testing.T) {
	journal, now, record, token := tokenJournalFixture(t)
	defer journal.Close()
	base := record.Window
	*now = base.Add(30 * time.Minute)
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	// Second window, spent inside the one-minute overlap so both coexist.
	*now = base.Add(time.Hour + 30*time.Second)
	record.Window = base.Add(time.Hour)
	token[0]++
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	if !journal.earliestWindow.Equal(base) {
		t.Fatalf("earliestWindow=%s want=%s", journal.earliestWindow, base)
	}
	windows := []time.Time{base, base.Add(time.Hour)}
	retainedAt := func(at time.Time) int {
		count := 0
		for _, window := range windows {
			if at.Before(window.Add(time.Hour + time.Minute)) {
				count++
			}
		}
		return count
	}
	for _, at := range []time.Time{
		base.Add(time.Hour + 30*time.Second),
		base.Add(time.Hour + 59*time.Second),
		base.Add(time.Hour + time.Minute),
		base.Add(2*time.Hour + time.Minute),
	} {
		if err := journal.prune(at); err != nil {
			t.Fatal(err)
		}
		if len(journal.records) != retainedAt(at) {
			t.Fatalf("at=%s retained=%d want=%d", at, len(journal.records), retainedAt(at))
		}
		var earliest time.Time
		for _, rec := range journal.records {
			if earliest.IsZero() || rec.Window.Before(earliest) {
				earliest = rec.Window
			}
		}
		if !journal.earliestWindow.Equal(earliest) {
			t.Fatalf("at=%s earliestWindow=%s want=%s", at, journal.earliestWindow, earliest)
		}
	}
}

func TestTextTokenJournalPruningCrashRetainsDeletionTimeFloor(t *testing.T) {
	journal, now, record, token := tokenJournalFixture(t)
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	pruningTime := record.Window.Add(time.Hour + time.Minute)
	*now = pruningTime
	// Stop at the durable compaction boundary, before the next append.
	if err := journal.prune(pruningTime); err != nil {
		t.Fatal(err)
	}
	if len(journal.records) != 0 {
		t.Fatal("expired receipt retained")
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	*now = record.Window.Add(30 * time.Minute)
	if reopened, err := Open(journal.root, journal.network, journal.clock); err == nil {
		defer reopened.Close()
		t.Fatal("restart accepted time before durable receipt deletion")
	}
	*now = pruningTime
	reopened, err := Open(journal.root, journal.network, journal.clock)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.floor != pruningTime || len(reopened.records) != 0 {
		t.Fatal("compaction did not persist its deletion floor")
	}
}
