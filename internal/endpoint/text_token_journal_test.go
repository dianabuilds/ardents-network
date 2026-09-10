//go:build linux

package endpoint

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tokenJournalFixture(t *testing.T) (*textTokenJournal, *time.Time, textTokenAttempt, []byte) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	journal, err := openTextTokenJournal(t.TempDir(), [32]byte{1}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	record := textTokenAttempt{profile: [32]byte{2}, receiver: [32]byte{3}, duty: 4, window: now.Truncate(time.Hour), class: 2, attempt: [32]byte{5}}
	t.Cleanup(func() { _ = journal.Close() })
	return journal, &now, record, bytes.Repeat([]byte{6}, 354)
}

func TestTextTokenJournalPersistsBurnWithoutSecretsOrReplay(t *testing.T) {
	journal, _, record, token := tokenJournalFixture(t)
	if err := journal.mark(token, record); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(journal.root, "attempts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != textTokenJournalHeader+textTokenAttemptSize || bytes.Contains(raw, token) {
		t.Fatal("journal shape or secret retention wrong")
	}
	if other, err := openTextTokenJournal(journal.root, journal.network, journal.clock); err == nil {
		_ = other.Close()
		t.Fatal("concurrent journal ownership")
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openTextTokenJournal(journal.root, journal.network, journal.clock)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record.attempt[0]++
	if err := reopened.mark(token, record); err == nil {
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
			if err := journal.mark(token, record); err != nil {
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
			if reopened, err := openTextTokenJournal(journal.root, journal.network, journal.clock); err == nil {
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
	if err := journal.mark(token, record); err == nil {
		t.Fatal("replacement accepted")
	}
	if len(journal.records) != 0 || journal.failure == nil {
		t.Fatal("failed mark consumed no durable authority but owner stayed usable")
	}
	token[0]++
	if err := journal.mark(token, record); err == nil {
		t.Fatal("failed owner recovered silently")
	}
	if err := journal.Close(); err == nil {
		t.Fatal("cleanup erased failure")
	}
}

func TestTextTokenJournalPrunesOnlyAfterWindowCleanupMargin(t *testing.T) {
	journal, now, record, token := tokenJournalFixture(t)
	defer journal.Close()
	if err := journal.mark(token, record); err != nil {
		t.Fatal(err)
	}
	*now = record.window.Add(time.Hour + 59*time.Second)
	record.window = record.window.Add(time.Hour)
	token[0]++
	if err := journal.mark(token, record); err != nil {
		t.Fatal(err)
	}
	if len(journal.records) != 2 {
		t.Fatal("pruned before cleanup margin")
	}
	*now = now.Add(time.Second)
	token[0]++
	if err := journal.mark(token, record); err != nil {
		t.Fatal(err)
	}
	if len(journal.records) != 2 {
		t.Fatal("expired receipt not pruned")
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openTextTokenJournal(journal.root, journal.network, journal.clock)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.records) != 2 || reopened.floor != now.Truncate(time.Second) {
		t.Fatal("compaction lost retained receipts or time floor")
	}
}

func TestTextTokenJournalPruningCrashRetainsDeletionTimeFloor(t *testing.T) {
	journal, now, record, token := tokenJournalFixture(t)
	if err := journal.mark(token, record); err != nil {
		t.Fatal(err)
	}
	pruningTime := record.window.Add(time.Hour + time.Minute)
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
	*now = record.window.Add(30 * time.Minute)
	if reopened, err := openTextTokenJournal(journal.root, journal.network, journal.clock); err == nil {
		defer reopened.Close()
		t.Fatal("restart accepted time before durable receipt deletion")
	}
	*now = pruningTime
	reopened, err := openTextTokenJournal(journal.root, journal.network, journal.clock)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.floor != pruningTime || len(reopened.records) != 0 {
		t.Fatal("compaction did not persist its deletion floor")
	}
}
