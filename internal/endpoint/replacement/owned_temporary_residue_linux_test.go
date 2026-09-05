//go:build linux

package replacement

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errAtomicWriteInterrupted = errors.New("atomic writer interrupted")

func TestAtomicWriterTemporaryResiduePreservesCommittedProgramVerificationAndRecovery(t *testing.T) {
	stateRoot, programPath, committed := committedReplacementForResidue(t)
	for _, destination := range temporaryEntryDestinations {
		destination := destination
		t.Run(destination.name, func(t *testing.T) {
			mode := os.FileMode(0o600)
			if destination.name == rollbackName {
				mode = 0o700
			}
			var residuePath string
			err := writeAtomicModeWithInterruption(stateRoot, destination.name, []byte("interrupted "+destination.name), mode,
				func(path string, checkpoint atomicWriteCheckpoint) error {
					if checkpoint == atomicWriteTemporarySynced {
						residuePath = path
						return errAtomicWriteInterrupted
					}
					return nil
				})
			if !errors.Is(err, errAtomicWriteInterrupted) {
				t.Fatalf("writeAtomicModeWithInterruption() error = %v", err)
			}
			if _, err := os.Lstat(residuePath); err != nil {
				t.Fatalf("interrupted writer residue: %v", err)
			}
			running, err := VerifyRunning(stateRoot, programPath)
			if err != nil || running.State != StateCurrent || running.Record.Digest != committed.Digest {
				t.Fatalf("VerifyRunning() = %+v, %v", running, err)
			}
			firstRecovery, err := Recover(stateRoot, programPath)
			if err != nil || firstRecovery.State != string(StateCurrent) || firstRecovery.Current.Digest != committed.Digest {
				t.Fatalf("first Recover() = %+v, %v", firstRecovery, err)
			}
			secondRecovery, err := Recover(stateRoot, programPath)
			if err != nil || secondRecovery != firstRecovery {
				t.Fatalf("second Recover() = %+v, %v; first = %+v", secondRecovery, err, firstRecovery)
			}
			if _, err := os.Lstat(residuePath); err != nil {
				t.Fatalf("read-only verification/recovery removed residue: %v", err)
			}
			if err := os.Remove(residuePath); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAtomicWriterInterruptionAfterJournalRenameIsClassifiedWithoutMutation(t *testing.T) {
	stateRoot, programPath, committed := committedReplacementForResidue(t)
	predecessor := sha256.Sum256([]byte("retained predecessor"))
	journal := []byte(fmt.Sprintf("schema=ardents-endpoint-replacement-journal-v1\nphase=committed\nprogram_path=%s\npredecessor=%x\ncandidate=%x\n", programPath, predecessor, committed.Digest))
	err := writeAtomicModeWithInterruption(stateRoot, journalName, journal, 0o600,
		func(_ string, checkpoint atomicWriteCheckpoint) error {
			if checkpoint == atomicWriteRenamed {
				return errAtomicWriteInterrupted
			}
			return nil
		})
	if !errors.Is(err, errAtomicWriteInterrupted) {
		t.Fatalf("writeAtomicModeWithInterruption() error = %v", err)
	}
	running, err := VerifyRunning(stateRoot, programPath)
	if err != nil || running.State != StateCurrent || running.Record.Digest != committed.Digest {
		t.Fatalf("VerifyRunning() = %+v, %v", running, err)
	}
	firstRecovery, err := Recover(stateRoot, programPath)
	if err != nil || firstRecovery.State != "committed-restart-permitted" || firstRecovery.Current.Digest != committed.Digest {
		t.Fatalf("first Recover() = %+v, %v", firstRecovery, err)
	}
	secondRecovery, err := Recover(stateRoot, programPath)
	if err != nil || secondRecovery != firstRecovery {
		t.Fatalf("second Recover() = %+v, %v; first = %+v", secondRecovery, err, firstRecovery)
	}
	if _, err := os.Lstat(filepath.Join(stateRoot, journalName)); err != nil {
		t.Fatalf("read-only recovery removed renamed journal: %v", err)
	}
}

func TestOwnedTemporaryResidueLeavesInconsistentJournalRepairRequired(t *testing.T) {
	stateRoot, programPath, committed := committedReplacementForResidue(t)
	residuePath := interruptedTemporaryWriter(t, stateRoot, currentName)
	predecessor := sha256.Sum256([]byte("retained predecessor"))
	inconsistentCandidate := sha256.Sum256([]byte("inconsistent candidate"))
	journal := []byte(fmt.Sprintf("schema=ardents-endpoint-replacement-journal-v1\nphase=committed\nprogram_path=%s\npredecessor=%x\ncandidate=%x\n", programPath, predecessor, inconsistentCandidate))
	if err := writeAtomicModeWithInterruption(stateRoot, journalName, journal, 0o600,
		func(_ string, checkpoint atomicWriteCheckpoint) error {
			if checkpoint == atomicWriteRenamed {
				return errAtomicWriteInterrupted
			}
			return nil
		}); !errors.Is(err, errAtomicWriteInterrupted) {
		t.Fatalf("writeAtomicModeWithInterruption() error = %v", err)
	}
	running, err := VerifyRunning(stateRoot, programPath)
	if err != nil || running.State != StateCurrent || running.Record.Digest != committed.Digest {
		t.Fatalf("VerifyRunning() = %+v, %v", running, err)
	}
	firstRecovery, err := Recover(stateRoot, programPath)
	if err != nil || firstRecovery.State != "repair-required" || firstRecovery.Predecessor != predecessor {
		t.Fatalf("first Recover() = %+v, %v", firstRecovery, err)
	}
	secondRecovery, err := Recover(stateRoot, programPath)
	if err != nil || secondRecovery != firstRecovery {
		t.Fatalf("second Recover() = %+v, %v; first = %+v", secondRecovery, err, firstRecovery)
	}
	if _, err := os.Lstat(residuePath); err != nil {
		t.Fatalf("read-only recovery removed temporary residue: %v", err)
	}
	if retainedJournal, err := os.ReadFile(filepath.Join(stateRoot, journalName)); err != nil || string(retainedJournal) != string(journal) {
		t.Fatalf("read-only recovery changed inconsistent journal: %q, %v", retainedJournal, err)
	}
}

func TestOwnedTemporaryResidueRecoveryPreservesWriterLeaseAndReleaseFloors(t *testing.T) {
	fixture := replacementProtectedFixture(t, true)
	residuePath := interruptedTemporaryWriter(t, fixture.stateRoot, journalName)
	writer, err := openStore(fixture.stateRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.close() })
	result, recoveryErr := Recover(fixture.stateRoot, fixture.program)
	if recoveryErr != nil || result.State != string(StateCurrent) {
		t.Fatalf("Recover() = %+v, %v", result, recoveryErr)
	}
	secondResult, secondRecoveryErr := Recover(fixture.stateRoot, fixture.program)
	if secondRecoveryErr != nil || secondResult != result {
		t.Fatalf("second Recover() = %+v, %v; first = %+v", secondResult, secondRecoveryErr, result)
	}
	if contender, contenderErr := openStore(fixture.stateRoot, false); contenderErr == nil {
		_ = contender.close()
		t.Fatal("Recover() released the active writer lease")
	} else if !strings.Contains(contenderErr.Error(), "state is busy") {
		t.Fatalf("writer lease contention error = %v, want busy state", contenderErr)
	}
	if err := writer.close(); err != nil {
		t.Fatal(err)
	}
	contender, err := openStore(fixture.stateRoot, false)
	if err != nil {
		t.Fatalf("writer lease remained held after its owner closed: %v", err)
	}
	if err := contender.close(); err != nil {
		t.Fatal(err)
	}
	assertProtectedTreeUnchanged(t, fixture.vaultRoot, fixture.vaultBefore)
	assertProtectedTreeUnchanged(t, fixture.releaseRoot, fixture.releaseBefore)
	assertReleaseFloorRootRemainsValid(t, fixture)
	if _, err := os.Lstat(residuePath); err != nil {
		t.Fatalf("read-only recovery removed temporary residue: %v", err)
	}
}

func TestOwnedTemporaryResidueRejectsSymlink(t *testing.T) {
	root, programPath, _ := committedReplacementForResidue(t)
	target := filepath.Join(t.TempDir(), "foreign")
	if err := os.WriteFile(target, []byte("foreign file"), 0o600); err != nil {
		t.Fatal(err)
	}
	residuePath := filepath.Join(root, ".journal-0123456789abcdef.tmp")
	if err := os.Symlink(target, residuePath); err != nil {
		t.Fatal(err)
	}
	if _, err := openReadStore(root); err == nil {
		t.Fatal("openReadStore() accepted a symlink that matches an owned temporary name")
	}
	if _, err := VerifyRunning(root, programPath); err == nil {
		t.Fatal("VerifyRunning() accepted a symlink that matches an owned temporary name")
	}
	if _, err := Recover(root, programPath); err == nil {
		t.Fatal("Recover() accepted a symlink that matches an owned temporary name")
	}
	if _, err := os.Lstat(residuePath); err != nil {
		t.Fatalf("read-only verification/recovery removed symlink: %v", err)
	}
}

func committedReplacementForResidue(t *testing.T) (string, string, Record) {
	t.Helper()
	workspace := t.TempDir()
	programPath := filepath.Join(workspace, "ardents")
	program := []byte("committed Endpoint program")
	if err := os.WriteFile(programPath, program, 0o700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(workspace, "replacement-state")
	if err := os.Mkdir(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: program, decision: replacementDecision(program, 1)}); err != nil {
		t.Fatal(err)
	}
	committed, err := CommitPrepared(stateRoot, programPath)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, programPath, committed
}

func interruptedTemporaryWriter(t *testing.T, stateRoot, destination string) string {
	t.Helper()
	var residuePath string
	err := writeAtomicModeWithInterruption(stateRoot, destination, []byte("interrupted "+destination), 0o600,
		func(path string, checkpoint atomicWriteCheckpoint) error {
			if checkpoint == atomicWriteTemporarySynced {
				residuePath = path
				return errAtomicWriteInterrupted
			}
			return nil
		})
	if !errors.Is(err, errAtomicWriteInterrupted) {
		t.Fatalf("writeAtomicModeWithInterruption() error = %v", err)
	}
	if _, err := os.Lstat(residuePath); err != nil {
		t.Fatalf("interrupted writer residue: %v", err)
	}
	return residuePath
}
