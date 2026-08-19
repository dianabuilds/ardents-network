package blockedverify

import (
	"fmt"
	"math"
	"reflect"
)

type finalPressureInput struct {
	Schema    string                    `json:"schema"`
	Batch     uint16                    `json:"batch"`
	Admission finalAdmissionInput       `json:"admission"`
	Before    finalPressureRuntimeInput `json:"before"`
	After     finalPressureRuntimeInput `json:"after"`
	Progress  bool                      `json:"progress"`
	Residuals uint16                    `json:"residuals"`
}

type finalAdmissionInput struct {
	Schema        string                `json:"schema"`
	Offers        uint16                `json:"offers"`
	Refused       uint16                `json:"refused"`
	MaximumMillis uint32                `json:"maximum_millis"`
	Outcomes      []finalAdmissionOffer `json:"outcomes"`
}

type finalAdmissionOffer struct {
	Ordinal               uint16 `json:"ordinal"`
	CanarySHA256          string `json:"canary_sha256"`
	ScheduledOffsetMillis uint32 `json:"scheduled_offset_millis"`
	StartedOffsetMillis   uint32 `json:"started_offset_millis"`
	RefusalMillis         uint32 `json:"refusal_millis"`
	Refused               bool   `json:"refused"`
}

type finalPressureRuntimeInput struct {
	AdmissionAccepted uint64 `json:"admission_accepted"`
	Goroutines        uint64 `json:"goroutines"`
	Timers            uint64 `json:"timers"`
}

func validFinalPressureInput(raw []byte) bool {
	var values []finalPressureInput
	return decodeFinalTelemetryLines(raw, &values) && len(values) == 1 &&
		values[0].Schema == "ardents-h3-final-pressure-input-v1" &&
		values[0].Admission.Schema == "ardents-h3-s5-admission-result-v1"
}

func finalPressureFromTelemetry(files []finalRawTelemetry, id, cellSeed string) (finalPressureCell, bool) {
	var values []finalPressureCell
	for _, file := range files {
		if file.Kind == "pressure.json" && validFinalPressureMeasurement(file.Data) &&
			decodeFinalTelemetryLines(file.Data, &values) {
			break
		}
	}
	if len(values) != 1 || values[0].ID != id || !validFinalPressureBasis(files, values[0], cellSeed) {
		return finalPressureCell{}, false
	}
	values[0].Schema = ""
	return values[0], true
}

func validFinalPressureBasis(files []finalRawTelemetry, value finalPressureCell, cellSeed string) bool {
	switch value.ID {
	case "P0", "P1":
		input, samples, ok := finalPressureRoot(files, 0, true)
		if !ok || !validFinalPressureResourceGate(samples) || value.Progress != input.Progress ||
			value.OOMEvents != finalPressureOOM(samples) || !finalPostCleanup(samples) {
			return false
		}
		if value.ID == "P0" {
			return input.Admission.Offers == 0 && input.Admission.Refused == 0 &&
				len(input.Admission.Outcomes) == 0
		}
		offers, refused, maximum, ok := validFinalAdmissionOffers(input.Admission, cellSeed, "offers")
		return ok && value.Offers == offers && value.Refused == refused &&
			value.MaximumRefusalMillis == maximum
	case "P2", "P3":
		_, samples, ok := finalPressureRoot(files, 0, false)
		count := 20
		if value.ID == "P3" {
			count = 23
		}
		injections, states, injectionOK := finalPressureInjectionBasis(files, cellSeed, count)
		return ok && injectionOK && validFinalSocketBasis(samples, injections, states, value)
	case "P4":
		return validFinalChurnBasis(files, value, cellSeed)
	default:
		return false
	}
}

func finalPressureRoot(files []finalRawTelemetry, root uint16, requireInput bool) (finalPressureInput,
	[]finalResourceSample, bool,
) {
	var input finalPressureInput
	var samples []finalResourceSample
	hasInput, hasSamples := false, false
	for _, file := range files {
		if file.Root != root || file.Role != "bridge" {
			continue
		}
		switch file.Kind {
		case "pressure-input.json":
			var values []finalPressureInput
			if !validFinalPressureInput(file.Data) || !decodeFinalTelemetryLines(file.Data, &values) {
				return input, nil, false
			}
			input, hasInput = values[0], true
		case "resource.jsonl":
			if !validFinalResourceStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &samples) {
				return input, nil, false
			}
			hasSamples = true
		}
	}
	return input, samples, hasSamples && (!requireInput || hasInput)
}

func validFinalPressureResourceGate(samples []finalResourceSample) bool {
	for _, sample := range samples {
		if sample.RSSBytes > 128<<20 || sample.FDs > 64 || sample.Sockets > 32 || sample.SwapBytes != 0 {
			return false
		}
	}
	return len(samples) > 0
}

