package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
)

const maximumFinalTelemetryBytes = 2 << 20

type finalRawTelemetry struct {
	Root     uint16             `json:"root"`
	Role     string             `json:"role"`
	Kind     string             `json:"kind"`
	Artifact artifactCommitment `json:"artifact"`
}

type finalRawTelemetryEvidence struct {
	Schema string              `json:"schema"`
	CellID string              `json:"cell_id"`
	Files  []finalRawTelemetry `json:"files"`
}

func admitFinalTelemetryEvidence(secretRoot string, output cellObservation) (artifactCommitment, error) {
	var raw finalRawTelemetryEvidence
	if err := readFinalHandoffArtifact(secretRoot, "final-telemetry", output.CellID,
		output.TelemetryEvidence, &raw); err != nil || raw.Schema != "ardents-h3-final-raw-telemetry-v2" ||
		raw.CellID != output.CellID || !validFinalRawTelemetry(raw.Files, output.CellID) {
		return artifactCommitment{}, errors.New("final cell raw telemetry is incomplete or invalid")
	}
	for index, file := range raw.Files {
		if err := admitFinalTelemetryStream(secretRoot, output.CellID, index, file.Artifact); err != nil {
			return artifactCommitment{}, errors.Join(err, errors.New("final cell raw telemetry stream is invalid"))
		}
	}
	return output.TelemetryEvidence, nil
}

func validFinalRawTelemetry(files []finalRawTelemetry, cell string) bool {
	roots := 1
	if cell == "pressure/P4" {
		roots = 10
	}
	if len(files) != roots*6 {
		return false
	}
	expected := 0
	for index, file := range files {
		root := expected / 6
		role := []string{"endpoint", "bridge", "publisher"}[(expected%6)/2]
		kind := []string{"resource.jsonl", "carrier.jsonl"}[expected%2]
		if file.Root != uint16(root) || file.Role != role || file.Kind != kind ||
			file.Artifact.Path != finalTelemetryStreamPath(cell, index) ||
			file.Artifact.Bytes < 1 || file.Artifact.Bytes > maximumFinalTelemetryBytes ||
			!hexDigest(file.Artifact.SHA256, 32) {
			return false
		}
		expected++
	}
	return true
}

func admitFinalTelemetryStream(root, cell string, index int, artifact artifactCommitment) error {
	expected := finalTelemetryStreamPath(cell, index)
	path := filepath.Join(root, filepath.FromSlash(expected))
	aliased, aliasErr := pathHasSymlink(path)
	if artifact.Path != expected || aliasErr != nil || aliased {
		return errors.Join(aliasErr, errors.New("final telemetry stream path is invalid or aliased"))
	}
	raw, err := readStableFinalArtifact(path)
	if err != nil || int64(len(raw)) != artifact.Bytes || len(raw) > maximumFinalTelemetryBytes {
		return errors.Join(err, errors.New("final telemetry stream size differs from commitment"))
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return errors.New("final telemetry stream hash differs from commitment")
	}
	return nil
}

func finalTelemetryStreamPath(cell string, index int) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join("final-telemetry", hex.EncodeToString(digest[:]),
		fmt.Sprintf("%03d.jsonl", index)))
}
