package installation

import (
	"context"
	"errors"
	"os"
)

func (owned *initialPreparation) archiveInitialIntent(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := owned.observeStoppedInstallation(ctx, request); err != nil {
		return err
	}
	if err := owned.stage.archiveIntent(ctx); err != nil {
		return err
	}
	return owned.observe(ctx, request)
}

// Copy/sync the exact owned intent before removing its original inode. A
// retained archive is refused here; explicit recovery has separate admission.
func (stage *generationStage) archiveIntent(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	if _, complete := stage.journal.files["0007.json"]; !complete || stage.archivedIntent != nil {
		return ErrBinding
	}
	if err := stage.journal.write(ctx, "completed-intent.json", stage.intent.body, 0600, 0); err != nil {
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
	archive := stage.journal.files["completed-intent.json"]
	stage.archivedIntent = &archive
	if err := syncStagingRoot(stage.lease.root); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func (stage *generationStage) observeIntent() error {
	if stage.archivedIntent == nil {
		return observeStagedFile(stage.lease.root, "transition.json", stage.intent)
	}
	if _, err := stage.lease.root.Lstat("transition.json"); !os.IsNotExist(err) {
		return errors.Join(ErrBinding, err)
	}
	return observeStagedFile(stage.journal.root, "completed-intent.json", *stage.archivedIntent)
}
