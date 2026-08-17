//go:build live

package network_test

import (
	"testing"
	"time"
)

func runSelectedFinalRefusalCell(t *testing.T, repository, image, toolImage, client, server, cell string) {
	t.Helper()
	started := time.Now()
	switch cell {
	case "pressure/P0":
		result := runFinalRefusalBatch(t, repository, image, toolImage, client, server, 0, 0)
		if !result.progress || result.oom != 0 {
			t.Fatalf("P0 four-unit hold failed: %+v", result)
		}
		emitFinalWorkerCell(t, cell, "normal", started)
	case "pressure/P1":
		result := runFinalRefusalBatch(t, repository, image, toolImage, client, server, 0, 100)
		if !result.progress || result.oom != 0 || result.admission.Offers != 100 || result.admission.Refused != 100 {
			t.Fatalf("P1 projected admission failed: %+v", result)
		}
		emitFinalWorkerCell(t, cell, "normal", started)
	case "pressure/P4":
		var refused uint16
		for batch := range 10 {
			result := runFinalRefusalBatch(t, repository, image, toolImage, client, server, batch+1, 100)
			if !result.progress || result.oom != 0 || !exactLiveReconciliations([]finalReconciliationEvidence{result.reconcile}) {
				t.Fatalf("P4 batch %d failed: %+v", batch, result)
			}
			refused += result.admission.Refused
		}
		if refused != 1_000 {
			t.Fatalf("P4 refused %d/1000 offers", refused)
		}
		emitFinalWorkerCell(t, cell, "normal", started)
	default:
		t.Fatalf("invalid selected refusal cell %q", cell)
	}
}
