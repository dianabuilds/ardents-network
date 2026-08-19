package blockedverify

import (
	"bufio"
	"bytes"
	"encoding/json"
)

type finalResourceSample struct {
	Schema          string `json:"schema"`
	Ordinal         uint16 `json:"ordinal"`
	OffsetMillis    uint64 `json:"offset_millis"`
	RSSBytes        uint64 `json:"rss_bytes"`
	FDs             uint16 `json:"fds"`
	Sockets         uint16 `json:"sockets"`
	Processes       uint16 `json:"processes"`
	SwapBytes       uint64 `json:"swap_bytes"`
	EmergencyEvents uint64 `json:"emergency_events"`
	Threads         uint16 `json:"threads"`
	StateBytes      uint64 `json:"state_bytes"`
	StateEntries    uint16 `json:"state_entries"`
	EvidenceRecords uint16 `json:"evidence_records"`
	EvidenceBytes   uint64 `json:"evidence_bytes"`
	Capabilities    uint16 `json:"capabilities"`
	DurableMembers  uint16 `json:"durable_members"`
	DurableContacts uint16 `json:"durable_contacts"`
	DurableAttempts uint16 `json:"durable_attempts"`
	DurableRegimes  uint16 `json:"durable_regimes"`
	Boundary        string `json:"boundary,omitempty"`
}

type finalCarrierSample struct {
	Schema       string `json:"schema"`
	Ordinal      uint16 `json:"ordinal"`
	OffsetMillis uint64 `json:"offset_millis"`
	Bytes        uint64 `json:"bytes"`
	Boundary     string `json:"boundary,omitempty"`
}

type finalTreeSample struct {
	Schema       string  `json:"schema"`
	Ordinal      uint16  `json:"ordinal"`
	OffsetMillis uint64  `json:"offset_millis"`
	CPUCores     float64 `json:"cpu_cores"`
	RSSBytes     uint64  `json:"rss_bytes"`
}

type finalRuntimeResource struct {
	CPUUsageUsec      uint64  `json:"cpu_usage_usec"`
	MemoryBytes       uint64  `json:"memory_bytes"`
	GoMemoryBytes     uint64  `json:"go_memory_bytes"`
	SocketMemoryBytes uint64  `json:"socket_memory_bytes"`
	Sockets           uint64  `json:"sockets"`
	FDs               uint64  `json:"fds"`
	Goroutines        uint64  `json:"goroutines"`
	Threads           uint64  `json:"threads"`
	Timers            uint64  `json:"timers"`
	QueueItems        uint64  `json:"queue_items"`
	QueueBytes        uint64  `json:"queue_bytes"`
	CPUPressure       float64 `json:"cpu_pressure"`
	MemoryPressure    float64 `json:"memory_pressure"`
	IOPressure        float64 `json:"io_pressure"`
	HighEvents        uint64  `json:"high_events"`
	EmergencyEvents   uint64  `json:"emergency_events"`
	AdmissionActive   uint64  `json:"admission_active"`
	AdmissionAccepted uint64  `json:"admission_accepted"`
	AdmissionRefused  uint64  `json:"admission_refused"`
}

type finalBridgeRuntime struct {
	Schema   string               `json:"schema"`
	Ordinal  uint16               `json:"ordinal"`
	Resource finalRuntimeResource `json:"resource"`
}

func validFinalTelemetryStreams(files []finalRawTelemetry) bool {
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		key := string(rune(file.Root)) + "/" + file.Role + "/" + file.Kind
		if seen[key] {
			return false
		}
		seen[key] = true
		if file.Kind == "resource.jsonl" && !validFinalResourceStream(file.Data) ||
			file.Kind == "carrier.jsonl" && !validFinalCarrierStream(file.Data) ||
			file.Kind == "tree.jsonl" && !validFinalTreeStream(file.Data) ||
			file.Kind == "runtime.jsonl" && !validFinalRuntimeStream(file.Data) ||
			file.Kind == "pressure-input.json" && !validFinalPressureInput(file.Data) ||
			file.Kind == "pressure-injection.jsonl" && !validFinalPressureInjectionStream(file.Data) ||
			file.Kind == "pressure-state.jsonl" && !validFinalPressureStateStream(file.Data) ||
			file.Kind == "pressure.json" && !validFinalPressureMeasurement(file.Data) ||
			file.Kind == "recovery.json" && !validFinalRecoveryMeasurement(file.Data) {
			return false
		}
	}
	return true
}

