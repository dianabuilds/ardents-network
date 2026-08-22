package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func (raw *offlineImportFlags) buildInputs() (release.Inputs, error) {
	if raw.stateRoot == "" {
		return release.Inputs{}, errors.New("state-root is required")
	}
	if raw.metadataDir == "" {
		return release.Inputs{}, errors.New("metadata-dir is required")
	}
	if raw.rootPath == "" {
		return release.Inputs{}, errors.New("root path is required")
	}
	if raw.targetPath == "" {
		return release.Inputs{}, errors.New("target path is required")
	}
	if raw.artifactPath == "" {
		return release.Inputs{}, errors.New("artifact path is required")
	}
	if raw.refTime == "" {
		return release.Inputs{}, errors.New("ref-time is required")
	}
	for name, value := range map[string]string{
		"environment": raw.environment, "network": raw.network,
		"platform": raw.platform, "architecture": raw.architecture,
	} {
		if value == "" {
			return release.Inputs{}, fmt.Errorf("%s is required", name)
		}
	}
	rootBytes, err := readBoundedRegularFile(raw.rootPath, maximumMetadataFileBytes)
	if err != nil {
		return release.Inputs{}, fmt.Errorf("read root: %w", err)
	}
	files, err := readMetadataDir(raw.metadataDir)
	if err != nil {
		return release.Inputs{}, err
	}
	artifact, err := readBoundedRegularFile(raw.artifactPath, maximumArtifactBytes)
	if err != nil {
		return release.Inputs{}, fmt.Errorf("read artifact: %w", err)
	}
	refTime, err := time.Parse(time.RFC3339, raw.refTime)
	if err != nil {
		return release.Inputs{}, fmt.Errorf("parse time %q: %w", raw.refTime, err)
	}
	refTime = refTime.UTC()
	return release.Inputs{
		RootBytes:  rootBytes,
		Files:      files,
		TargetPath: raw.targetPath,
		Artifact:   artifact,
		Local: release.LocalEnvironment{
			Environment:  raw.environment,
			Network:      raw.network,
			Platform:     raw.platform,
			Architecture: raw.architecture,
			RefTime:      refTime,
		},
	}, nil
}

const maximumMetadataFileBytes int64 = 1 << 20
const maximumArtifactBytes int64 = 64 << 20

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("input is not a regular file")
	}
	if before.Size() > limit {
		return nil, fmt.Errorf("input exceeds %d-byte bound", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, statErr := file.Stat()
	if statErr != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		closeErr := file.Close()
		return nil, errors.Join(errors.New("input changed while it was opened"), statErr, closeErr)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("input exceeds %d-byte bound", limit)
	}
	return data, nil
}
