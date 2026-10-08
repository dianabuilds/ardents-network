package installation

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"os"
)

// Copy/sync the exact owned intent before removing its original inode. A
// retained archive is refused here; explicit recovery has separate admission.
func (stage *generationStage) archiveIntent(ctx context.Context) error {
	if ctx == nil || stage == nil || stage.journal == nil {
		return ErrInput
	}
	// Initial stopped completion cannot authorize successor archival. That
	// operation must observe its actual started invocation and completion peer.
	var intent initialTransitionIntent
	if decodeCanonical(stage.intent.body, 128<<10, &intent) != nil ||
		intent.Schema != "ardents-endpoint-installation-initial-v1" || intent.Candidate != stage.selected {
		return ErrBinding
	}
	if err := stage.verifyTransitionPhase("0007.json", "installed-stopped"); err != nil {
		return err
	}
	return stage.copyAndRemoveIntent(ctx)
}

// Physical archival only. Each caller admits its own exact completion before
// reaching this mechanism; an archive is not a process or startup proof.
func (stage *generationStage) copyAndRemoveIntent(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if len(stage.journal.Bytes(journal.Transitions, "0007.json")) == 0 || stage.archivedIntent {
		return ErrBinding
	}
	if err := stage.journal.Write(ctx, journal.Transitions, "completed-intent.json", stage.intent.body); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := observeStagedFile(stage.lease.root, "transition.json", stage.intent); err != nil {
		return err
	}
	if err := stage.lease.root.Remove("transition.json"); err != nil {
		return err
	}
	stage.archivedIntent = true
	if err := syncStagingRoot(stage.lease.root); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func (stage *generationStage) observeIntent() error {
	if !stage.archivedIntent {
		return observeStagedFile(stage.lease.root, "transition.json", stage.intent)
	}
	if _, err := stage.lease.root.Lstat("transition.json"); !os.IsNotExist(err) {
		return errors.Join(ErrBinding, err)
	}
	if !bytes.Equal(stage.journal.Bytes(journal.Transitions, "completed-intent.json"), stage.intent.body) {
		return ErrBinding
	}
	if err := stage.journal.Observe(); err != nil {
		return errors.Join(ErrBinding, err)
	}
	return nil
}
