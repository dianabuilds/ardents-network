package contributor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func (profile *Profile) installAbsent(ctx context.Context, bundle verifiedBundle) (resultErr error) {
	if err := ensureAbsent(profile.paths.programRoot, profile.paths.privateRoot, profile.paths.unit, profile.paths.installing); err != nil {
		return err
	}
	deployed := true
	defer func() {
		if resultErr != nil && deployed {
			_, stopErr := profile.supervisor.Do(context.Background(), SupervisorStop)
			_, disableErr := profile.supervisor.Do(context.Background(), SupervisorDisable)
			removeErr := removeInstallation(profile.paths)
			markerErr := removeIfPresent(profile.paths.installing)
			_, reloadErr := profile.supervisor.Do(context.Background(), SupervisorReload)
			resultErr = errors.Join(resultErr, stopErr, disableErr, removeErr, markerErr, reloadErr)
		}
	}()
	if err := os.MkdirAll(filepath.Dir(profile.paths.installing), 0o755); err != nil {
		return err
	}
	if err := writeJSONAtomic(profile.paths.installing, markerFor(bundle), 0o600); err != nil {
		return err
	}
	if err := writeDeployment(profile.paths, bundle); err != nil {
		return err
	}
	if _, err := profile.supervisor.Do(ctx, SupervisorReload); err != nil {
		return err
	}
	if _, err := profile.supervisor.Do(ctx, SupervisorEnable); err != nil {
		return err
	}
	state, err := profile.supervisor.Do(ctx, SupervisorStart)
	if err != nil {
		return err
	}
	if !state.Active {
		return errors.New("contributor service did not become active")
	}
	if _, err := profile.awaitLifecycle(ctx, profile.paths.lifecycle, "READY", 15*time.Second); err != nil {
		return err
	}
	record := installationRecord{Schema: "ardents-contributor-installation-v1", Profile: profileName,
		DeploymentID: bundle.manifest.DeploymentID, Generation: bundle.manifest.Generation,
		ManifestDigest: bundle.manifestDigest, InstalledFiles: bundle.manifest.Files}
	unitDigest := sha256.Sum256([]byte(systemdUnit))
	record.SystemdUnitHash = hex.EncodeToString(unitDigest[:])
	if err := writeJSONAtomic(profile.paths.record, record, 0o600); err != nil {
		return err
	}
	deployed = false
	return removeIfPresent(profile.paths.installing)
}

func writeDeployment(paths hostPaths, bundle verifiedBundle) error {
	if err := writeBundleDirectories(bundle, paths.programCurrent, paths.configCurrent); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.diagnostics, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.unit), 0o755); err != nil {
		return err
	}
	return writeFileExclusive(paths.unit, []byte(systemdUnit), 0o644)
}

func writeBundleDirectories(bundle verifiedBundle, programDirectory, configDirectory string) error {
	if err := os.MkdirAll(programDirectory, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		return err
	}
	for _, spec := range bundleFileSpecs {
		destination := filepath.Join(configDirectory, spec.name)
		if spec.executable {
			destination = filepath.Join(programDirectory, spec.name)
		}
		if err := writeFileExclusive(destination, bundle.files[spec.name], spec.mode); err != nil {
			return err
		}
	}
	return nil
}
