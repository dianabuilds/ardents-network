//go:build live

package network_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinalRunnerSummaryPreservesFrozenCellOrder(t *testing.T) {
	schedule := finalSupplyFixture()
	schedule.CellOrder, schedule.Seeds = []string{"profile/C1/00", "profile/C1/01"},
		[]string{strings.Repeat("a", 64), strings.Repeat("b", 64)}
	cached := map[string]finalWorkerResult{
		"profile/C1/00": {CellID: "profile/C1/00", Terminal: "success"},
		"profile/C1/01": {CellID: "profile/C1/01", Terminal: "success"},
	}
	summary := finalRunnerSummaryFromWorkers(schedule, cached)
	if summary == nil || len(summary.Cells) != 2 || summary.Cells[1].ID != "profile/C1/01" ||
		summary.Cells[1].Seed != schedule.Seeds[1] {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestFinalRunnerSummaryCombinesSustainedWorkerMeasurements(t *testing.T) {
	schedule := finalSupplyFixture()
	cached := make(map[string]finalWorkerResult)
	for _, direction := range []string{"endpoint-to-publisher", "publisher-to-endpoint"} {
		pairID := strings.Repeat(string(direction[0]), 64)
		for _, kind := range []string{"direct-before", "run-0", "run-1", "run-2", "run-3", "run-4", "direct-after"} {
			cell := "sustained/" + direction + "/" + kind
			schedule.CellOrder = append(schedule.CellOrder, cell)
			schedule.Seeds = append(schedule.Seeds, strings.Repeat("a", 64))
			measurement := &finalWorkerSustained{Direction: direction, Kind: kind}
			if strings.HasPrefix(kind, "direct-") {
				measurement.DirectMbit = 20
				measurement.Direct = &finalDirectRunEvidence{PairID: pairID, DeliveredBytes: 1}
			} else {
				measurement.Run = &finalSustainedRunEvidence{DeliveredBytes: 100}
				measurement.EndpointCarrierBytes, measurement.PublisherCarrierBytes = 110, 120
			}
			cached[cell] = finalWorkerResult{CellID: cell, Terminal: "complete", Sustained: measurement}
		}
	}
	summary := finalRunnerSummaryFromWorkers(schedule, cached)
	if summary == nil || len(summary.Sustained) != 2 || len(summary.Sustained[0].Runs) != 5 ||
		summary.Sustained[1].EndpointCarrierBytes != 550 || summary.Sustained[1].PublisherCarrierRatio != 1.2 {
		t.Fatalf("sustained summary=%+v", summary)
	}
}

func TestFinalRunnerProfilesRequireEveryFrozenEpisode(t *testing.T) {
	cached := make(map[string]finalWorkerResult)
	for _, profile := range finalRunnerProfileFloor() {
		for episode := range profile.attempts {
			cell := fmt.Sprintf("profile/%s/%02d", profile.id, episode)
			cached[cell] = finalWorkerResult{CellID: cell, Terminal: profile.terminal}
		}
	}
	profiles, ok := finalRunnerProfilesFromWorkers(cached)
	if !ok || len(profiles) != 7 || profiles[6].Successful != 20 {
		t.Fatalf("profiles=%+v ok=%t", profiles, ok)
	}
	delete(cached, "profile/C6/19")
	if _, ok := finalRunnerProfilesFromWorkers(cached); ok {
		t.Fatal("partial profile floor was accepted")
	}
}

func TestFinalRunnerProfilesPreserveAdverseOutcomeForVerifier(t *testing.T) {
	cached := make(map[string]finalWorkerResult)
	for _, profile := range finalRunnerProfileFloor() {
		for episode := range profile.attempts {
			cell := fmt.Sprintf("profile/%s/%02d", profile.id, episode)
			cached[cell] = finalWorkerResult{CellID: cell, Terminal: profile.terminal}
		}
	}
	cached["profile/C1/03"] = finalWorkerResult{CellID: "profile/C1/03", Terminal: "failed"}
	profiles, ok := finalRunnerProfilesFromWorkers(cached)
	if !ok || profiles[1].Successful != 19 {
		t.Fatalf("profiles=%+v ok=%t", profiles, ok)
	}
}

func TestFinalRunnerPressureRequiresAllFiveMeasuredCells(t *testing.T) {
	cached := make(map[string]finalWorkerResult)
	for index := range 5 {
		cell, id := fmt.Sprintf("pressure/P%d", index), fmt.Sprintf("P%d", index)
		cached[cell] = finalWorkerResult{CellID: cell, Terminal: "normal",
			Pressure: &finalPressureEvidence{ID: id, Terminal: "normal"}}
	}
	values, ok := finalRunnerPressureFromWorkers(cached)
	if !ok || len(values) != 5 || values[4].ID != "P4" {
		t.Fatalf("pressure=%+v ok=%t", values, ok)
	}
	delete(cached, "pressure/P3")
	if _, ok := finalRunnerPressureFromWorkers(cached); ok {
		t.Fatal("partial pressure evidence was accepted")
	}
}

func TestFinalRunnerWithholdsPartialFinalSummary(t *testing.T) {
	schedule := finalSupplyFixture()
	schedule.CellOrder, schedule.Seeds = []string{"capacity/h3-s5-b1-v1/0"}, []string{strings.Repeat("a", 64)}
	cached := map[string]finalWorkerResult{"capacity/h3-s5-b1-v1/0": {
		CellID: "capacity/h3-s5-b1-v1/0", Terminal: "complete",
	}}
	if summary := finalRunnerSummaryFromWorkers(schedule, cached); summary != nil {
		t.Fatalf("partial capacity summary was published: %+v", summary)
	}
}
