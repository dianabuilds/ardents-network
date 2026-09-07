package contributor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func (profile *Profile) recoverInterruptedUpdate(ctx context.Context, record installationRecord) error {
	previousProgram := filepath.Join(profile.paths.programRoot, "previous")
	previousConfig := filepath.Join(profile.paths.configRoot, "previous")
	nextProgram := filepath.Join(profile.paths.programRoot, "next")
	nextConfig := filepath.Join(profile.paths.configRoot, "next")
	update, updateErr := readUpdateRecord(profile.paths.updating)
	if updateErr != nil && !errors.Is(updateErr, os.ErrNotExist) {
		return updateErr
	}
	if err := verifyInstalled(profile.paths, record); err == nil {
		if errors.Is(updateErr, os.ErrNotExist) {
			previous, previousErr := pairedDirectories(previousProgram, previousConfig)
			next, nextErr := pairedDirectories(nextProgram, nextConfig)
			if previousErr != nil || nextErr != nil {
				return errors.New("contributor update residue has no authenticated transition record")
			}
			if !previous && !next {
				return nil
			}
			return errors.Join(os.RemoveAll(previousProgram), os.RemoveAll(previousConfig),
				os.RemoveAll(nextProgram), os.RemoveAll(nextConfig))
		}
		if !sameInstallationRecord(update.Previous, record) &&
			(update.Previous.DeploymentID != record.DeploymentID || update.Previous.Generation+1 != record.Generation) {
			return errors.New("contributor update record conflicts with the committed generation")
		}
		if sameInstallationRecord(update.Previous, record) {
			if err := profile.restoreManagementExecutableFromCurrent(); err != nil {
				return err
			}
			state, err := profile.supervisor.Do(ctx, SupervisorStatus)
			if err != nil {
				return err
			}
			if !state.Active {
				if err := profile.restartCurrentGeneration(); err != nil {
					return err
				}
			}
		}
		if err := errors.Join(os.RemoveAll(previousProgram), os.RemoveAll(previousConfig),
			os.RemoveAll(nextProgram), os.RemoveAll(nextConfig)); err != nil {
			return err
		}
		return removeIfPresent(profile.paths.updating)
	}
	if updateErr == nil && !sameInstallationRecord(update.Previous, record) {
		return errors.New("contributor update record conflicts with the recovery generation")
	}
	if err := profile.completeInterruptedPreviousMove(previousProgram, previousConfig, record); err != nil {
		return err
	}
	previous, err := pairedDirectories(previousProgram, previousConfig)
	if err != nil {
		return err
	}
	if !previous {
		return errors.New("interrupted Contributor update cannot authenticate its current generation")
	}
	previousPaths := profile.paths
	previousPaths.programCurrent = previousProgram
	previousPaths.configCurrent = previousConfig
	if err := verifyInstalled(previousPaths, record); err != nil {
		return errors.New("interrupted Contributor update has no authenticated recovery generation")
	}
	before, err := profile.supervisor.Do(ctx, SupervisorStatus)
	if err != nil {
		return err
	}
	if _, err := profile.supervisor.Do(ctx, SupervisorStop); err != nil {
		return err
	}
	if before.Active {
		if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "WITHDRAWN", 15*time.Second); err != nil {
			return err
		}
	}
	if err := errors.Join(os.RemoveAll(profile.paths.programCurrent), os.RemoveAll(profile.paths.configCurrent)); err != nil {
		return err
	}
	if err := os.Rename(previousProgram, profile.paths.programCurrent); err != nil {
		return err
	}
	if err := os.Rename(previousConfig, profile.paths.configCurrent); err != nil {
		return errors.Join(err, os.Rename(profile.paths.programCurrent, previousProgram))
	}
	if err := errors.Join(os.RemoveAll(nextProgram), os.RemoveAll(nextConfig), removeIfPresent(profile.paths.lifecycle)); err != nil {
		return err
	}
	state, err := profile.supervisor.Do(ctx, SupervisorStart)
	if err != nil || !state.Active {
		return errors.Join(err, errors.New("recovered Contributor generation did not become active"))
	}
	if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "READY", 15*time.Second); err != nil {
		return err
	}
	if updateErr == nil && sameInstallationRecord(update.Previous, record) {
		if err := profile.restoreManagementExecutableFromCurrent(); err != nil {
			return err
		}
	}
	if err := verifyInstalled(profile.paths, record); err != nil {
		return err
	}
	return removeIfPresent(profile.paths.updating)
}

// completeInterruptedPreviousMove authenticates the predecessor before it
// completes the one remaining writer switch. A process may stop between the
// program and configuration renames, so the two verified parts are temporarily
// located at previous/current rather than at matching directory names.
func (profile *Profile) completeInterruptedPreviousMove(previousProgram, previousConfig string, record installationRecord) error {
	programPrevious, err := directoryPresent(previousProgram)
	if err != nil {
		return err
	}
	configPrevious, err := directoryPresent(previousConfig)
	if err != nil {
		return err
	}
	if programPrevious == configPrevious {
		return nil
	}
	predecessor := profile.paths
	if programPrevious {
		predecessor.programCurrent = previousProgram
	} else {
		predecessor.configCurrent = previousConfig
	}
	if err := verifyInstalled(predecessor, record); err != nil {
		return errors.New("interrupted Contributor update has no authenticated recovery generation")
	}
	if programPrevious {
		return os.Rename(profile.paths.configCurrent, previousConfig)
	}
	return os.Rename(profile.paths.programCurrent, previousProgram)
}

func removeIfPresent(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func pairedDirectories(first, second string) (bool, error) {
	firstPresent, err := directoryPresent(first)
	if err != nil {
		return false, err
	}
	secondPresent, err := directoryPresent(second)
	if err != nil {
		return false, err
	}
	if firstPresent != secondPresent {
		return false, errors.New("contributor update residue is incomplete")
	}
	return firstPresent, nil
}

func directoryPresent(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, errors.New("contributor update residue is not a directory")
	}
	return true, nil
}
