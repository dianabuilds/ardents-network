package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
	wanted := finalTelemetryLayout(cell)
	if len(files) != len(wanted) {
		return false
	}
	for index, file := range files {
		expected := wanted[index]
		if file.Root != expected.root || file.Role != expected.role || file.Kind != expected.kind ||
			file.Artifact.Path != finalTelemetryStreamPath(cell, index) ||
			file.Artifact.Bytes < 1 || file.Artifact.Bytes > maximumFinalTelemetryBytes ||
			!hexDigest(file.Artifact.SHA256, 32) {
			return false
		}
	}
	return true
}

type finalTelemetrySlot struct {
	root uint16
	role string
	kind string
}

func finalTelemetryLayout(cell string) []finalTelemetrySlot {
	var result []finalTelemetrySlot
	appendRole := func(root uint16, role string, tree bool) {
		kinds := []string{"resource.jsonl", "carrier.jsonl"}
		if tree {
			kinds = append(kinds, "tree.jsonl")
			if role == "bridge" {
				kinds = append(kinds, "runtime.jsonl")
			}
		}
		for _, kind := range kinds {
			result = append(result, finalTelemetrySlot{root, role, kind})
		}
	}
	capacity := 0
	if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1-strong/") {
		capacity = 16
	} else if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1/") {
		capacity = 4
	}
	if capacity > 0 {
		for root := range capacity {
			appendRole(uint16(root), "endpoint", true)
		}
		appendRole(0, "bridge", true)
		appendRole(0, "publisher", true)
		return result
	}
	if strings.HasPrefix(cell, "pressure/") {
		roots := 1
		if cell == "pressure/P4" {
			roots = 10
		}
		for root := range roots {
			result = append(result, finalTelemetrySlot{uint16(root), "bridge", "resource.jsonl"})
			if cell == "pressure/P0" || cell == "pressure/P1" || cell == "pressure/P4" {
				result = append(result, finalTelemetrySlot{uint16(root), "bridge", "pressure-input.json"})
			} else {
				result = append(result, finalTelemetrySlot{uint16(root), "pressure", "pressure-injection.jsonl"})
				result = append(result, finalTelemetrySlot{uint16(root), "bridge", "pressure-state.jsonl"})
			}
		}
		return append(result, finalTelemetrySlot{0, "bridge", "pressure.json"})
	}
	roots := 1
	for root := range roots {
		for _, role := range []string{"endpoint", "bridge", "publisher"} {
			appendRole(uint16(root), role, strings.HasPrefix(cell, "sustained/"))
		}
	}
	if strings.HasPrefix(cell, "recovery/") {
		result = append(result, finalTelemetrySlot{0, "bridge", "recovery.json"})
	}
	return result
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
