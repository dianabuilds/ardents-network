package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

const maximumFinalTelemetryBytes = 2 << 20

type finalRawTelemetry struct {
	Root uint16 `json:"root"`
	Role string `json:"role"`
	Kind string `json:"kind"`
	Data []byte `json:"data"`
}

type finalRawTelemetryEvidence struct {
	Schema string              `json:"schema"`
	CellID string              `json:"cell_id"`
	Files  []finalRawTelemetry `json:"files"`
}

func captureFinalTelemetryEvidence(secretRoot string, output cellObservation) (artifactCommitment, error) {
	if !validFinalRawTelemetry(output.RawTelemetry) {
		return artifactCommitment{}, errors.New("final cell raw telemetry is incomplete or invalid")
	}
	digest := sha256.Sum256([]byte(output.CellID))
	relative := filepath.ToSlash(filepath.Join("final-telemetry", hex.EncodeToString(digest[:])+".json"))
	path := filepath.Join(secretRoot, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return artifactCommitment{}, err
	}
	if err := writeJSON(path, finalRawTelemetryEvidence{
		Schema: "ardents-h3-final-raw-telemetry-v1", CellID: output.CellID, Files: output.RawTelemetry}); err != nil {
		return artifactCommitment{}, err
	}
	return commitment(secretRoot, relative)
}

func validFinalRawTelemetry(files []finalRawTelemetry) bool {
	for _, file := range files {
		if file.Role != "endpoint" && file.Role != "bridge" && file.Role != "publisher" ||
			file.Kind != "resource.jsonl" && file.Kind != "carrier.jsonl" ||
			len(file.Data) == 0 || len(file.Data) > maximumFinalTelemetryBytes {
			return false
		}
	}
	return true
}
