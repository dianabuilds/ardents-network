package contributor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type installedFileSpec struct {
	name       string
	maximum    int64
	executable bool
}

// Historical installed inventory remains the verification contract for owned installations.
var installedFileSpecs = []installedFileSpec{
	{name: "ardents-node", maximum: 128 << 20, executable: true},
	{name: "node.json", maximum: 64 << 10},
	{name: "rendezvous-cert.pem", maximum: 64 << 10},
	{name: "rendezvous-key.pem", maximum: 64 << 10},
	{name: "rendezvous-identity.pem", maximum: 64 << 10},
	{name: "source-client-cert.pem", maximum: 64 << 10},
	{name: "source-client-key.pem", maximum: 64 << 10},
	{name: "source-a-root.pem", maximum: 64 << 10},
	{name: "source-b-root.pem", maximum: 64 << 10},
	{name: "clock.observation", maximum: 64 << 10},
}

type installationRecord struct {
	Schema          string            `json:"schema"`
	Profile         string            `json:"profile"`
	DeploymentID    string            `json:"deployment_id"`
	Generation      uint64            `json:"generation"`
	ManifestDigest  string            `json:"manifest_digest"`
	InstalledFiles  map[string]string `json:"installed_files"`
	SystemdUnitHash string            `json:"systemd_unit_sha256"`
}

func readInstallation(path string) (installationRecord, error) {
	raw, err := readRegular(path, 64<<10)
	if err != nil {
		return installationRecord{}, err
	}
	var record installationRecord
	if err := decodeStrict(raw, &record); err != nil {
		return installationRecord{}, errors.New("contributor installation record is invalid")
	}
	if !validInstallationRecord(&record) {
		return installationRecord{}, errors.New("contributor installation record is invalid")
	}
	return record, nil
}

func validInstallationRecord(record *installationRecord) bool {
	normalizedProfile, knownProfile := normalizeRendezvousDedicatedHostProfile(record.Profile)
	if record.Schema != "ardents-contributor-installation-v1" || !knownProfile || !fixedHex(record.DeploymentID, 32) || record.Generation == 0 ||
		!fixedHex(record.ManifestDigest, 32) || len(record.InstalledFiles) != len(installedFileSpecs) || !fixedHex(record.SystemdUnitHash, 32) {
		return false
	}
	record.Profile = normalizedProfile
	return true
}

func verifyInstalled(paths hostPaths, record installationRecord) error {
	for _, spec := range installedFileSpecs {
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

func verifyManagementExecutable(paths hostPaths, record installationRecord) error {
	raw, err := readRegular(paths.programManagement, 128<<20)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != record.InstalledFiles["ardents-node"] {
		return errors.New("installed Contributor management executable differs from its authenticated bundle")
	}
	return nil
}

func removeInstallation(paths hostPaths) error {
	return errors.Join(os.RemoveAll(paths.programRoot), os.RemoveAll(paths.privateRoot), removeIfPresent(paths.unit))
}
