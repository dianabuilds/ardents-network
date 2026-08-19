package blockedentry

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishFinalMeasurementsCreatesCompleteCommittedBasis(t *testing.T) {
	root := t.TempDir()
	summary := &finalSummary{
		Profiles:  []finalProfileResult{{ID: "C0"}},
		Capacity:  []finalCapacityBatch{{Profile: "reference"}},
		Sustained: []finalSustainedCell{{Direction: "endpoint-to-publisher", Runs: []finalSustainedRun{{}}}},
		Pressure:  []finalPressureCell{{ID: "P0"}},
		Recovery:  finalRecovery{Attempts: 5},
		Hosts:     []finalObservedHost{{ID: "host"}},
		Cells:     []finalCellObservation{{ID: "profile/C0/00"}},
	}
	if err := publishFinalMeasurements(root, summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Artifacts) != len(finalMeasurementPaths) {
		t.Fatalf("artifact count=%d", len(summary.Artifacts))
	}
	for index, artifact := range summary.Artifacts {
		if artifact.Path != finalMeasurementPaths[index] || artifact.SHA256 == "" || artifact.Bytes < 1 {
			t.Fatalf("artifact[%d]=%+v", index, artifact)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "measurements", "capacity.jsonl"))
	var decoded finalCapacityBatch
	decodeErr := json.Unmarshal(bytes.TrimSuffix(raw, []byte{'\n'}), &decoded)
	canonical, marshalErr := json.Marshal(decoded)
	if err != nil || decodeErr != nil || marshalErr != nil ||
		!bytes.Equal(raw, append(canonical, '\n')) || decoded.Profile != "reference" {
		t.Fatalf("capacity measurement is not canonical JSONL: read=%v decode=%v marshal=%v\n%s",
			err, decodeErr, marshalErr, raw)
	}
}
