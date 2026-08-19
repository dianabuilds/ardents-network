package blockedverify

import "reflect"

func finalMean(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	if len(values) == 0 {
		return 0
	}
	return total / float64(len(values))
}

func reproducesFinalCapacityResources(files []finalRawTelemetry, published finalResourceObservation,
	strong bool,
) bool {
	endpointTrees := finalCapacityTrees(files, "endpoint")
	bridgeTrees := finalCapacityTrees(files, "bridge")
	publisherTrees := finalCapacityTrees(files, "publisher")
	wantEndpoints := 4
	cpuMax, memoryMax, link := 1.6, 1280.0, 100.0
	if strong {
		wantEndpoints, cpuMax, memoryMax, link = 16, 6.4, 5120, 400
	}
	if len(endpointTrees) != wantEndpoints || len(bridgeTrees) != 1 || len(publisherTrees) != 1 {
		return false
	}
	derived := finalResourceObservation{Samples: uint16(wantEndpoints + 2), SamplesComplete: true}
	mergeFinalTreeResources(&derived, endpointTrees, bridgeTrees)
	var endpointProcess []finalResourceSample
	for _, file := range files {
		if file.Kind != "resource.jsonl" {
			continue
		}
		var samples []finalResourceSample
		if !validFinalResourceStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &samples) {
			return false
		}
		switch file.Role {
		case "endpoint":
			endpointProcess = append(endpointProcess, samples...)
		case "bridge":
			mergeFinalHelperSamples(&derived, samples)
		case "publisher":
			if len(samples) == 0 {
				return false
			}
		}
	}
	mergeFinalAdapterSamples(&derived, endpointProcess)
	derived.EvidenceProjectedPC = float64(derived.EvidenceBytes) * 100 / (16 << 20)
	if !mergeFinalRuntimeResources(files, &derived) {
		return false
	}
	derived.ReservePercent = finalTreeReserve(derived, cpuMax, memoryMax)
	if carrier, ok := finalCarrierReserveAt(files, "bridge", link); ok {
		derived.ReservePercent = min(derived.ReservePercent, carrier)
	} else {
		return false
	}
	derived.Collected = requiredResourceObservations()
	return sameFinalCapacityResources(published, derived)
}

func finalCapacityTrees(files []finalRawTelemetry, role string) []finalTreeSample {
	var result []finalTreeSample
	for _, file := range files {
		if file.Role != role || file.Kind != "tree.jsonl" {
			continue
		}
		var samples []finalTreeSample
		if !validFinalTreeStream(file.Data) || !decodeFinalTelemetryLines(file.Data, &samples) || len(samples) != 1 {
			return nil
		}
		result = append(result, samples[0])
	}
	return result
}

func sameFinalCapacityResources(got, want finalResourceObservation) bool {
	return got.EndpointCPUMean == want.EndpointCPUMean && got.EndpointCPUP95 == want.EndpointCPUP95 &&
		got.EndpointRSSP95MiB == want.EndpointRSSP95MiB && got.BridgeCPUMean == want.BridgeCPUMean &&
		got.BridgeCPUP95 == want.BridgeCPUP95 && got.BridgeMemoryP95MiB == want.BridgeMemoryP95MiB &&
		got.HelperRSSP95MiB == want.HelperRSSP95MiB && got.HelperFDPeak == want.HelperFDPeak &&
		got.HelperSocketPeak == want.HelperSocketPeak && got.SwapEvents == want.SwapEvents &&
		got.OOMEvents == want.OOMEvents && got.ReservePercent == want.ReservePercent &&
		got.Samples == want.Samples && got.SamplesComplete == want.SamplesComplete &&
		got.AdapterRSSP95MiB == want.AdapterRSSP95MiB && got.AdapterFDPeak == want.AdapterFDPeak &&
		got.AdapterSocketPeak == want.AdapterSocketPeak && got.AdapterStateBytes == want.AdapterStateBytes &&
		got.AdapterStateEntries == want.AdapterStateEntries && got.ThreadsPeak == want.ThreadsPeak &&
		got.GoroutinesPeak == want.GoroutinesPeak && got.TimersPeak == want.TimersPeak &&
		got.QueueItemsPeak == want.QueueItemsPeak && got.QueueBytesPeak == want.QueueBytesPeak &&
		got.DurableMembers == want.DurableMembers && got.DurableContacts == want.DurableContacts &&
		got.DurableAttempts == want.DurableAttempts && got.DurableRegimes == want.DurableRegimes &&
		got.DurableStateBytes == want.DurableStateBytes && got.EvidenceBytes == want.EvidenceBytes &&
		got.EvidenceProjectedPC == want.EvidenceProjectedPC && got.Descendants == want.Descendants &&
		got.EvidenceDropped == want.EvidenceDropped && got.Capabilities == want.Capabilities &&
		reflect.DeepEqual(got.Collected, want.Collected)
}
