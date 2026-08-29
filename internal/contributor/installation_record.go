package contributor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		!fixedHex(record.ManifestDigest, 32) || len(record.InstalledFiles) != len(bundleFileSpecs) || !fixedHex(record.SystemdUnitHash, 32) {
		return installationRecord{}, errors.New("contributor installation record is invalid")
	}
	return record, nil
}

func verifyInstalled(paths hostPaths, record installationRecord) error {
	for _, spec := range bundleFileSpecs {
		path := filepath.Join(paths.configCurrent, spec.name)
		if spec.executable {
			path = filepath.Join(paths.programCurrent, spec.name)
		}
		raw, err := readRegular(path, spec.maximum)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != record.InstalledFiles[spec.name] {
			return fmt.Errorf("installed Contributor file %s differs from its authenticated bundle", spec.name)
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

func removeInstallation(paths hostPaths) error {
	return errors.Join(os.RemoveAll(paths.programRoot), os.RemoveAll(paths.privateRoot), removeIfPresent(paths.unit))
}
