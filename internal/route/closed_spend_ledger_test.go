package route

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
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

func TestClosedSpendLedgerRecoveryClassifiesOneFinalCrashTail(t *testing.T) {
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	for _, test := range []struct {
		name     string
		tail     []byte
		wantOpen bool
		wantSize int
	}{
		{name: "uncommitted final record", tail: make([]byte, closedSpendRecordSize), wantOpen: true},
		{name: "partial final record", tail: make([]byte, 7), wantOpen: true},
		{name: "complete after incomplete", tail: append(make([]byte, closedSpendRecordSize), closedSpendCommittedRecord([32]byte{7}, window)...), wantSize: closedSpendRecordSize * 2},
		{name: "partial after incomplete", tail: append(make([]byte, closedSpendRecordSize), make([]byte, 7)...), wantSize: closedSpendRecordSize + 7},
		{name: "invalid commit marker", tail: append(make([]byte, closedSpendRecordSize-1), 2), wantSize: closedSpendRecordSize},
		{name: "corrupt committed before partial tail", tail: append(append(make([]byte, closedSpendRecordSize-1), 1), make([]byte, 7)...), wantSize: closedSpendRecordSize + 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			ledger, err := OpenClosedSpendLedger(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			token := make([]byte, 354)
			token[0] = 1
			if err := ledger.Spend(token, window, window.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := ledger.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, closedSpendLedgerName)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(before, test.tail...), 0o600); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenClosedSpendLedger(root, binding)
			if test.wantOpen {
				if err != nil {
					t.Fatalf("OpenClosedSpendLedger() = %v", err)
				}
				if err := reopened.Spend(token, window, window.Add(time.Minute)); err == nil {
					t.Fatal("safe recovery revived a committed spend")
				}
				got, readErr := os.ReadFile(path)
				if readErr != nil || !bytes.Equal(got, before) {
					t.Fatalf("recovered journal = %d bytes / %v, want original bytes", len(got), readErr)
				}
				if err := reopened.Close(); err != nil {
					t.Fatal(err)
				}
				again, err := OpenClosedSpendLedger(root, binding)
				if err != nil {
					t.Fatalf("idempotent reopen = %v", err)
				}
				defer again.Close()
				if err := again.Spend(token, window, window.Add(time.Minute)); err == nil {
					t.Fatal("idempotent reopen revived a committed spend")
				}
				return
			}
			if err == nil {
				_ = reopened.Close()
				t.Fatal("ambiguous journal opened")
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(got, append(before, test.tail...)) {
				t.Fatalf("startup refusal changed journal: %d bytes / %v", len(got), readErr)
			}
		})
	}
}

func TestClosedSpendLedgerRecoveryRefusesRepairFailure(t *testing.T) {
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	repairErr := errors.New("truncate sync failed")
	calls := 0
	ledger, err := decodeClosedSpendLedgerWithRepair("journal", binding, append(encodeClosedSpendHeader(binding), 1), func(string, int64) error {
		calls++
		return repairErr
	})
	if ledger != nil || !errors.Is(err, repairErr) || calls != 1 {
		t.Fatalf("recovery = %v / %v after %d repair calls, want nil owner and repair failure once", ledger, err, calls)
	}
}

func closedSpendCommittedRecord(digest [32]byte, window time.Time) []byte {
	if digest == [32]byte{} {
		digest = sha256.Sum256([]byte("closed spend test"))
	}
	record := make([]byte, closedSpendRecordSize)
	copy(record, digest[:])
	binary.BigEndian.PutUint64(record[32:40], uint64(window.Unix()))
	record[40] = 1
	return record
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
