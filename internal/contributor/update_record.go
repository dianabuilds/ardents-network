package contributor

import (
	"errors"
	"os"
)

type updateRecord struct {
	Schema   string             `json:"schema"`
	Previous installationRecord `json:"previous"`
}

func updateRecordFor(previous installationRecord) updateRecord {
	return updateRecord{Schema: "ardents-contributor-updating-v1", Previous: previous}
}

func readUpdateRecord(path string) (updateRecord, error) {
	raw, err := readRegular(path, 64<<10)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
			return updateRecord{}, os.ErrNotExist
		}
		return updateRecord{}, err
	}
	var record updateRecord
	if err := decodeStrict(raw, &record); err != nil || record.Schema != "ardents-contributor-updating-v1" || !validInstallationRecord(&record.Previous) {
		return updateRecord{}, errors.New("contributor update record is invalid")
	}
	return record, nil
}

func sameInstallationRecord(first, second installationRecord) bool {
	if first.Schema != second.Schema || first.Profile != second.Profile || first.DeploymentID != second.DeploymentID ||
		first.Generation != second.Generation || first.ManifestDigest != second.ManifestDigest || first.SystemdUnitHash != second.SystemdUnitHash ||
		len(first.InstalledFiles) != len(second.InstalledFiles) {
		return false
	}
	for name, digest := range first.InstalledFiles {
		if second.InstalledFiles[name] != digest {
			return false
		}
	}
	return true
}
