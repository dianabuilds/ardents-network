//go:build live

package network_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
)

type blockedProcessSample struct {
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

func readBlockedProcessSamples(path string) ([]blockedProcessSample, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	var result []blockedProcessSample
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var value blockedProcessSample
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil ||
			value.Schema != "ardents-h3-process-resource-v1" || value.Ordinal != uint16(len(result)) {
			return nil, errors.New("Bridge process resource stream is invalid or reordered")
		}
		result = append(result, value)
	}
	if err := scanner.Err(); err != nil || !completeBlockedResourceCadence(result) {
		return nil, errors.Join(err, errors.New("process resource stream is missing or coalesced"))
	}
	return result, nil
}

func completeBlockedResourceCadence(samples []blockedProcessSample) bool {
	var periodic []blockedProcessSample
	lastBoundary := 0
	for _, sample := range samples {
		if sample.Boundary == "" {
			periodic = append(periodic, sample)
		} else {
			rank := map[string]int{"baseline": 1, "after-churn": 2, "post-cleanup": 3}[sample.Boundary]
			if rank == 0 || rank <= lastBoundary {
				return false
			}
			lastBoundary = rank
		}
	}
	for index := 1; index < len(periodic); index++ {
		gap := periodic[index].OffsetMillis - periodic[index-1].OffsetMillis
		if gap < 750 || gap > 1_250 {
			return false
		}
	}
	return len(periodic) > 0
}

func mergeFinalProcessResources(path string, value finalResourceEvidence) (finalResourceEvidence, error) {
	samples, err := readBlockedProcessSamples(path)
	if err != nil || len(samples) < 600 || !hasPostCleanupSample(samples) {
		return value, errors.Join(err, errors.New("Bridge process resource stream is incomplete"))
	}
	rss := make([]float64, 0, len(samples))
	for _, sample := range samples {
		rss = append(rss, float64(sample.RSSBytes)/(1<<20))
		value.HelperFDPeak = max(value.HelperFDPeak, sample.FDs)
		value.HelperSocketPeak = max(value.HelperSocketPeak, sample.Sockets)
		value.ThreadsPeak = max(value.ThreadsPeak, sample.Threads)
		value.DurableStateBytes = max(value.DurableStateBytes, uint32(sample.StateBytes))
		value.DurableMembers = max(value.DurableMembers, sample.DurableMembers)
		value.DurableContacts = max(value.DurableContacts, sample.DurableContacts)
		value.DurableAttempts = max(value.DurableAttempts, sample.DurableAttempts)
		value.DurableRegimes = max(value.DurableRegimes, sample.DurableRegimes)
		value.EvidenceBytes = max(value.EvidenceBytes, sample.EvidenceBytes)
		value.Capabilities = max(value.Capabilities, sample.Capabilities)
		value.Descendants = max(value.Descendants, unexpectedProcesses(sample.Processes))
		if sample.SwapBytes != 0 {
			value.SwapEvents++
		}
		value.OOMEvents = uint16(max(uint64(value.OOMEvents), sample.EmergencyEvents))
	}
	value.HelperRSSP95MiB = percentile(rss, .95)
	value.EvidenceProjectedPC = float64(value.EvidenceBytes) * 100 / (16 << 20)
	return value, nil
}

func mergeFinalAdapterResources(path string, value finalResourceEvidence) (finalResourceEvidence, error) {
	samples, err := readBlockedProcessSamples(path)
	if err != nil || len(samples) < 600 || !hasPostCleanupSample(samples) {
		return value, errors.Join(err, errors.New("Endpoint Adapter resource stream is incomplete"))
	}
	rss := make([]float64, 0, len(samples))
	for _, sample := range samples {
		rss = append(rss, float64(sample.RSSBytes)/(1<<20))
		value.AdapterFDPeak = max(value.AdapterFDPeak, sample.FDs)
		value.AdapterSocketPeak = max(value.AdapterSocketPeak, sample.Sockets)
		value.AdapterStateBytes = max(value.AdapterStateBytes, uint32(sample.StateBytes))
		value.AdapterStateEntries = max(value.AdapterStateEntries, sample.StateEntries)
		value.Capabilities = max(value.Capabilities, sample.Capabilities)
		value.Descendants = max(value.Descendants, unexpectedProcesses(sample.Processes))
	}
	value.AdapterRSSP95MiB = percentile(rss, .95)
	return value, nil
}

func hasPostCleanupSample(samples []blockedProcessSample) bool {
	for _, sample := range samples {
		if sample.Boundary == "post-cleanup" {
			return true
		}
	}
	return false
}

func unexpectedProcesses(processes uint16) uint16 {
	if processes > 2 {
		return processes - 2
	}
	return 0
}

func readBlockedCarrierSamples(path string) ([]blockedCarrierSample, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	var result []blockedCarrierSample
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var value blockedCarrierSample
		if err := json.Unmarshal(scanner.Bytes(), &value); err != nil ||
			value.Schema != "ardents-h3-carrier-counter-v1" || value.Ordinal != uint16(len(result)) {
			return nil, errors.New("carrier counter stream is invalid or reordered")
		}
		result = append(result, value)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) < 602 || result[0].Boundary != "before" || result[len(result)-1].Boundary != "after" ||
		result[len(result)-1].Bytes < result[0].Bytes || !completeBlockedCarrierCadence(result) {
		return nil, errors.New("carrier counter stream is incomplete or coalesced")
	}
	return result, nil
}

func completeBlockedCarrierCadence(samples []blockedCarrierSample) bool {
	for index := 1; index < len(samples)-1; index++ {
		gap := samples[index].OffsetMillis - samples[index-1].OffsetMillis
		if samples[index].Boundary != "" || samples[index].Bytes < samples[index-1].Bytes ||
			gap < 750 || gap > 1_250 {
			return false
		}
	}
	last, prior := samples[len(samples)-1], samples[len(samples)-2]
	return last.Bytes >= prior.Bytes && last.OffsetMillis >= prior.OffsetMillis &&
		last.OffsetMillis-prior.OffsetMillis <= 1_250
}

func finalCarrierDelta(samples []blockedCarrierSample) uint64 {
	return samples[len(samples)-1].Bytes - samples[0].Bytes
}

func mergeFinalCarrierReserve(value finalResourceEvidence, samples []blockedCarrierSample) finalResourceEvidence {
	rates := make([]float64, 0, len(samples)-2)
	for index := 1; index < len(samples)-1; index++ {
		bytes := samples[index].Bytes - samples[index-1].Bytes
		seconds := float64(samples[index].OffsetMillis-samples[index-1].OffsetMillis) / 1_000
		rates = append(rates, float64(bytes)*8/seconds/1e6)
	}
	linkReserve := 100 * (1 - percentile(rates, .95)/100)
	value.ReservePercent = min(value.ReservePercent, linkReserve)
	return value
}
