package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

// buildInputs reads the supplied flag values and returns the
// releasedecision.Inputs the offline-import call must authenticate.
// The function performs only local I/O and small validation; every
// trust decision is delegated to the releasedecision package.
func (raw *offlineImportFlags) buildInputs() (releasedecision.Inputs, error) {
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
	rootBytes, err := os.ReadFile(raw.rootPath)
	if err != nil {
		return releasedecision.Inputs{}, fmt.Errorf("read root: %w", err)
	}
	files, err := readMetadataDir(raw.metadataDir)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	artifact, err := os.ReadFile(raw.artifactPath)
	if err != nil {
		return releasedecision.Inputs{}, fmt.Errorf("read artifact: %w", err)
	}
	refTime, err := parseTime(raw.refTime)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	if refTime.IsZero() {
		return releasedecision.Inputs{}, errors.New("ref-time is required")
	}
	overlappedSince, err := parseTime(raw.protocolOverlappedSince)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	emergencyExpiry, err := parseTime(raw.emergencyExpiry)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	buildSafetyNoNew, err := parseTime(raw.buildSafetyNoNewAfter)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	buildSafetyTerm, err := parseTime(raw.buildSafetyTermAfter)
	if err != nil {
		return releasedecision.Inputs{}, err
	}
	return releasedecision.Inputs{
		RootBytes:  rootBytes,
		Files:      files,
		TargetPath: raw.targetPath,
		Artifact:   artifact,
		Local: releasedecision.LocalEnvironment{
			Environment:               raw.environment,
			Network:                   raw.network,
			Platform:                  raw.platform,
			Architecture:              raw.architecture,
			RefTime:                   refTime,
			ProtocolOverlappedSince:   overlappedSince,
			CapacityReady:             raw.capacityReady,
			DrainReady:                raw.drainReady,
			EmergencyExpiry:           emergencyExpiry,
			EmergencyReason:           raw.emergencyReason,
			BuildSafetyNoNewWorkAfter: buildSafetyNoNew,
			BuildSafetyTerminateAfter: buildSafetyTerm,
		},
	}, nil
}
