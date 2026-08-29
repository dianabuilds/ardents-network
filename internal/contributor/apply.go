package contributor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type installationRecord struct {
	Schema          string            `json:"schema"`
	Profile         string            `json:"profile"`
	DeploymentID    string            `json:"deployment_id"`
	Generation      uint64            `json:"generation"`
	ManifestDigest  string            `json:"manifest_digest"`
	InstalledFiles  map[string]string `json:"installed_files"`
	SystemdUnitHash string            `json:"systemd_unit_sha256"`
}

type lifecycleEvent struct {
	Schema           string          `json:"schema"`
	Kind             string          `json:"kind"`
	State            string          `json:"state"`
	At               time.Time       `json:"at"`
	Epoch            uint64          `json:"epoch,omitempty"`
	Generation       string          `json:"generation,omitempty"`
	Assignment       string          `json:"assignment,omitempty"`
	CarrierProfile   string          `json:"carrier_profile,omitempty"`
	AssignmentDigest [32]byte        `json:"assignment_digest,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	Resource         json.RawMessage `json:"resource,omitempty"`
}

// Apply verifies one independently pinned bundle before parsing it. Generation
// one creates an absent profile; later generations use the update transaction.
func (profile *Profile) Apply(ctx context.Context, directory, manifestPin string) (Report, error) {
	if profile == nil || ctx == nil {
		return Report{}, errors.New("contributor profile is unavailable")
	}
	bundle, err := openBundle(directory, manifestPin)
	if err != nil {
		return Report{}, err
	}
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
		_ = os.RemoveAll(nextProgram)
		_ = os.RemoveAll(nextConfig)
		return err
	}
	switched := false
	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(nextProgram)
			_ = os.RemoveAll(nextConfig)
			if switched {
				rollbackErr := profile.rollbackUpdate(previousProgram, previousConfig)
				resultErr = errors.Join(resultErr, rollbackErr)
			}
		}
	}()
	if _, err := profile.supervisor.Do(ctx, SupervisorStop); err != nil {
		return err
	}
	if _, err := awaitLifecycle(ctx, profile.paths.lifecycle, "WITHDRAWN", 15*time.Second); err != nil {
		return err
	}
	if err := os.Rename(profile.paths.programCurrent, previousProgram); err != nil {
		return err
	}
	if err := os.Rename(profile.paths.configCurrent, previousConfig); err != nil {
		_ = os.Rename(previousProgram, profile.paths.programCurrent)
		return err
	}
	if err := os.Rename(nextProgram, profile.paths.programCurrent); err != nil {
		_ = os.Rename(previousConfig, profile.paths.configCurrent)
		_ = os.Rename(previousProgram, profile.paths.programCurrent)
		return err
	}
	if err := os.Rename(nextConfig, profile.paths.configCurrent); err != nil {
		_ = os.RemoveAll(profile.paths.programCurrent)
		_ = os.Rename(previousConfig, profile.paths.configCurrent)
		_ = os.Rename(previousProgram, profile.paths.programCurrent)
		return err
	}
	switched = true
	_ = os.Remove(profile.paths.lifecycle)
	state, err := profile.supervisor.Do(ctx, SupervisorStart)
	if err != nil || !state.Active {
		return errors.Join(err, errors.New("contributor successor did not become active"))
	}
	if _, err := awaitLifecycle(ctx, profile.paths.lifecycle, "READY", 15*time.Second); err != nil {
		return err
	}
	record := installationRecord{Schema: "ardents-contributor-installation-v1", Profile: profileName,
		DeploymentID: bundle.manifest.DeploymentID, Generation: bundle.manifest.Generation,
		ManifestDigest: bundle.manifestDigest, InstalledFiles: bundle.manifest.Files,
		SystemdUnitHash: current.SystemdUnitHash}
	if err := writeJSONAtomic(profile.paths.record, record, 0o600); err != nil {
		return err
	}
	switched = false
	return errors.Join(os.RemoveAll(previousProgram), os.RemoveAll(previousConfig))
}

func (profile *Profile) rollbackUpdate(previousProgram, previousConfig string) error {
	_, stopErr := profile.supervisor.Do(context.Background(), SupervisorStop)
	removeErr := errors.Join(os.RemoveAll(profile.paths.programCurrent), os.RemoveAll(profile.paths.configCurrent))
	renameErr := errors.Join(os.Rename(previousProgram, profile.paths.programCurrent), os.Rename(previousConfig, profile.paths.configCurrent))
	_ = os.Remove(profile.paths.lifecycle)
	state, startErr := profile.supervisor.Do(context.Background(), SupervisorStart)
	if startErr == nil && !state.Active {
		startErr = errors.New("rolled-back Contributor did not become active")
	}
	if startErr == nil {
		_, startErr = awaitLifecycle(context.Background(), profile.paths.lifecycle, "READY", 15*time.Second)
	}
	return errors.Join(stopErr, removeErr, renameErr, startErr)
}

func (profile *Profile) installAbsent(ctx context.Context, bundle verifiedBundle) (resultErr error) {
	if err := ensureAbsent(profile.paths.programRoot, profile.paths.privateRoot, profile.paths.unit, profile.paths.installing); err != nil {
		return err
	}
	deployed := true
	defer func() {
		if resultErr != nil && deployed {
			_, _ = profile.supervisor.Do(context.Background(), SupervisorStop)
			_, _ = profile.supervisor.Do(context.Background(), SupervisorDisable)
			_ = removeInstallation(profile.paths)
			_ = removeIfPresent(profile.paths.installing)
			_, _ = profile.supervisor.Do(context.Background(), SupervisorReload)
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
	if _, err := awaitLifecycle(ctx, profile.paths.lifecycle, "READY", 15*time.Second); err != nil {
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

func (profile *Profile) report(ctx context.Context) (Report, error) {
	record, err := readInstallation(profile.paths.record)
	if err != nil {
		return Report{}, err
	}
	if err := verifyInstalled(profile.paths, record); err != nil {
		return Report{}, err
	}
	lifecycle, err := readLifecycle(profile.paths.lifecycle)
	if err != nil {
		return Report{}, err
	}
	state, err := profile.supervisor.Do(ctx, SupervisorStatus)
	if err != nil {
		return Report{}, err
	}
	return Report{Profile: record.Profile, DeploymentID: record.DeploymentID, Generation: record.Generation,
		ManifestDigest: record.ManifestDigest, ProgramDigest: record.InstalledFiles["ardents-node"],
		LifecycleState: lifecycle.State, Active: state.Active, Enabled: state.Enabled}, nil
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
	for _, name := range bundleFiles {
		destination := filepath.Join(configDirectory, name)
		mode := os.FileMode(0o600)
		if name == "ardents-node" {
			destination, mode = filepath.Join(programDirectory, name), 0o755
		}
		if err := writeFileExclusive(destination, bundle.files[name], mode); err != nil {
			return err
		}
	}
	return nil
}

func ensureAbsent(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			return errors.New("contributor host contains conflicting installation state")
		}
	}
	return nil
}

func writeFileExclusive(path string, raw []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(raw)
	if writeErr == nil && written != len(raw) {
		writeErr = errors.New("short Contributor file write")
	}
	return errors.Join(writeErr, file.Sync(), file.Close())
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".new"
	if err := os.Remove(temporary); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := writeFileExclusive(temporary, append(raw, '\n'), mode); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readInstallation(path string) (installationRecord, error) {
	raw, err := readRegular(path, 64<<10)
	if err != nil {
		return installationRecord{}, err
	}
	var record installationRecord
	if err := decodeStrict(raw, &record); err != nil || record.Schema != "ardents-contributor-installation-v1" ||
		record.Profile != profileName || !fixedHex(record.DeploymentID, 32) || record.Generation == 0 ||
		!fixedHex(record.ManifestDigest, 32) || len(record.InstalledFiles) != len(bundleFiles) || !fixedHex(record.SystemdUnitHash, 32) {
		return installationRecord{}, errors.New("contributor installation record is invalid")
	}
	return record, nil
}

func verifyInstalled(paths hostPaths, record installationRecord) error {
	for _, name := range bundleFiles {
		path := filepath.Join(paths.configCurrent, name)
		if name == "ardents-node" {
			path = filepath.Join(paths.programCurrent, name)
		}
		raw, err := readRegular(path, 128<<20)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != record.InstalledFiles[name] {
			return fmt.Errorf("installed Contributor file %s differs from its authenticated bundle", name)
		}
	}
	unit, err := readRegular(paths.unit, 64<<10)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(unit)
	if hex.EncodeToString(digest[:]) != record.SystemdUnitHash {
		return errors.New("installed Contributor systemd unit differs from its profile")
	}
	return nil
}

func awaitLifecycle(ctx context.Context, path, want string, timeout time.Duration) (lifecycleEvent, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if event, err := readLifecycle(path); err == nil && event.State == want {
			return event, nil
		}
		select {
		case <-ctx.Done():
			return lifecycleEvent{}, ctx.Err()
		case <-deadline.C:
			return lifecycleEvent{}, errors.New("contributor lifecycle did not reach " + want)
		case <-ticker.C:
		}
	}
}

func readLifecycle(path string) (lifecycleEvent, error) {
	raw, err := readRegular(path, 4096)
	if err != nil {
		return lifecycleEvent{}, err
	}
	var event lifecycleEvent
	if err := decodeStrict(raw, &event); err != nil || event.Schema != "ardents-node-event-v1" || event.Kind != "lifecycle" || event.State == "" {
		return lifecycleEvent{}, errors.New("contributor lifecycle diagnostic is invalid")
	}
	return event, nil
}

func removeInstallation(paths hostPaths) error {
	return errors.Join(os.RemoveAll(paths.programRoot), os.RemoveAll(paths.privateRoot), removeIfPresent(paths.unit))
}