func validFinalSocketBasis(samples []finalResourceSample, injections []finalPressureInjection,
	states []finalPressureStateObservation, value finalPressureCell,
) bool {
	if !validFinalPressureResourceGate(samples) || !finalPostCleanup(samples) ||
		value.OOMEvents != finalPressureOOM(samples) || value.BaselineSockets != 6 || value.PartialBytes != 128 ||
		value.RatePerSecond != 2 || value.Injected != uint16(len(injections)) || !value.Cleanup ||
		!value.Progress || states[0].State != "NORMAL" {
		return false
	}
	var peak uint16
	var baseline, highBeforeProtect, lowAfterProtect uint16
	for _, sample := range samples {
		if sample.Boundary != "" {
			continue
		}
		peak = max(peak, sample.Sockets)
		if sample.Sockets == value.BaselineSockets {
			baseline++
		}
		if sample.Sockets >= value.PeakSockets && sample.OffsetMillis <= states[1].OffsetMillis {
			highBeforeProtect++
		}
		if sample.Sockets <= 19 && sample.OffsetMillis > states[1].OffsetMillis &&
			sample.OffsetMillis <= states[2].OffsetMillis {
			lowAfterProtect++
		}
	}
	if peak != value.PeakSockets || baseline == 0 {
		return false
	}
	if value.ID == "P2" {
		return value.PeakSockets == 26 && value.HighSamples == 3 && value.LowSamples == 120 &&
			value.Protect && value.Normal && !value.Drain && states[1].State == "PROTECT" &&
			states[2].State == "NORMAL" && highBeforeProtect >= 3 && lowAfterProtect >= 120 &&
			states[1].OffsetMillis >= injections[len(injections)-1].StartedOffsetMillis
	}
	return value.PeakSockets == 29 && value.Drain && !value.Normal && states[1].State == "DRAIN" &&
		states[2].State == "EXIT" && highBeforeProtect > 0 && states[1].OffsetMillis >= injections[len(injections)-1].StartedOffsetMillis &&
		states[2].OffsetMillis-states[1].OffsetMillis <= 60_000 &&
		value.ExitMillis == uint32(states[2].OffsetMillis-states[1].OffsetMillis)
}

func validFinalChurnBasis(files []finalRawTelemetry, value finalPressureCell, cellSeed string) bool {
	derived := make([]finalReconciliation, 0, 10)
	var offers, refused, oom, residuals uint16
	var maximum uint32
	progress := true
	for root := range 10 {
		input, samples, ok := finalPressureRoot(files, uint16(root), true)
		batchOffers, batchRefused, batchMaximum, admissionOK := validFinalAdmissionOffers(input.Admission,
			cellSeed, fmt.Sprintf("batch-%02d-offers", root))
		if !ok || !admissionOK || input.Batch != uint16(root) || !validFinalPressureResourceGate(samples) ||
			!finalPostCleanup(samples) || batchOffers != 100 {
			return false
		}
		offers += batchOffers
		refused += batchRefused
		oom += finalPressureOOM(samples)
		residuals += input.Residuals
		maximum = max(maximum, batchMaximum)
		progress = progress && input.Progress
		reconciliation, ok := deriveFinalReconciliation(input, samples)
		if !ok {
			return false
		}
		derived = append(derived, reconciliation)
	}
	return value.Offers == offers && value.Refused == refused && value.MaximumRefusalMillis == maximum &&
		value.OOMEvents == oom && value.Residuals == residuals && value.Progress == progress &&
		value.UpwardTrend == !exactReconciliations(derived) && reflect.DeepEqual(value.Reconciliations, derived)
}

func deriveFinalReconciliation(input finalPressureInput, samples []finalResourceSample) (finalReconciliation, bool) {
	first, firstOK := finalPressureBoundary(samples, "baseline")
	last, lastOK := finalPressureBoundary(samples, "after-churn")
	cleanup, cleanupOK := finalPressureBoundary(samples, "post-cleanup")
	if !firstOK || !lastOK || !cleanupOK || input.Before.AdmissionAccepted > math.MaxInt32 ||
		input.After.AdmissionAccepted > math.MaxInt32 || input.Before.Goroutines > math.MaxInt32 ||
		input.After.Goroutines > math.MaxInt32 || input.Before.Timers > math.MaxInt32 ||
		input.After.Timers > math.MaxInt32 || first.StateBytes > math.MaxInt64 || last.StateBytes > math.MaxInt64 {
		return finalReconciliation{}, false
	}
	return finalReconciliation{Batch: input.Batch,
		AllocationDelta:      int32(input.After.AdmissionAccepted) - int32(input.Before.AdmissionAccepted),
		FDDelta:              int32(last.FDs) - int32(first.FDs),
		SocketDelta:          int32(last.Sockets) - int32(first.Sockets),
		GoroutineDelta:       int32(input.After.Goroutines) - int32(input.Before.Goroutines),
		TimerDelta:           int32(input.After.Timers) - int32(input.Before.Timers),
		StateBytesDelta:      int64(last.StateBytes) - int64(first.StateBytes),
		EvidenceRecordsDelta: int32(last.EvidenceRecords) - int32(first.EvidenceRecords),
		CleanupSockets:       cleanup.Sockets, CleanupDescendants: finalUnexpectedProcesses(cleanup.Processes),
		CleanupStateBytes: cleanup.StateBytes, Residuals: input.Residuals}, true
}

func finalPressureBoundary(samples []finalResourceSample, boundary string) (finalResourceSample, bool) {
	for _, sample := range samples {
		if sample.Boundary == boundary {
			return sample, true
		}
	}
	return finalResourceSample{}, false
}

func finalPressureOOM(samples []finalResourceSample) uint16 {
	var value uint64
	for _, sample := range samples {
		value = max(value, sample.EmergencyEvents)
	}
	if value > math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(value)
}
