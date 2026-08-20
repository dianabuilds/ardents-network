package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

func (raw *offlineImportFlags) buildInputs() (releasedecision.Inputs, error) {
	if raw.stateRoot == "" {
		return releasedecision.Inputs{}, errors.New("state-root is required")
	}
	if raw.metadataDir == "" {
		return releasedecision.Inputs{}, errors.New("metadata-dir is required")
	}
	if raw.rootPath == "" {
		return releasedecision.Inputs{}, errors.New("root path is required")
	}
	if raw.targetPath == "" {
		return releasedecision.Inputs{}, errors.New("target path is required")
	}
	if raw.artifactPath == "" {
		return releasedecision.Inputs{}, errors.New("artifact path is required")
	}
	if raw.refTime == "" {
		return releasedecision.Inputs{}, errors.New("ref-time is required")
	}
	for name, value := range map[string]string{
		"environment": raw.environment, "network": raw.network,
		"platform": raw.platform, "architecture": raw.architecture,
	} {
		if value == "" {
			return releasedecision.Inputs{}, fmt.Errorf("%s is required", name)
		}
	}
	rootBytes, err := readBoundedRegularFile(raw.rootPath, maximumMetadataFileBytes)
	if err != nil {
		return releasedecision.Inputs{}, fmt.Errorf("read root: %w", err)
	}
	files, err := readMetadataDir(raw.metadataDir)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	artifact, err := readBoundedRegularFile(raw.artifactPath, maximumArtifactBytes)
	if err != nil {
		return releasedecision.Inputs{}, fmt.Errorf("read artifact: %w", err)
	}
	refTime, err := time.Parse(time.RFC3339, raw.refTime)
	if err != nil {
		return releasedecision.Inputs{}, fmt.Errorf("parse time %q: %w", raw.refTime, err)
	}
	refTime = refTime.UTC()
	return releasedecision.Inputs{
		RootBytes:  rootBytes,
		Files:      files,
		TargetPath: raw.targetPath,
		Artifact:   artifact,
		Local: releasedecision.LocalEnvironment{
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
