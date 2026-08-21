package updatetransaction

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// cleanupOps is the private per-invocation recovery operations seam.
// Only the cleanup fault/delay tests may inject custom remove, move,
// replace, and sync functions; public Recover always uses
// nativeCleanupOps. The seam cannot replace locking, inventory,
// decoding, validation, classification, clocks, deadlines, or Results.
type cleanupOps struct {
	removeFile           func(string) error
	removeDirectory      func(string) error
	moveDirectory        func(string, string) error
	atomicReplaceCurrent func(string, []byte, time.Time) error
	syncDirectory        func(string) error
}

func nativeCleanupOps() cleanupOps {
	return cleanupOps{
		removeFile:           os.Remove,
		removeDirectory:      os.Remove,
		moveDirectory:        moveDirectoryNative,
		atomicReplaceCurrent: atomicReplaceCurrentNative,
		syncDirectory:        syncDirectoryNative,
	}
}

func moveDirectoryNative(source, destination string) error {
	return nativeDurability().publishGeneration(source, destination)
}

func atomicReplaceCurrentNative(root string, payload []byte, deadline time.Time) error {
	if len(payload) == 0 {
		return errors.New("empty atomic replace payload")
	}
	var token [8]byte
	if _, err := io.ReadFull(rand.Reader, token[:]); err != nil {
		return err
	}
	temporary := filepath.Join(root, ".current."+hex.EncodeToString(token[:])+".tmp")
	if err := cleanupDeadline(deadline); err != nil {
		return err
	}
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(payload)
	if writeErr != nil || written != len(payload) {
		closeErr := file.Close()
		removeErr := removeTemporary(temporary, deadline)
		if writeErr == nil {
			writeErr = io.ErrShortWrite
		}
		return errors.Join(writeErr, closeErr, removeErr)
	}
	if deadlineErr := cleanupDeadline(deadline); deadlineErr != nil {
		return errors.Join(deadlineErr, file.Close())
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil || closeErr != nil {
		removeErr := removeTemporary(temporary, deadline)
		return errors.Join(syncErr, closeErr, removeErr)
	}
	if deadlineErr := cleanupDeadline(deadline); deadlineErr != nil {
		return deadlineErr
	}
	current := filepath.Join(root, "current")
	if err := nativeDurability().replaceCurrent(temporary, current); err != nil {
		removeErr := removeTemporary(temporary, deadline)
		return errors.Join(err, removeErr)
	}
	return nil
}

func removeTemporary(path string, deadline time.Time) error {
	if err := cleanupDeadline(deadline); err != nil {
		return err
	}
	return os.Remove(path)
}

func cleanupDeadline(deadline time.Time) error {
	if !time.Now().Before(deadline) {
		return errCleanupOverrun
	}
	return nil
}

func syncDirectoryNative(path string) error {
	return nativeDurability().syncDirectory(path)
}

// executePlan runs the deterministic cleanup plan with the supplied
// operations under a fixed continuation budget. Time is observed
// before and after every step; no new operation starts at or after
// expiry. It returns the elapsed time, the bounded observation slice,
// and the joined errors. When the budget expires before a step
// starts, it returns errCleanupOverrun so callers can return the
// bounded cleanup-incomplete Result without pretending the operation
// started.
func executePlan(root string, plan recoveryPlan, ops cleanupOps) error {
	deadline := time.Now().Add(5 * time.Second)
	for _, op := range plan.Operations {
		if !time.Now().Before(deadline) {
			return errCleanupOverrun
		}
		var stepErr error
		switch op.Kind {
		case opRemoveFile:
			stepErr = ops.removeFile(filepath.Join(root, op.Path))
		case opRemoveDirectory:
			stepErr = ops.removeDirectory(filepath.Join(root, op.Path))
		case opMoveDirectory:
			stepErr = ops.moveDirectory(filepath.Join(root, op.Path), filepath.Join(root, op.DestPath))
		case opAtomicReplaceCurrent:
			stepErr = ops.atomicReplaceCurrent(root, op.Payload, deadline)
		case opSyncDirectory:
			stepErr = ops.syncDirectory(filepath.Join(root, op.Path))
		default:
			stepErr = fmt.Errorf("unknown op %d", op.Kind)
		}
		if stepErr != nil {
			if !time.Now().Before(deadline) {
				return errors.Join(errCleanupOverrun, stepErr)
			}
			return stepErr
		}
		if !time.Now().Before(deadline) {
			return errCleanupOverrun
		}
	}
	return nil
}

var errCleanupOverrun = errors.New("update cleanup budget overrun")
