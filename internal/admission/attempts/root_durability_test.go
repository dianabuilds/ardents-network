//go:build linux

package attempts

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestReopenRepeatsFailedInitialDurability(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	failure := errors.New("root flush unavailable")
	for attempt := 0; attempt < 2; attempt++ {
		called := false
		journal, err := openJournal(root, [32]byte{1}, clock, func(string) error {
			called = true
			return failure
		})
		if journal != nil {
			_ = journal.Close()
		}
		if journal != nil || !errors.Is(err, failure) || !called {
			t.Fatalf("attempt %d bypassed persistence: owner=%p, error=%v, sync=%v", attempt, journal, err, called)
		}
	}
	journal, err := Open(root, [32]byte{1}, clock)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	record := Attempt{Profile: [32]byte{2}, Receiver: [32]byte{3}, Duty: 4, Window: now.Truncate(time.Hour), Class: 2, Nonce: [32]byte{5}}
	token := bytes.Repeat([]byte{6}, 354)
	if err := journal.Mark(token, record); err != nil {
		t.Fatal(err)
	}
	if err := journal.Mark(token, record); err == nil {
		t.Fatal("durable root accepted duplicate potential spend")
	}
}
