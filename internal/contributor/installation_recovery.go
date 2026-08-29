package contributor

import (
	"context"
	"errors"
	"os"
)

type installationMarker struct {
	Schema         string `json:"schema"`
	DeploymentID   string `json:"deployment_id"`
	Generation     uint64 `json:"generation"`
	ManifestDigest string `json:"manifest_digest"`
}

func markerFor(bundle verifiedBundle) installationMarker {
	return installationMarker{Schema: "ardents-contributor-installing-v1", DeploymentID: bundle.manifest.DeploymentID,
		Generation: bundle.manifest.Generation, ManifestDigest: bundle.manifestDigest}
}

func (profile *Profile) recoverUncommittedInstallation(ctx context.Context, bundle verifiedBundle) error {
	marker, err := readInstallationMarker(profile.paths.installing)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if marker != markerFor(bundle) {
		return errors.New("unfinished Contributor installation belongs to another authenticated bundle")
	}
	_, stopErr := profile.supervisor.Do(ctx, SupervisorStop)
	_, disableErr := profile.supervisor.Do(ctx, SupervisorDisable)
	removeErr := removeInstallation(profile.paths)
	markerErr := removeIfPresent(profile.paths.installing)
	_, reloadErr := profile.supervisor.Do(ctx, SupervisorReload)
	return errors.Join(stopErr, disableErr, removeErr, markerErr, reloadErr)
}

func (profile *Profile) clearCommittedInstallationMarker(record installationRecord) error {
	marker, err := readInstallationMarker(profile.paths.installing)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if marker.DeploymentID != record.DeploymentID || marker.Generation != record.Generation || marker.ManifestDigest != record.ManifestDigest {
		return errors.New("contributor installation marker conflicts with the committed generation")
	}
	return removeIfPresent(profile.paths.installing)
}

func readInstallationMarker(path string) (installationMarker, error) {
	raw, err := readRegular(path, 4096)
	if err != nil {
		if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
			return installationMarker{}, os.ErrNotExist
		}
		return installationMarker{}, err
	}
	var marker installationMarker
	if err := decodeStrict(raw, &marker); err != nil || marker.Schema != "ardents-contributor-installing-v1" ||
		!fixedHex(marker.DeploymentID, 32) || marker.Generation == 0 || !fixedHex(marker.ManifestDigest, 32) {
		return installationMarker{}, errors.New("contributor installation marker is invalid")
	}
	return marker, nil
}
