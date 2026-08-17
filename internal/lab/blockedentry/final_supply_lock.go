package blockedentry

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const finalSupplyLockSourcePath = "tests/live/stage5-final/supply.lock.json"
const finalSupplyLockPath = "runtime/supply.lock.json"
const finalGoArchiveHash = "708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89"

type finalSupplyLock struct {
	Schema           string `json:"schema"`
	GoBuilderImageID string `json:"go_builder_image_id"`
	GoBuilderVersion string `json:"go_builder_version"`
	GoArchiveSHA256  string `json:"go_archive_sha256"`
	GoRecipeSHA256   string `json:"go_builder_recipe_sha256"`
	GoModuleSHA256   string `json:"go_module_cache_sha256"`
	ToolImageID      string `json:"tool_image_id"`
	ToolLockSHA256   string `json:"tool_lock_sha256"`
	CarrierLabSHA256 string `json:"carrier_sha256"`
}

func loadFinalSupplyLock(sourceRoot string) (finalSupplyLock, error) {
	path := filepath.Join(sourceRoot, filepath.FromSlash(finalSupplyLockSourcePath))
	raw, err := readStableSupplyLock(path)
	if err != nil {
		return finalSupplyLock{}, errors.Join(err,
			errors.New("final supply lock is unavailable or oversized"))
	}
	var value finalSupplyLock
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return finalSupplyLock{}, errors.New("final supply lock is not one strict value")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return finalSupplyLock{}, errors.New("final supply lock is not canonical JSON")
	}
	toolLock, _, lockErr := hashFile(filepath.Join(sourceRoot, "lab", "carrier", "tools.lock"))
	recipe, _, recipeErr := hashFile(filepath.Join(sourceRoot, "tests", "live", "stage5-final", "go-builder.Dockerfile"))
	if value.Schema != "ardents-h3-s5-supply-lock-v1" || !imageID(value.GoBuilderImageID) ||
		!imageID(value.ToolImageID) || value.GoBuilderImageID == value.ToolImageID ||
		value.GoBuilderVersion != finalGoBuilderVersion || value.GoArchiveSHA256 != finalGoArchiveHash ||
		value.GoRecipeSHA256 != recipe || !hexDigest(value.CarrierLabSHA256, 32) ||
		!hexDigest(value.GoModuleSHA256, 32) ||
		lockErr != nil || recipeErr != nil || value.ToolLockSHA256 != toolLock {
		return finalSupplyLock{}, errors.Join(lockErr, recipeErr,
			errors.New("final supply lock has not been accepted for the qualifying stand"))
	}
	return value, nil
}

func readStableSupplyLock(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() > 4<<10 {
		return nil, errors.Join(err, errors.New("supply lock is not a bounded regular file"))
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(input, (4<<10)+1))
	after, statErr := input.Stat()
	closeErr := input.Close()
	if readErr != nil || statErr != nil || closeErr != nil || !os.SameFile(before, after) || len(raw) == 0 || len(raw) > 4<<10 {
		return nil, errors.Join(readErr, statErr, closeErr, errors.New("supply lock changed or exceeded its bound"))
	}
	return raw, nil
}

func freezeFinalSupplyLock(sourceRoot, outputRoot string) (artifactCommitment, error) {
	source := filepath.Join(sourceRoot, filepath.FromSlash(finalSupplyLockSourcePath))
	target := filepath.Join(outputRoot, filepath.FromSlash(finalSupplyLockPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return artifactCommitment{}, err
	}
	if err := copyStableArtifact(source, target, 0o400); err != nil {
		return artifactCommitment{}, err
	}
	return commitment(outputRoot, finalSupplyLockPath)
}

func freezeFinalSupplyLockInput(secretRoot, sourceRoot string, value artifactCommitment) error {
	if value.Path != finalSupplyLockPath || !hexDigest(value.SHA256, 32) || value.Bytes < 1 {
		return errors.New("final supply lock commitment is invalid")
	}
	source := filepath.Join(sourceRoot, filepath.FromSlash(finalSupplyLockPath))
	hash, size, err := hashFile(source)
	if err != nil || hash != value.SHA256 || size != value.Bytes {
		return errors.Join(err, errors.New("final supply lock differs from its commitment"))
	}
	target := filepath.Join(secretRoot, filepath.FromSlash(finalSupplyLockPath))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return copyStableArtifact(source, target, 0o400)
}

func verifyFinalSupplyLockWorkspace(workspace string, spec finalSpec) (returnErr error) {
	commit, sourceHash, sourceRoot, temporary, err := materializeCommittedSource(workspace)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, os.RemoveAll(temporary)) }()
	if commit != spec.RepositoryCommit || sourceHash != spec.SourceSHA256 {
		return errors.New("final campaign repository source differs from its specification")
	}
	value, err := loadFinalSupplyLock(sourceRoot)
	if err != nil {
		return err
	}
	sourceCommitment, err := commitment(sourceRoot, finalSupplyLockSourcePath)
	if err != nil || sourceCommitment.SHA256 != spec.SupplyLock.SHA256 ||
		sourceCommitment.Bytes != spec.SupplyLock.Bytes {
		return errors.Join(err, errors.New("repository supply lock differs from its runtime commitment"))
	}
	if !finalSupplyLockMatchesSpec(value, spec) {
		return errors.New("final campaign supply identities differ from the accepted repository lock")
	}
	return nil
}

func finalSupplyLockMatchesSpec(value finalSupplyLock, spec finalSpec) bool {
	return value.GoBuilderImageID == spec.GoBuilderImageID && value.GoBuilderVersion == spec.GoBuilderVersion &&
		value.GoArchiveSHA256 == spec.ProductReceipt.GoArchiveSHA256 &&
		value.GoRecipeSHA256 == spec.ProductReceipt.GoRecipeSHA256 &&
		value.GoModuleSHA256 == spec.ProductReceipt.GoModuleSHA256 &&
		value.ToolImageID == spec.ToolImageID && value.ToolLockSHA256 == spec.ToolReceipt.ToolLockSHA256 &&
		value.CarrierLabSHA256 == spec.ToolReceipt.CarrierSHA256
}
