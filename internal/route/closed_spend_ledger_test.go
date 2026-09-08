package route

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedSpendLedgerDurablyRefusesDuplicateAndRecoversPartialTail(t *testing.T) {
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	root := t.TempDir()
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	token := make([]byte, 354)
	token[0] = 1
	ledger, err := OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err == nil {
		t.Fatal("spent a token twice in one process")
	}
	if _, err := OpenClosedSpendLedger(root, binding); err == nil {
		t.Fatal("opened a second receiver spend owner")
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Spend(token, window, window.Add(time.Minute)); err == nil {
		t.Fatal("spent a token twice after restart")
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, closedSpendLedgerName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(make([]byte, closedSpendRecordSize)); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatalf("recover uncommitted tail: %v", err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenClosedSpendLedger(root, ClosedSpendBinding{NetworkID: [32]byte{9}, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.ReceiverNodeID, ReceiverDutyGeneration: binding.ReceiverDutyGeneration}); err == nil {
		t.Fatal("rebound a receiver spend journal")
	}
}

func TestClosedSpendLedgerPrunesOnlyAfterWindowMargin(t *testing.T) {
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	first, second := make([]byte, 354), make([]byte, 354)
	first[0], second[0] = 1, 2
	ledger, err := OpenClosedSpendLedger(t.TempDir(), binding)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ledger.Close() }()
	if err := ledger.Spend(first, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend(second, window, window.Add(time.Hour-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.prune(window.Add(time.Hour + 59*time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(ledger.spent) != 2 {
		t.Fatal("pruned a spend before its safety margin")
	}
	if err := ledger.prune(window.Add(time.Hour + time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(ledger.spent) != 0 {
		t.Fatal("retained a spend after its safety margin")
	}
}
