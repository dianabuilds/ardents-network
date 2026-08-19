package blockedverify

import (
	"math"
	"reflect"
	"sort"
)

func reproducesFinalRoleResources(files []finalRawTelemetry, published finalResourceObservation,
	started, finished uint64,
) bool {
	endpoint, endpointOK := finalResourceSamples(files, "endpoint")
	bridge, bridgeOK := finalResourceSamples(files, "bridge")
	publisher, publisherOK := finalResourceSamples(files, "publisher")
	endpointActive, endpointComplete := finalSustainedResourceInterval(endpoint, started, finished)
	bridgeActive, bridgeComplete := finalSustainedResourceInterval(bridge, started, finished)
	_, publisherComplete := finalSustainedResourceInterval(publisher, started, finished)
	if !endpointOK || !bridgeOK || !publisherOK || !endpointComplete || !bridgeComplete || !publisherComplete {
		return false
	}
	derived := finalRoleResourceAggregate(endpointActive, bridgeActive)
	endpointTree, endpointTreeOK := finalActiveTreeSamples(files, "endpoint", started, finished, true)
	bridgeTree, bridgeTreeOK := finalActiveTreeSamples(files, "bridge", started, finished, true)
	if !endpointTreeOK || !bridgeTreeOK {
		return false
	}
	mergeFinalTreeResources(&derived, endpointTree, bridgeTree)
	derived.ReservePercent = finalTreeReserve(derived, 1.6, 1280)
	if link, ok := finalCarrierReserve(files, "bridge"); ok {
		derived.ReservePercent = min(derived.ReservePercent, link)
	} else {
		return false
	}
	derived.Samples, derived.SamplesComplete = uint16(len(endpointTree)), true
	if !mergeFinalRuntimeResources(files, &derived) {
		return false
	}
	return published.EndpointCPUMean == derived.EndpointCPUMean &&
		published.EndpointCPUP95 == derived.EndpointCPUP95 &&
		published.EndpointRSSP95MiB == derived.EndpointRSSP95MiB &&
		published.BridgeCPUMean == derived.BridgeCPUMean &&
		published.BridgeCPUP95 == derived.BridgeCPUP95 &&
		published.BridgeMemoryP95MiB == derived.BridgeMemoryP95MiB &&
		published.ReservePercent == derived.ReservePercent && published.Samples == derived.Samples &&
		published.SamplesComplete == derived.SamplesComplete &&
		published.HelperRSSP95MiB == derived.HelperRSSP95MiB &&
		published.HelperFDPeak == derived.HelperFDPeak &&
		published.HelperSocketPeak == derived.HelperSocketPeak &&
		published.SwapEvents == derived.SwapEvents && published.OOMEvents == derived.OOMEvents &&
		published.AdapterRSSP95MiB == derived.AdapterRSSP95MiB &&
		published.AdapterFDPeak == derived.AdapterFDPeak &&
		published.AdapterSocketPeak == derived.AdapterSocketPeak &&
		published.AdapterStateBytes == derived.AdapterStateBytes &&
		published.AdapterStateEntries == derived.AdapterStateEntries &&
		published.ThreadsPeak == derived.ThreadsPeak &&
		published.GoroutinesPeak == derived.GoroutinesPeak &&
		published.TimersPeak == derived.TimersPeak && published.QueueItemsPeak == derived.QueueItemsPeak &&
		published.QueueBytesPeak == derived.QueueBytesPeak && published.EvidenceDropped == derived.EvidenceDropped &&
		published.DurableMembers == derived.DurableMembers &&
		published.DurableContacts == derived.DurableContacts &&
		published.DurableAttempts == derived.DurableAttempts &&
		published.DurableRegimes == derived.DurableRegimes &&
		published.DurableStateBytes == derived.DurableStateBytes &&
		published.EvidenceBytes == derived.EvidenceBytes &&
		published.EvidenceProjectedPC == derived.EvidenceProjectedPC &&
		published.Descendants == derived.Descendants && published.Capabilities == derived.Capabilities &&
		reflect.DeepEqual(published.Collected, requiredResourceObservations())
}

