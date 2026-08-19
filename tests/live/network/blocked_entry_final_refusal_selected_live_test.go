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
		armFinalWorkerTerminal("normal")
		result := runFinalRefusalBatch(t, repository, image, toolImage, client, server, 0, 0)
		if !result.progress || result.oom != 0 {
			t.Fatalf("P0 four-unit hold failed: %+v", result)
		}
		emitFinalWorkerPressure(t, cell, "normal", started, finalPressureEvidence{Schema: "ardents-h3-final-pressure-v1",
			ID: "P0", Terminal: "normal", Units: 4, StreamMbit: 10, DurationMillis: 30_000,
			Progress: result.progress, Cleanup: true, OOMEvents: result.oom}, result.root)
	case "pressure/P1":
		armFinalWorkerTerminal("normal")
		result := runFinalRefusalBatch(t, repository, image, toolImage, client, server, 0, 100)
		if !result.progress || result.oom != 0 || result.admission.Offers != 100 || result.admission.Refused != 100 {
			t.Fatalf("P1 projected admission failed: %+v", result)
		}
		emitFinalWorkerPressure(t, cell, "normal", started, finalPressureEvidence{Schema: "ardents-h3-final-pressure-v1",
			ID: "P1", Terminal: "normal", Offers: result.admission.Offers, Refused: result.admission.Refused,
			CadenceMillis: 100, DurationMillis: 10_000, MaximumRefusalMillis: result.admission.MaximumMillis,
			Progress: result.progress, Cleanup: true, OOMEvents: result.oom}, result.root)
	case "pressure/P4":
		var refused uint16
		var roots []string
		value := finalPressureEvidence{Schema: "ardents-h3-final-pressure-v1", ID: "P4", Terminal: "normal",
			Offers: 1_000, Batches: 10, CadenceMillis: 100, DurationMillis: 100_000, Progress: true, Cleanup: true}
		for batch := range 10 {
			if batch == 9 {
				armFinalWorkerTerminal("normal")
			}
			result := runFinalRefusalBatch(t, repository, image, toolImage, client, server, batch+1, 100)
			roots = append(roots, result.root)
			if !result.progress || result.oom != 0 || !exactLiveReconciliations([]finalReconciliationEvidence{result.reconcile}) {
				t.Fatalf("P4 batch %d failed: %+v", batch, result)
			}
			refused += result.admission.Refused
			result.reconcile.Batch = uint16(batch)
			value.Reconciliations = append(value.Reconciliations, result.reconcile)
			value.MaximumRefusalMillis = max(value.MaximumRefusalMillis, result.admission.MaximumMillis)
			value.OOMEvents += result.oom
		}
		if refused != 1_000 {
			t.Fatalf("P4 refused %d/1000 offers", refused)
		}
		value.Refused = refused
		value.UpwardTrend = !exactLiveReconciliations(value.Reconciliations)
		emitFinalWorkerPressure(t, cell, "normal", started, value, roots...)
	default:
		t.Fatalf("invalid selected refusal cell %q", cell)
	}
}
