package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	sourceRecoverableInitialPhase   = "initial-recoverable"
	sourceRecoverableInitialRecords = 115
)

func sourceInitialPendingPath(root string) string {
	return root + ".source-initial-pending"
}

func sourceInitialPending(binding string) []byte {
	return []byte("ardents-source-initial-pending-v1\n" + binding + "\n")
}

// An ordinary consumer cannot bypass a pending initial operation. Only the
// actual original writer may read back its own complete blocked transcript.
func observeSourceInitialPending(root string, transition *sourceBirths) error {
	path := sourceInitialPendingPath(root)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return errors.Join(ErrSourceRepairRequired, err)
	}
	if transition == nil || transition.journal == nil || transition.journal.path != root+".source-initial-journal" || transition.firstPhase != sourceRecoverableInitialPhase || transition.ctx == nil {
		return ErrSourceRepairRequired
	}
	if err := transition.ctx.Err(); err != nil {
		return err
	}
	for _, original := range transition.files {
		if original.path == path {
			return original.check()
		}
	}
	return ErrSourceRepairRequired
}

// Only physical publication lives here. The caller separately checks fresh
// Release and owns the same original pending birth, joined inputs and operation.
// No required Installation effect follows unlink; a lost unlink restores denial.
func publishSourceInitialPending(ctx context.Context, births *sourceBirths, pending *sourceBirth, parent os.FileInfo) error {
	if ctx == nil || births == nil || pending == nil || parent == nil || !births.closed || births.terminal != nil {
		if births != nil {
			return errors.Join(ErrBinding, births.terminal)
		}
		return ErrInput
	}
	owned := false
	for _, original := range births.files {
		if original == pending {
			owned = true
		}
	}
	if !owned || births.journal == nil || births.firstPhase != sourceRecoverableInitialPhase || !strings.HasSuffix(births.journal.path, ".source-initial-journal") {
		return ErrBinding
	}
	root := strings.TrimSuffix(births.journal.path, ".source-initial-journal")
	if pending.path != sourceInitialPendingPath(root) {
		return ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	namedParent, err := os.Lstat(filepath.Dir(pending.path))
	if err != nil || !os.SameFile(parent, namedParent) || !sourceAccess(namedParent, parent.Mode().Perm(), 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	named, err := os.Lstat(pending.path)
	if err != nil || !sameSourceFile(pending.info, named) {
		return errors.Join(ErrBinding, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Remove(pending.path)
}
