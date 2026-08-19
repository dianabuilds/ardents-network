//go:build live

package network_test

import (
	"fmt"
	"strings"
)

func finalRunnerSummaryFromWorkers(schedule finalRunnerSchedule, cached map[string]finalWorkerResult) *finalRunnerSummary {
	summary := &finalRunnerSummary{Schema: "ardents-h3-s5-final-summary-v1", ImageHash: schedule.ImageSHA256,
		ClientHash: schedule.ClientSHA256, ServerHash: schedule.ServerSHA256,
		Cells: make([]finalRunnerCell, 0, len(schedule.CellOrder))}
	for index, id := range schedule.CellOrder {
		value, ok := cached[id]
		if !ok || value.CellID != id {
			return nil
		}
		summary.Cells = append(summary.Cells, finalRunnerCell{ID: id, Seed: schedule.Seeds[index], Terminal: value.Terminal,
			StartedOffsetMillis: value.StartedOffsetMillis, TerminalOffsetMillis: value.TerminalOffsetMillis,
			CleanupOffsetMillis: value.CleanupOffsetMillis})
	}
	if finalScheduleIncludesProfiles(schedule) {
		var ok bool
		if summary.Profiles, ok = finalRunnerProfilesFromWorkers(cached); !ok {
			return nil
		}
	}
	if finalScheduleHasPrefix(schedule, "sustained/") {
		var ok bool
		if summary.Sustained, ok = finalRunnerSustainedFromWorkers(cached); !ok {
			return nil
		}
	}
	if finalScheduleHasPrefix(schedule, "pressure/") {
		var ok bool
		if summary.Pressure, ok = finalRunnerPressureFromWorkers(cached); !ok {
			return nil
		}
	}
	if finalScheduleHasPrefix(schedule, "capacity/") {
		var ok bool
		if summary.Capacity, ok = finalRunnerCapacityFromWorkers(cached); !ok {
			return nil
		}
	}
	if finalScheduleHasPrefix(schedule, "recovery/") {
		var ok bool
		if summary.Recovery, ok = finalRunnerRecoveryFromWorkers(cached); !ok {
			return nil
		}
	}
	return summary
}

func finalRunnerCapacityFromWorkers(cached map[string]finalWorkerResult) ([]finalWorkerCapacity, bool) {
	result := make([]finalWorkerCapacity, 0, 10)
	for _, profile := range []string{"h3-s5-b1-v1", "h3-s5-b1-v1-strong"} {
		for batch := range 5 {
			cell := fmt.Sprintf("capacity/%s/%d", profile, batch)
			worker, ok := cached[cell]
			if !ok || worker.Capacity == nil || worker.Capacity.Profile != profile ||
				worker.Capacity.Batch != uint16(batch) || worker.Capacity.Terminal != worker.Terminal {
				return nil, false
			}
			result = append(result, *worker.Capacity)
		}
	}
	return result, true
}

func finalRunnerRecoveryFromWorkers(cached map[string]finalWorkerResult) (finalRunnerRecovery, bool) {
	result := finalRunnerRecovery{AttemptIdentityStable: true, DeadlineStable: true}
	for episode := range 5 {
		cell := fmt.Sprintf("recovery/%d", episode)
		worker, ok := cached[cell]
		if !ok || worker.Recovery == nil || worker.Recovery.Episode != uint16(episode) {
			return finalRunnerRecovery{}, false
		}
		value := worker.Recovery
		connectionLoss, attemptStable, deadlineStable := finalRecoveryOutcome(*value)
		result.Attempts++
		if connectionLoss {
			result.ConnectionLoss++
		}
		result.LaterStarts += value.LaterOrdinals
		result.Residuals += value.Residuals
		result.AttemptIdentityStable = result.AttemptIdentityStable && attemptStable
		result.DeadlineStable = result.DeadlineStable && deadlineStable
	}
	return result, true
}

func finalScheduleIncludesProfiles(schedule finalRunnerSchedule) bool {
	for _, profile := range finalRunnerProfileFloor() {
		for episode := range profile.attempts {
			if !finalScheduleHasCell(schedule, fmt.Sprintf("profile/%s/%02d", profile.id, episode)) {
				return false
			}
		}
	}
	return true
}

func finalScheduleHasCell(schedule finalRunnerSchedule, wanted string) bool {
	for _, cell := range schedule.CellOrder {
		if cell == wanted {
			return true
		}
	}
	return false
}

type finalRunnerProfileRequirement struct {
	id       string
	terminal string
	attempts uint16
}

func finalRunnerProfileFloor() []finalRunnerProfileRequirement {
	return []finalRunnerProfileRequirement{{"C0", "success", 20}, {"C1", "success", 20}, {"C2", "success", 20},
		{"C3", "bridge-attempt-exhausted", 5}, {"C4", "bridge-attempt-exhausted", 5},
		{"C5", "probe-contained", 20}, {"C6", "limitation-recorded", 20}}
}

