package spending

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestSpendPruningNeverRevivesEarlierWindow(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "same-process"
		if restart {
			name = "restart"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			window := time.Unix(1800000000, 0).UTC().Truncate(time.Hour)
			first, next, fresh := bytes.Repeat([]byte{1}, 354), bytes.Repeat([]byte{2}, 354), bytes.Repeat([]byte{3}, 354)
			ledger, err := Open(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ledger.Close() }()
			if err := ledger.Spend(first, window, window.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			later := window.Add(time.Hour + 2*time.Minute)
			if err := ledger.Spend(next, window.Add(time.Hour), later); err != nil {
				t.Fatal(err)
			}
			if len(ledger.spent) != 1 {
				t.Fatal("earlier spend was not pruned")
			}
			if restart {
				if err := ledger.Close(); err != nil {
					t.Fatal(err)
				}
				ledger, err = Open(root, binding)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := ledger.Spend(fresh, window.Add(time.Hour), later.Add(time.Second)); err != nil {
				t.Fatalf("current window unavailable: %v", err)
			}
			before, err := os.ReadFile(ledger.path)
			if err != nil {
				t.Fatal(err)
			}
			if err := ledger.Spend(first, window, window.Add(time.Minute)); err == nil {
				t.Fatal("backward time revived spent token")
			}
			after, err := os.ReadFile(ledger.path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("rollback refusal mutated journal: %v", err)
			}
			if err := ledger.Spend(bytes.Repeat([]byte{4}, 354), window, window.Add(2*time.Minute)); err == nil {
				t.Fatal("backward time admitted a fresh token in expired window")
			}
		})
	}
}

func TestSpendPruningCrashBeforeNextAppendRetainsFloor(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	root := t.TempDir()
	window := time.Unix(1800000000, 0).UTC().Truncate(time.Hour)
	token := bytes.Repeat([]byte{1}, 354)
	ledger, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	// Persist only compaction, simulating loss before the next spend append.
	if err := ledger.prune(window.Add(time.Hour + time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	ledger, err = Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ledger.Close() }()
	if len(ledger.spent) != 0 {
		t.Fatal("compaction did not remove expired record")
	}
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err == nil {
		t.Fatal("compaction without a subsequent spend lost the time floor")
	}
}

func TestSpendRecoveryRejectsInvalidPruningFloor(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	window := time.Unix(1800000000, 0).UTC().Truncate(time.Hour)
	floor := make([]byte, closedSpendRecordSize)
	binary.BigEndian.PutUint64(floor[32:40], uint64(window.Add(time.Hour+time.Minute).Unix()))
	floor[40] = 2
	for _, name := range []string{"zero", "overflow", "digest", "duplicate", "after-spend"} {
		t.Run(name, func(t *testing.T) {
			record := bytes.Clone(floor)
			raw := encodeClosedSpendHeader(binding)
			switch name {
			case "zero":
				clear(record[32:40])
			case "overflow":
				binary.BigEndian.PutUint64(record[32:40], 1<<63)
			case "digest":
				record[0] = 1
			case "duplicate":
				raw = append(raw, floor...)
			case "after-spend":
				raw = append(raw, closedSpendCommittedRecord([32]byte{1}, window)...)
			}
			raw = append(raw, record...)
			repaired := false
			ledger, err := decodeLedgerWithRepair("journal", binding, raw, func(string, int64) error { repaired = true; return nil })
			if err == nil || ledger != nil || repaired {
				t.Fatalf("invalid floor accepted or repaired: %v / %v / %v", ledger, err, repaired)
			}
		})
	}
}
