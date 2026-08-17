package blockedentry

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

var finalConfigurationPaths = []string{
	"configuration/topology.json",
	"configuration/cgroups.json",
	"configuration/network.json",
	"configuration/workloads.json",
	"configuration/invites.sha256",
	"configuration/route-credentials.sha256",
	"configuration/observers.json",
}

func freezeCampaignSpec(config Config, secretRoot string) (Config, *finalSpec, error) {
	if config.CampaignSpecPath == "" {
		return config, nil, nil
	}
	if config.Mode != "final-campaign" {
		return Config{}, nil, errors.New("final campaign specification requires final-campaign mode")
	}
	target := filepath.Join(secretRoot, "final-spec.json")
	if err := copyStableArtifact(config.CampaignSpecPath, target, 0o400); err != nil {
		return Config{}, nil, err
	}
	var value finalSpec
	if err := decodeCanonical(target, &value); err != nil {
		return Config{}, nil, err
	}
	if value.Schema != "ardents-h3-s5-final-spec-v1" || len(value.CellOrder) == 0 ||
		len(value.Seeds) != len(value.CellOrder) || len(value.Configurations) != len(finalConfigurationPaths) {
		return Config{}, nil, errors.New("final campaign specification is incomplete")
	}
	if !imageID(value.ProductImageID) || !imageID(value.ToolImageID) || !imageID(value.GoBuilderImageID) ||
		value.ProductImageID == value.ToolImageID || value.ProductImageID == value.GoBuilderImageID ||
		value.ToolImageID == value.GoBuilderImageID || value.GoBuilderVersion != finalGoBuilderVersion {
		return Config{}, nil, errors.New("final campaign image supply is invalid")
	}
	if value.SupplyLock.Path != finalSupplyLockPath || !hexDigest(value.SupplyLock.SHA256, 32) ||
		value.SupplyLock.Bytes < 1 {
		return Config{}, nil, errors.New("final campaign supply lock commitment is invalid")
	}
	if !validProductReceipt(value.ProductReceipt, value.SourceSHA256) || !validToolReceipt(value.ToolReceipt) {
		return Config{}, nil, errors.New("final campaign image receipts are invalid")
	}
	if err := verifyFinalSupplyLockWorkspace(config.WorkspaceRoot, value); err != nil {
		return Config{}, nil, err
	}
	root := filepath.Dir(config.CampaignSpecPath)
	config, err := freezeRuntimeComposeInput(config, secretRoot, root, value.RuntimeCompose)
	if err != nil {
		return Config{}, nil, err
	}
	if err := freezeFinalSupplyLockInput(secretRoot, root, value.SupplyLock); err != nil {
		return Config{}, nil, err
	}
	for index, expected := range finalConfigurationPaths {
		commitment := value.Configurations[index]
		if commitment.Path != expected || commitment.Bytes < 1 {
			return Config{}, nil, errors.New("final campaign configuration inventory differs from its schema")
		}
		source := filepath.Join(root, filepath.FromSlash(expected))
		hash, size, err := hashFile(source)
		if err != nil || hash != commitment.SHA256 || size != commitment.Bytes {
			return Config{}, nil, errors.Join(err, errors.New("final campaign configuration commitment mismatch"))
		}
		frozen := filepath.Join(secretRoot, filepath.FromSlash(expected))
		if err := os.MkdirAll(filepath.Dir(frozen), 0o700); err != nil {
			return Config{}, nil, err
		}
		if err := copyStableArtifact(source, frozen, 0o400); err != nil {
			return Config{}, nil, err
		}
	}
	config.CampaignSpecPath = target
	config.ProductImageID, config.ToolImageID = value.ProductImageID, value.ToolImageID
	config.GoBuilderImageID = value.GoBuilderImageID
	return config, &value, nil
}

func decodeCanonical(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > maximumEvidenceFile {
		return errors.Join(err, errors.New("final campaign specification is unavailable or oversized"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("final campaign specification is not one strict schema value")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("final campaign specification is not canonical JSON")
	}
	return nil
}

func copyStableArtifact(source, target string, mode os.FileMode) error {
	pathInfo, err := os.Lstat(source)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 {
		return errors.Join(err, errors.New("campaign artifact source is invalid"))
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) ||
		info.Size() < 1 || info.Size() > maximumEvidenceFile {
		return errors.Join(err, errors.New("campaign artifact is not a bounded stable file"))
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, maximumEvidenceFile+1))
	syncErr, closeErr := output.Sync(), output.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil || written != info.Size() {
		return errors.Join(copyErr, syncErr, closeErr, errors.New("campaign artifact copy is incomplete"))
	}
	return os.Chmod(target, mode)
}
