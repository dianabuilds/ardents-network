package contributor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Apply verifies one independently pinned bundle before parsing it. Generation
// one creates an absent profile; later generations use the update transaction.
func (profile *Profile) Apply(ctx context.Context, directory, manifestPin string) (report Report, resultErr error) {
	if profile == nil || ctx == nil {
		return Report{}, errors.New("contributor profile is unavailable")
	}
	bundle, err := openBundle(directory, manifestPin)
	if err != nil {
		return Report{}, err
	}
	lease, err := profile.acquireRootLease()
	if err != nil {
		return Report{}, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, lease.release())
	}()
	if _, statErr := os.Stat(profile.paths.record); statErr == nil {
		current, readErr := readInstallation(profile.paths.record)
		if readErr != nil {
			return Report{}, readErr
		}
		if err := profile.clearCommittedInstallationMarker(current); err != nil {
			return Report{}, err
		}
		if err := profile.recoverInterruptedUpdate(ctx, current); err != nil {
			return Report{}, err
		}
		if bundle.manifest.DeploymentID != current.DeploymentID || bundle.manifest.Generation != current.Generation+1 {
			return Report{}, errors.New("contributor successor must continue the same deployment by one generation")
		}
		if err := verifyInstalled(profile.paths, current); err != nil {
			return Report{}, err
		}
		if err := profile.update(ctx, bundle, current); err != nil {
			return Report{}, err
		}
		return profile.report(ctx)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Report{}, statErr
	} else if bundle.manifest.Generation != 1 {
		return Report{}, errors.New("first Contributor installation requires generation one")
	}
	if err := profile.recoverUncommittedInstallation(ctx, bundle); err != nil {
		return Report{}, err
	}
	if err := profile.installAbsent(ctx, bundle); err != nil {
		return Report{}, err
	}
	return profile.report(ctx)
}

func (profile *Profile) update(ctx context.Context, bundle verifiedBundle, current installationRecord) (resultErr error) {
	nextProgram := filepath.Join(profile.paths.programRoot, "next")
	nextConfig := filepath.Join(profile.paths.configRoot, "next")
	previousProgram := filepath.Join(profile.paths.programRoot, "previous")
	previousConfig := filepath.Join(profile.paths.configRoot, "previous")
	if err := ensureAbsent(nextProgram, nextConfig, previousProgram, previousConfig); err != nil {
		return err
	}
	if err := writeBundleDirectories(bundle, nextProgram, nextConfig); err != nil {
		return errors.Join(err, os.RemoveAll(nextProgram), os.RemoveAll(nextConfig))
	}
	if err := writeJSONAtomic(profile.paths.updating, updateRecordFor(current), 0o600); err != nil {
		return errors.Join(err, os.RemoveAll(nextProgram), os.RemoveAll(nextConfig))
	}
	if err := profile.replaceManagementExecutable(bundle.files["ardents-node"]); err != nil {
		cleanupErr := errors.Join(os.RemoveAll(nextProgram), os.RemoveAll(nextConfig))
		restoreErr := profile.restoreManagementExecutableFromCurrent()
		result := errors.Join(err, cleanupErr, restoreErr)
		if cleanupErr == nil && restoreErr == nil {
			return errors.Join(result, removeIfPresent(profile.paths.updating))
		}
		return result
	}
	switched, stopAttempted, committed := false, false, false
	defer func() {
		if resultErr != nil {
			cleanupErr := errors.Join(os.RemoveAll(nextProgram), os.RemoveAll(nextConfig))
			if committed {
				cleanupErr = errors.Join(cleanupErr, os.RemoveAll(previousProgram), os.RemoveAll(previousConfig))
			}
			resultErr = errors.Join(resultErr, cleanupErr)
			recovered := cleanupErr == nil
			if !committed {
				if switched {
					err := profile.rollbackUpdate(previousProgram, previousConfig)
					recovered = err == nil
					resultErr = errors.Join(resultErr, err)
				} else if stopAttempted {
					err := profile.restartCurrentGeneration()
					recovered = err == nil
					resultErr = errors.Join(resultErr, err)
				}
				managerRestoreErr := profile.restoreManagementExecutableFromCurrent()
				recovered = recovered && managerRestoreErr == nil
				resultErr = errors.Join(resultErr, managerRestoreErr)
			}
			if committed || recovered {
				resultErr = errors.Join(resultErr, removeIfPresent(profile.paths.updating))
			}
		}
	}()
	stopAttempted = true
	if _, err := profile.supervisor.Do(ctx, SupervisorStop); err != nil {
		return err
	}
	if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "WITHDRAWN", 15*time.Second); err != nil {
		return err
	}
	if err := os.Rename(profile.paths.programCurrent, previousProgram); err != nil {
		return err
	}
	if err := os.Rename(profile.paths.configCurrent, previousConfig); err != nil {
		return errors.Join(err, os.Rename(previousProgram, profile.paths.programCurrent))
	}
	if err := os.Rename(nextProgram, profile.paths.programCurrent); err != nil {
		return errors.Join(err,
			os.Rename(previousConfig, profile.paths.configCurrent),
			os.Rename(previousProgram, profile.paths.programCurrent))
	}
	if err := os.Rename(nextConfig, profile.paths.configCurrent); err != nil {
		return errors.Join(err,
			os.RemoveAll(profile.paths.programCurrent),
			os.Rename(previousConfig, profile.paths.configCurrent),
			os.Rename(previousProgram, profile.paths.programCurrent))
	}
	switched = true
	if err := removeIfPresent(profile.paths.lifecycle); err != nil {
		return err
	}
	state, err := profile.supervisor.Do(ctx, SupervisorStart)
	if err != nil || !state.Active {
		return errors.Join(err, errors.New("contributor successor did not become active"))
	}
	if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "READY", 15*time.Second); err != nil {
		return err
	}
	record := installationRecord{Schema: "ardents-contributor-installation-v1", Profile: rendezvousDedicatedHostProfile,
		DeploymentID: bundle.manifest.DeploymentID, Generation: bundle.manifest.Generation,
		ManifestDigest: bundle.manifestDigest, InstalledFiles: bundle.manifest.Files,
		SystemdUnitHash: current.SystemdUnitHash}
	if err := writeJSONAtomic(profile.paths.record, record, 0o600); err != nil {
		return err
	}
	committed = true
	switched = false
	if err := errors.Join(os.RemoveAll(previousProgram), os.RemoveAll(previousConfig)); err != nil {
		return err
	}
	return removeIfPresent(profile.paths.updating)
}

