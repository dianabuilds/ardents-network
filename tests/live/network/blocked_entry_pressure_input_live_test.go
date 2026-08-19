//go:build live

package network_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type finalPressureInput struct {
	Schema    string                    `json:"schema"`
	Batch     uint16                    `json:"batch"`
	Admission blockedAdmissionResult    `json:"admission"`
	Before    finalPressureRuntimeInput `json:"before"`
	After     finalPressureRuntimeInput `json:"after"`
	Progress  bool                      `json:"progress"`
	Residuals uint16                    `json:"residuals"`
}

type finalPressureRuntimeInput struct {
	AdmissionAccepted uint64 `json:"admission_accepted"`
	Goroutines        uint64 `json:"goroutines"`
	Timers            uint64 `json:"timers"`
}

func writeFinalPressureInput(t *testing.T, root string, batch int, admission blockedAdmissionResult,
	before, after resource.Sample, progress bool, residuals uint16,
) {
	t.Helper()
	if admission.Schema == "" {
		admission.Schema = "ardents-h3-s5-admission-result-v1"
	}
	value := finalPressureInput{Schema: "ardents-h3-final-pressure-input-v1", Batch: uint16(batch),
		Admission: admission, Before: finalPressureRuntimeObservation(before),
		After: finalPressureRuntimeObservation(after), Progress: progress, Residuals: residuals}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sync", "bridge", "pressure-input.json"),
		append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func finalPressureRuntimeObservation(value resource.Sample) finalPressureRuntimeInput {
	return finalPressureRuntimeInput{AdmissionAccepted: value.AdmissionAccepted,
		Goroutines: value.Goroutines, Timers: value.Timers}
}