func finalActiveTreeSamples(files []finalRawTelemetry, role string, started, finished uint64,
	sustained bool,
) ([]finalTreeSample, bool) {
	for _, file := range files {
		if file.Role != role || file.Kind != "tree.jsonl" {
			continue
		}
		var samples []finalTreeSample
		if !validFinalTreeStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &samples) {
			return nil, false
		}
		if !sustained {
			return samples, len(samples) > 0
		}
		active := samples[:0]
		for _, sample := range samples {
			if sample.OffsetMillis >= started && sample.OffsetMillis < finished {
				active = append(active, sample)
			}
		}
		return active, len(active) >= 600
	}
	return nil, false
}

func mergeFinalTreeResources(value *finalResourceObservation, endpoint, bridge []finalTreeSample) {
	endpointCPU, endpointRSS := make([]float64, 0, len(endpoint)), make([]float64, 0, len(endpoint))
	bridgeCPU, bridgeMemory := make([]float64, 0, len(bridge)), make([]float64, 0, len(bridge))
	for _, sample := range endpoint {
		endpointCPU, endpointRSS = append(endpointCPU, sample.CPUCores),
			append(endpointRSS, float64(sample.RSSBytes)/(1<<20))
	}
	for _, sample := range bridge {
		bridgeCPU, bridgeMemory = append(bridgeCPU, sample.CPUCores),
			append(bridgeMemory, float64(sample.RSSBytes)/(1<<20))
	}
	value.EndpointCPUMean, value.EndpointCPUP95 = finalMean(endpointCPU), finalPercentile(endpointCPU, .95)
	value.EndpointRSSP95MiB = finalPercentile(endpointRSS, .95)
	value.BridgeCPUMean, value.BridgeCPUP95 = finalMean(bridgeCPU), finalPercentile(bridgeCPU, .95)
	value.BridgeMemoryP95MiB = finalPercentile(bridgeMemory, .95)
}

func finalTreeReserve(value finalResourceObservation, cpuMax, memoryMaxMiB float64) float64 {
	return min(100*(1-value.BridgeCPUP95/cpuMax), 100*(1-value.BridgeMemoryP95MiB/memoryMaxMiB))
}

func finalCarrierReserve(files []finalRawTelemetry, role string) (float64, bool) {
	return finalCarrierReserveAt(files, role, 100)
}

func finalCarrierReserveAt(files []finalRawTelemetry, role string, linkMbit float64) (float64, bool) {
	for _, file := range files {
		if file.Role != role || file.Kind != "carrier.jsonl" {
			continue
		}
		samples, ok := decodeFinalCarrierStream(file.Data)
		if !ok || len(samples) < 3 {
			return 0, false
		}
		rates := make([]float64, 0, len(samples)-2)
		for index := 1; index < len(samples)-1; index++ {
			gap := samples[index].OffsetMillis - samples[index-1].OffsetMillis
			if gap == 0 {
				return 0, false
			}
			rates = append(rates, float64(samples[index].Bytes-samples[index-1].Bytes)*8/
				(float64(gap)/1_000)/1e6)
		}
		return 100 * (1 - finalPercentile(rates, .95)/linkMbit), true
	}
	return 0, false
}

func finalResourceSamples(files []finalRawTelemetry, role string) ([]finalResourceSample, bool) {
	for _, file := range files {
		if file.Role != role || file.Kind != "resource.jsonl" {
			continue
		}
		var samples []finalResourceSample
		if !validFinalResourceStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &samples) {
			return nil, false
		}
		return samples, true
	}
	return nil, false
}

func finalPostCleanup(samples []finalResourceSample) bool {
	for _, sample := range samples {
		if sample.Boundary == "post-cleanup" {
			return true
		}
	}
	return false
}