func (profile *Profile) replaceManagementExecutable(raw []byte) error {
	return writeFileAtomic(profile.paths.programManagement, raw, 0o755)
}

func (profile *Profile) restoreManagementExecutableFromCurrent() error {
	raw, err := readRegular(filepath.Join(profile.paths.programCurrent, "ardents-node"), 128<<20)
	if err != nil {
		return err
	}
	return profile.replaceManagementExecutable(raw)
}

func (profile *Profile) restartCurrentGeneration() error {
	if err := removeIfPresent(profile.paths.lifecycle); err != nil {
		return err
	}
	state, err := profile.supervisor.Do(context.Background(), SupervisorRestart)
	if err != nil || !state.Active {
		return errors.Join(err, errors.New("previous Contributor generation did not restart"))
	}
	_, err = profile.awaitLifecycle(context.Background(), profile.paths.lifecycle, "READY", 15*time.Second)
	return err
}

func (profile *Profile) rollbackUpdate(previousProgram, previousConfig string) error {
	_, stopErr := profile.supervisor.Do(context.Background(), SupervisorStop)
	removeErr := errors.Join(os.RemoveAll(profile.paths.programCurrent), os.RemoveAll(profile.paths.configCurrent))
	renameErr := errors.Join(os.Rename(previousProgram, profile.paths.programCurrent), os.Rename(previousConfig, profile.paths.configCurrent))
	markerErr := removeIfPresent(profile.paths.lifecycle)
	state, startErr := profile.supervisor.Do(context.Background(), SupervisorStart)
	if startErr == nil && !state.Active {
		startErr = errors.New("rolled-back Contributor did not become active")
	}
	if startErr == nil {
		_, startErr = profile.awaitLifecycle(context.Background(), profile.paths.lifecycle, "READY", 15*time.Second)
	}
	return errors.Join(stopErr, removeErr, renameErr, markerErr, startErr)
}
