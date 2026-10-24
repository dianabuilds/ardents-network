//go:build linux && network_signing_native

package infrastructure

import (
	"bytes"
	"errors"
	"os"
	"runtime/debug"
	"strings"
	"testing"
)

// A native file mechanism counterexample, not a genuine Source/Release/State
// installation. The same record append/Sync/Close mechanism runs at every
// initial/replacement sequence. A fresh parser/lease sees exact canonical
// bytes/native identities but cannot recover the original in-memory error.
func TestSourceRecoveryOutcomeNativePrefixDoesNotProveOriginalSuccess(t *testing.T) {
	previous := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previous)
	for _, fault := range []string{"journal-sync-result-uncertain", "real-journal-close-failure"} {
		t.Run(fault, func(t *testing.T) {
			original, _, _, root := nativeSourceInitialPending(t, t.Context(), true)
			uncertain := errors.New("original journal completion uncertain after actual Sync")
			var visible []byte
			var identity os.FileInfo
			if fault == "journal-sync-result-uncertain" {
				original.afterSync = func(file *os.File) error {
					if file.Name() != original.journal.path {
						return nil
					}
					var err error
					visible, err = os.ReadFile(file.Name())
					if err != nil {
						return err
					}
					identity, err = file.Stat()
					if err != nil {
						return err
					}
					return uncertain
				}
			}
			err := original.record("admitted", root, nil, strings.Repeat("a", 64), 0, 999, 999)
			if fault == "journal-sync-result-uncertain" {
				if !errors.Is(err, uncertain) {
					t.Fatal("journal uncertainty lost", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				visible, err = os.ReadFile(original.journal.path)
				if err != nil {
					t.Fatal(err)
				}
				identity, err = original.journal.file.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if err := original.journal.file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			closed := original.close()
			if fault == "journal-sync-result-uncertain" && !errors.Is(closed, uncertain) || fault == "real-journal-close-failure" && !errors.Is(closed, os.ErrClosed) {
				t.Fatal("original failed result disappeared", closed)
			}
			fresh, err := openSourceRecoveryPrefix(t.Context(), original.journal.path, "initial")
			if err != nil {
				t.Fatal("fresh native framing did distinguish the failure", err)
			}
			defer fresh.close()
			if len(fresh.records) != 5 || fresh.records[4].Phase != "admitted" || !bytes.Equal(fresh.journal.raw, visible) || !sameSourceFile(identity, fresh.journal.info) {
				t.Fatal("fixture failed to isolate identical visible prefix/native facts")
			}
			pending, err := os.ReadFile(sourceInitialPendingPath(root))
			if err != nil || !bytes.Equal(pending, sourceInitialPending(strings.Repeat("a", 64))) {
				t.Fatal("original blocker unexpectedly records the failed outcome", err)
			}
			if err := fresh.close(); err != nil {
				t.Fatal("fresh custody unexpectedly carries historical in-memory error", err)
			}
			t.Log("canonical prefix + native identity + acquired lease did not prove original successful Sync/Close", fault)
		})
	}
}