func finalSustainedResourceInterval(samples []finalResourceSample, started, finished uint64) ([]finalResourceSample, bool) {
	periodic := make([]finalResourceSample, 0, len(samples))
	active := make([]finalResourceSample, 0, len(samples))
	for _, sample := range samples {
		if sample.Boundary == "" {
			periodic = append(periodic, sample)
			if sample.OffsetMillis >= started && sample.OffsetMillis < finished {
				active = append(active, sample)
			}
		}
	}
	complete := len(periodic) >= 600 && len(active) >= 600 && started < finished && finished-started >= 10*60*1_000 &&
		periodic[0].OffsetMillis <= started && periodic[len(periodic)-1].OffsetMillis >= finished &&
		finalPostCleanup(samples)
	return active, complete
}

func finalRoleResourceAggregate(endpoint, bridge []finalResourceSample) finalResourceObservation {
	value := finalResourceObservation{}
	mergeFinalHelperSamples(&value, bridge)
	mergeFinalAdapterSamples(&value, endpoint)
	value.EvidenceProjectedPC = float64(value.EvidenceBytes) * 100 / (16 << 20)
	return value
}

func mergeFinalHelperSamples(value *finalResourceObservation, samples []finalResourceSample) {
	rss := make([]float64, 0, len(samples))
	var fdPeak, socketPeak, threadPeak uint16
	for _, sample := range samples {
		rss = append(rss, float64(sample.RSSBytes)/(1<<20))
		fdPeak, socketPeak, threadPeak = max(fdPeak, sample.FDs), max(socketPeak, sample.Sockets),
			max(threadPeak, sample.Threads)
		value.DurableStateBytes = max(value.DurableStateBytes, uint32(sample.StateBytes))
		value.DurableMembers = max(value.DurableMembers, sample.DurableMembers)
		value.DurableContacts = max(value.DurableContacts, sample.DurableContacts)
		value.DurableAttempts = max(value.DurableAttempts, sample.DurableAttempts)
		value.DurableRegimes = max(value.DurableRegimes, sample.DurableRegimes)
		value.EvidenceBytes = max(value.EvidenceBytes, sample.EvidenceBytes)
		value.Capabilities = max(value.Capabilities, sample.Capabilities)
		value.Descendants = max(value.Descendants, finalUnexpectedProcesses(sample.Processes))
		if sample.SwapBytes != 0 {
			value.SwapEvents++
		}
		value.OOMEvents = uint16(max(uint64(value.OOMEvents), sample.EmergencyEvents))
	}
	value.HelperRSSP95MiB = finalPercentile(rss, .95)
	value.HelperFDPeak = fdPeak
	value.HelperSocketPeak = socketPeak
	value.ThreadsPeak = threadPeak
}

func mergeFinalAdapterSamples(value *finalResourceObservation, samples []finalResourceSample) {
	rss := make([]float64, 0, len(samples))
	for _, sample := range samples {
		rss = append(rss, float64(sample.RSSBytes)/(1<<20))
		value.AdapterFDPeak = max(value.AdapterFDPeak, sample.FDs)
		value.AdapterSocketPeak = max(value.AdapterSocketPeak, sample.Sockets)
		value.AdapterStateBytes = max(value.AdapterStateBytes, uint32(sample.StateBytes))
		value.AdapterStateEntries = max(value.AdapterStateEntries, sample.StateEntries)
		value.Capabilities = max(value.Capabilities, sample.Capabilities)
		value.Descendants = max(value.Descendants, finalUnexpectedProcesses(sample.Processes))
	}
	value.AdapterRSSP95MiB = finalPercentile(rss, .95)
}

func finalUnexpectedProcesses(processes uint16) uint16 {
	if processes > 2 {
		return processes - 2
	}
	return 0
}

func finalPercentile(values []float64, fraction float64) float64 {
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	if len(ordered) == 0 {
		return 0
	}
	index := int(math.Ceil(fraction*float64(len(ordered)))) - 1
	return ordered[max(index, 0)]
}