func finalRunnerProfilesFromWorkers(cached map[string]finalWorkerResult) ([]finalRunnerProfile, bool) {
	result := make([]finalRunnerProfile, 0, 7)
	for _, profile := range finalRunnerProfileFloor() {
		value := finalRunnerProfile{ID: profile.id, Terminal: profile.terminal, Attempts: profile.attempts}
		for episode := range profile.attempts {
			worker, ok := cached[fmt.Sprintf("profile/%s/%02d", profile.id, episode)]
			if !ok {
				return nil, false
			}
			if worker.Terminal == profile.terminal {
				value.Successful++
			}
		}
		result = append(result, value)
	}
	return result, true
}

func finalScheduleHasPrefix(schedule finalRunnerSchedule, prefix string) bool {
	for _, cell := range schedule.CellOrder {
		if strings.HasPrefix(cell, prefix) {
			return true
		}
	}
	return false
}

func finalRunnerSustainedFromWorkers(cached map[string]finalWorkerResult) ([]finalSustainedCellEvidence, bool) {
	result := make([]finalSustainedCellEvidence, 0, 2)
	for _, direction := range []string{"endpoint-to-publisher", "publisher-to-endpoint"} {
		value := finalSustainedCellEvidence{Schema: "ardents-h3-final-sustained-v1", Direction: direction}
		for _, kind := range []string{"direct-before", "run-0", "run-1", "run-2", "run-3", "run-4", "direct-after"} {
			cell := "sustained/" + direction + "/" + kind
			worker, ok := cached[cell]
			if !ok || worker.Sustained == nil || worker.Sustained.Direction != direction || worker.Sustained.Kind != kind {
				return nil, false
			}
			measurement := worker.Sustained
			switch kind {
			case "direct-before":
				if measurement.Direct == nil {
					return nil, false
				}
				value.DirectBeforeMbit, value.DirectBefore = measurement.DirectMbit, *measurement.Direct
				value.DirectBeforeValid, value.DirectPairID = true, measurement.Direct.PairID
			case "direct-after":
				if measurement.Direct == nil || value.DirectPairID != measurement.Direct.PairID {
					return nil, false
				}
				value.DirectAfterMbit, value.DirectAfter, value.DirectAfterValid = measurement.DirectMbit, *measurement.Direct, true
			default:
				if measurement.Run == nil {
					return nil, false
				}
				value.Runs = append(value.Runs, *measurement.Run)
				value.EndpointCarrierBytes += measurement.EndpointCarrierBytes
				value.PublisherCarrierBytes += measurement.PublisherCarrierBytes
				value.DeliveredBytes += measurement.Run.DeliveredBytes
			}
		}
		if value.DeliveredBytes == 0 {
			return nil, false
		}
		value.EndpointCarrierRatio = float64(value.EndpointCarrierBytes) / float64(value.DeliveredBytes)
		value.PublisherCarrierRatio = float64(value.PublisherCarrierBytes) / float64(value.DeliveredBytes)
		result = append(result, value)
	}
	return result, true
}

func finalRunnerPressureFromWorkers(cached map[string]finalWorkerResult) ([]finalRunnerPressure, bool) {
	result := make([]finalRunnerPressure, 0, 5)
	for index := range 5 {
		cell := fmt.Sprintf("pressure/P%d", index)
		worker, ok := cached[cell]
		if !ok || worker.Pressure == nil || worker.Pressure.ID != fmt.Sprintf("P%d", index) ||
			worker.Pressure.Terminal != worker.Terminal {
			return nil, false
		}
		result = append(result, finalRunnerPressureFromEvidence(*worker.Pressure))
	}
	return result, true
}

func finalRunnerPressureFromEvidence(value finalPressureEvidence) finalRunnerPressure {
	result := finalRunnerPressure{ID: value.ID, Terminal: value.Terminal, BaselineSockets: value.BaselineSockets,
		Injected: value.Injected, PeakSockets: value.PeakSockets, Offers: value.Offers, Refused: value.Refused,
		HighSamples: value.HighSamples, LowSamples: value.LowSamples, Batches: value.Batches, Units: value.Units,
		StreamMbit: value.StreamMbit, DurationMillis: value.DurationMillis, CadenceMillis: value.CadenceMillis,
		PartialBytes: value.PartialBytes, RatePerSecond: value.RatePerSecond,
		MaximumRefusalMillis: value.MaximumRefusalMillis, ExitMillis: value.ExitMillis, Progress: value.Progress,
		Protect: value.Protect, Drain: value.Drain, Normal: value.Normal, Cleanup: value.Cleanup,
		OOMEvents: value.OOMEvents, Residuals: value.Residuals, UpwardTrend: value.UpwardTrend}
	for _, reconciliation := range value.Reconciliations {
		result.Reconciliations = append(result.Reconciliations, finalRunnerReconciliation{Batch: reconciliation.Batch,
			AllocationDelta: reconciliation.AllocationDelta, FDDelta: reconciliation.FDDelta,
			SocketDelta: reconciliation.SocketDelta, GoroutineDelta: reconciliation.GoroutineDelta,
			TimerDelta: reconciliation.TimerDelta, StateBytesDelta: reconciliation.StateBytesDelta,
			EvidenceRecordsDelta: reconciliation.EvidenceRecordsDelta, CleanupSockets: reconciliation.CleanupSockets,
			CleanupDescendants: reconciliation.CleanupDescendants, CleanupStateBytes: reconciliation.CleanupStateBytes,
			Residuals: reconciliation.Residuals})
	}
	return result
}
