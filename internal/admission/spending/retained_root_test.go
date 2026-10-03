package spending

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIncompleteRetainedRootCannotInitialize(t *testing.T) {
	for _, missing := range []string{closedSpendLedgerName, closedSpendLockName} {
		t.Run(missing, func(t *testing.T) {
			root := t.TempDir()
			binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
			token := bytes.Repeat([]byte{42}, 354)
			ledger, err := Open(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			if err := ledger.Spend(token, window, window.Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := ledger.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, missing)
			retained := filepath.Join(t.TempDir(), missing)
			if err := os.Rename(path, retained); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				reopened, err := Open(root, binding)
				if reopened != nil {
					_ = reopened.Close()
				}
				if err == nil || reopened != nil {
					t.Fatal("incomplete retained root became usable")
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("refusal recreated missing state: %v", err)
				}
			}
			// Restore the exact original file only to prove failed Open released
			// its lease and never changed the retained burn. This is not recovery API.
			if err := os.Rename(retained, path); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.Spend(token, window, window.Add(2*time.Minute)); err == nil {
				t.Fatal("restored original root forgot its spend")
			}
		})
	}
}

func TestInterruptedRootCreationRefusesWithoutJournal(t *testing.T) {
	root := t.TempDir()
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	if err := os.WriteFile(filepath.Join(root, closedSpendLockName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		ledger, err := Open(root, binding)
		if ledger != nil {
			_ = ledger.Close()
		}
		if err == nil || ledger != nil {
			t.Fatal("retained lease without a journal initialized")
		}
		if _, err := os.Stat(filepath.Join(root, closedSpendLedgerName)); !os.IsNotExist(err) {
			t.Fatalf("journal created: %v", err)
		}
	}
}

func TestRootReopenRequiresDurabilityAfterFailedInitialization(t *testing.T) {
	root := t.TempDir()
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	failure := errors.New("root durability unavailable")
	for attempt := 0; attempt < 2; attempt++ {
		called := false
		ledger, err := openLedger(root, binding, func(string) error {
			called = true
			return failure
		})
		if ledger != nil {
			_ = ledger.Close()
		}
		if ledger != nil || !errors.Is(err, failure) || !called {
			t.Fatalf("attempt %d bypassed durability: owner=%p, error=%v, sync=%v", attempt, ledger, err, called)
		}
	}
	ledger, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	token := bytes.Repeat([]byte{42}, 354)
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err == nil {
		t.Fatal("durable root accepted duplicate spend")
	}
}
