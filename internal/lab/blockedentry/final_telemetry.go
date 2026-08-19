package blockedentry

import (
	"errors"
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

func admitFinalTelemetryEvidence(secretRoot string, output cellObservation) (artifactCommitment, error) {
	var raw finalRawTelemetryEvidence
	if err := readFinalHandoffArtifact(secretRoot, "final-telemetry", output.CellID,
		output.TelemetryEvidence, &raw); err != nil || raw.Schema != "ardents-h3-final-raw-telemetry-v1" ||
		raw.CellID != output.CellID || !validFinalRawTelemetry(raw.Files) {
		return artifactCommitment{}, errors.New("final cell raw telemetry is incomplete or invalid")
	}
	return output.TelemetryEvidence, nil
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