func validFinalRuntimeStream(raw []byte) bool {
	var samples []finalBridgeRuntime
	if !decodeFinalTelemetryLines(raw, &samples) || len(samples) == 0 {
		return false
	}
	for index, sample := range samples {
		if sample.Schema != "ardents-h3-bridge-runtime-v1" || sample.Ordinal != uint16(index) {
			return false
		}
	}
	return true
}

func validFinalTreeStream(raw []byte) bool {
	var samples []finalTreeSample
	if !decodeFinalTelemetryLines(raw, &samples) || len(samples) == 0 {
		return false
	}
	offsets := make([]uint64, len(samples))
	for index, sample := range samples {
		if sample.Schema != "ardents-h3-tree-resource-v1" || sample.Ordinal != uint16(index) ||
			sample.CPUCores < 0 || sample.CPUCores != sample.CPUCores {
			return false
		}
		offsets[index] = sample.OffsetMillis
	}
	return validFinalTelemetryCadence(offsets)
}

func validFinalResourceStream(raw []byte) bool {
	var samples []finalResourceSample
	if !decodeFinalTelemetryLines(raw, &samples) || len(samples) == 0 {
		return false
	}
	periodic := make([]finalResourceSample, 0, len(samples))
	lastBoundary := 0
	for index, sample := range samples {
		if sample.Schema != "ardents-h3-process-resource-v1" || sample.Ordinal != uint16(index) {
			return false
		}
		if sample.Boundary == "" {
			periodic = append(periodic, sample)
			continue
		}
		rank := map[string]int{"baseline": 1, "after-churn": 2, "post-cleanup": 3}[sample.Boundary]
		if rank == 0 || rank <= lastBoundary {
			return false
		}
		lastBoundary = rank
	}
	return validFinalTelemetryCadence(resourceOffsets(periodic))
}

func validFinalCarrierStream(raw []byte) bool {
	samples, ok := decodeFinalCarrierStream(raw)
	if !ok {
		return false
	}
	return validFinalTelemetryCadence(carrierOffsets(samples[1 : len(samples)-1]))
}

func decodeFinalCarrierStream(raw []byte) ([]finalCarrierSample, bool) {
	var samples []finalCarrierSample
	if !decodeFinalTelemetryLines(raw, &samples) || len(samples) < 2 ||
		samples[0].Boundary != "before" || samples[len(samples)-1].Boundary != "after" {
		return nil, false
	}
	for index, sample := range samples {
		if sample.Schema != "ardents-h3-carrier-counter-v1" || sample.Ordinal != uint16(index) ||
			index > 0 && sample.Bytes < samples[index-1].Bytes {
			return nil, false
		}
		if sample.Boundary != "" && index != 0 && index != len(samples)-1 {
			return nil, false
		}
	}
	return samples, true
}

func decodeFinalTelemetryLines[T any](raw []byte, values *[]T) bool {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), maximumFinalTelemetryBytes)
	for scanner.Scan() {
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		var value T
		if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) == nil {
			return false
		}
		*values = append(*values, value)
	}
	return scanner.Err() == nil
}

func resourceOffsets(samples []finalResourceSample) []uint64 {
	result := make([]uint64, len(samples))
	for index := range samples {
		result[index] = samples[index].OffsetMillis
	}
	return result
}

func carrierOffsets(samples []finalCarrierSample) []uint64 {
	result := make([]uint64, len(samples))
	for index := range samples {
		result[index] = samples[index].OffsetMillis
	}
	return result
}

func validFinalTelemetryCadence(offsets []uint64) bool {
	if len(offsets) == 0 {
		return false
	}
	for index := 1; index < len(offsets); index++ {
		if offsets[index] < offsets[index-1] || offsets[index]-offsets[index-1] < 750 || offsets[index]-offsets[index-1] > 1_250 {
			return false
		}
	}
	return true
}
