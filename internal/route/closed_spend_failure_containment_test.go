package route

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedSpendLedgerFailedAppendTerminalizesOpenOwner(t *testing.T) {
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	root := t.TempDir()
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	ledger, err := OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })

	path := ledger.path
	ledger.path = t.TempDir()
	first := make([]byte, 354)
	first[0] = 1
	firstErr := ledger.Spend(first, window, window.Add(time.Minute))
	if firstErr == nil {
		t.Fatal("failed append admitted a token")
	}
	ledger.path = path

	second := make([]byte, 354)
	second[0] = 2
	if err := ledger.Spend(second, window, window.Add(time.Minute)); !errors.Is(err, firstErr) {
		t.Fatalf("second Spend = %v, want retained first failure %v", err, firstErr)
	}
}

func TestClosedSpendLedgerAppendFailuresTerminalizeOpenOwner(t *testing.T) {
	for _, failure := range []struct {
		name string
		file closedSpendAppendFile
	}{
		{name: "write", file: &closedSpendAppendFailure{writeErr: errors.New("write failed")}},
		{name: "first sync", file: &closedSpendAppendFailure{syncAt: 1, syncErr: errors.New("sync failed")}},
		{name: "commit", file: &closedSpendAppendFailure{writeAtErr: errors.New("commit failed")}},
		{name: "commit sync", file: &closedSpendAppendFailure{syncAt: 2, syncErr: errors.New("commit sync failed")}},
		{name: "close", file: &closedSpendAppendFailure{closeErr: errors.New("close failed")}},
	} {
		t.Run(failure.name, func(t *testing.T) {
			ledger, window := closedSpendFailureFixture(t)
			file := failure.file.(*closedSpendAppendFailure)
			ledger.openAppendFile = func(string) (closedSpendAppendFile, error) { return file, nil }
			firstErr := ledger.Spend(closedSpendFailureToken(1), window, window.Add(time.Minute))
			if firstErr == nil {
				t.Fatal("failed append admitted a token")
			}
			if file.closeCalls != 1 {
				t.Fatalf("close calls = %d, want 1", file.closeCalls)
			}
			mutations := file.openMutationCalls()
			if err := ledger.Spend(closedSpendFailureToken(2), window, window.Add(time.Minute)); !errors.Is(err, firstErr) {
				t.Fatalf("second Spend = %v, want retained first failure %v", err, firstErr)
			}
			if file.openMutationCalls() != mutations {
				t.Fatal("terminal owner attempted another append")
			}
		})
	}
}

func TestClosedSpendLedgerPostCommitFailureRetainsCommittedSpendAfterReopen(t *testing.T) {
	for _, failure := range []struct {
		name string
		wrap func(*os.File) closedSpendAppendFile
	}{
		{name: "commit sync", wrap: func(file *os.File) closedSpendAppendFile {
			return &closedSpendAppendOSFile{File: file, syncAt: 2, syncErr: errors.New("commit sync failed")}
		}},
		{name: "close", wrap: func(file *os.File) closedSpendAppendFile {
			return &closedSpendAppendOSFile{File: file, closeErr: errors.New("close failed")}
		}},
	} {
		t.Run(failure.name, func(t *testing.T) {
			ledger, window := closedSpendFailureFixture(t)
			path, binding := ledger.path, ledger.binding
			ledger.openAppendFile = func(path string) (closedSpendAppendFile, error) {
				file, err := os.OpenFile(path, os.O_RDWR, 0)
				if err != nil {
					return nil, err
				}
				return failure.wrap(file), nil
			}
			token := closedSpendFailureToken(1)
			firstErr := ledger.Spend(token, window, window.Add(time.Minute))
			if firstErr == nil {
				t.Fatal("post-commit failure admitted a token")
			}
			if err := ledger.Spend(closedSpendFailureToken(2), window, window.Add(time.Minute)); !errors.Is(err, firstErr) {
				t.Fatalf("later Spend = %v, want retained first failure %v", err, firstErr)
			}
			if err := ledger.Close(); !errors.Is(err, firstErr) {
				t.Fatalf("Close = %v, want %v", err, firstErr)
			}
			reopened, err := OpenClosedSpendLedger(filepath.Dir(path), binding)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err := reopened.Spend(token, window, window.Add(time.Minute)); err == nil {
				t.Fatal("safe reopen admitted the committed spend")
			}
		})
	}
}

func TestClosedSpendLedgerFailedPruneTerminalizesOpenOwnerAndReleasesLease(t *testing.T) {
	ledger, window := closedSpendFailureFixture(t)
	committed := closedSpendFailureToken(1)
	if err := ledger.Spend(committed, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	path := ledger.path
	ledger.path = t.TempDir()
	now := window.Add(time.Hour + time.Minute)
	pruneErr := ledger.Spend(closedSpendFailureToken(2), window.Add(time.Hour), now)
	if pruneErr == nil {
		t.Fatal("failed modifying prune admitted a token")
	}
	if err := ledger.Spend(closedSpendFailureToken(3), window.Add(time.Hour), now); !errors.Is(err, pruneErr) {
		t.Fatalf("Spend after failed prune = %v, want first failure %v", err, pruneErr)
	}
	ledger.path = path
	root, binding := ledger.path, ledger.binding
	if err := ledger.Close(); !errors.Is(err, pruneErr) {
		t.Fatalf("Close = %v, want retained failure %v", err, pruneErr)
	}
	reopened, err := OpenClosedSpendLedger(filepath.Dir(root), binding)
	if err != nil {
		t.Fatalf("Close did not release lease: %v", err)
	}
	defer reopened.Close()
	if err := reopened.Spend(committed, window, window.Add(time.Minute)); err == nil {
		t.Fatal("safe reopen admitted a committed spend")
	}
}

func closedSpendFailureFixture(t *testing.T) (*ClosedSpendLedger, time.Time) {
	t.Helper()
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	ledger, err := OpenClosedSpendLedger(t.TempDir(), ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	return ledger, window
}

func closedSpendFailureToken(marker byte) []byte {
	token := make([]byte, 354)
	token[0] = marker
	return token
}

type closedSpendAppendFailure struct {
	writeErr, writeAtErr, syncErr, closeErr                 error
	syncAt, syncCalls, writeCalls, writeAtCalls, closeCalls int
}

func (file *closedSpendAppendFailure) Seek(int64, int) (int64, error) { return 0, nil }
func (file *closedSpendAppendFailure) Write(raw []byte) (int, error) {
	file.writeCalls++
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return len(raw), nil
}
func (file *closedSpendAppendFailure) Sync() error {
	file.syncCalls++
	if file.syncCalls == file.syncAt {
		return file.syncErr
	}
	return nil
}
func (file *closedSpendAppendFailure) WriteAt(raw []byte, _ int64) (int, error) {
	file.writeAtCalls++
	if file.writeAtErr != nil {
		return 0, file.writeAtErr
	}
	return len(raw), nil
}
func (file *closedSpendAppendFailure) Close() error { file.closeCalls++; return file.closeErr }
func (file *closedSpendAppendFailure) openMutationCalls() int {
	return file.writeCalls + file.writeAtCalls
}

type closedSpendAppendOSFile struct {
	*os.File
	syncAt, syncCalls int
	syncErr, closeErr error
}

func (file *closedSpendAppendOSFile) Sync() error {
	file.syncCalls++
	if file.syncCalls == file.syncAt {
		return file.syncErr
	}
	return file.File.Sync()
}
func (file *closedSpendAppendOSFile) Close() error {
	return errors.Join(file.File.Close(), file.closeErr)
}

var _ io.Seeker = (*closedSpendAppendFailure)(nil)
