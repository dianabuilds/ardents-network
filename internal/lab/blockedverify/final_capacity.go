package blockedverify

import (
	"fmt"
	"reflect"
)

func requiredResourceObservations() []string {
	return []string{"endpoint-cpu", "endpoint-rss", "adapter-rss", "adapter-fds", "adapter-sockets",
		"adapter-state", "bridge-cpu", "bridge-memory", "helper-rss", "helper-fds", "helper-sockets",
		"swap-oom", "threads", "goroutines", "timers", "queues", "durable-state", "evidence",
		"traffic", "descendants", "capabilities", "reserve"}
}

func verifyFinalCapacity(values []finalCapacityBatch) ([]string, []string) {
	if len(values) != 10 {
		return []string{"capacity batch set is incomplete"}, nil
	}
	var invalid, failures []string
	for index, value := range values {
		profile := "h3-s5-b1-v1"
		offered, accepted, socketLimit := uint16(5), uint16(4), uint16(32)
		if index >= 5 {
			profile, offered, accepted, socketLimit = "h3-s5-b1-v1-strong", 17, 16, 128
		}
		batch := uint16(index % 5)
		if value.Profile != profile || value.Batch != batch || value.Offered != offered ||
			value.Accepted != accepted || value.Refused != 1 || value.Terminal == "" {
			invalid = append(invalid, "capacity batch identity or corpus differs from R-037")
			continue
		}
		if resourceInvalid := validateResourceShape(value.Resources); resourceInvalid != "" {
			invalid = append(invalid, resourceInvalid)
			continue
		}
		if value.Terminal != "complete" || value.MaximumRefusalMillis > 1_000 ||
			!value.EstablishedProgress || !value.Cleanup || !value.SecurityExact ||
			value.ReservePercent < 20 || value.ResponseP95Millis > 8_000 {
			failures = append(failures, fmt.Sprintf("capacity batch %s/%d failed", profile, batch))
		}
		if reasons := resourceGateFailures(value.Resources, profile == "h3-s5-b1-v1-strong", socketLimit); len(reasons) > 0 {
			failures = append(failures, reasons...)
		}
	}
	return invalid, failures
}

func validateResourceShape(value finalResourceObservation) string {
	if !reflect.DeepEqual(value.Collected, requiredResourceObservations()) || value.Samples == 0 ||
		!value.SamplesComplete || value.EndpointCPUMean < 0 || value.EndpointCPUP95 < 0 ||
		value.EndpointRSSP95MiB < 0 || value.BridgeCPUMean < 0 || value.BridgeCPUP95 < 0 ||
		value.BridgeMemoryP95MiB < 0 || value.HelperRSSP95MiB < 0 || value.AdapterRSSP95MiB < 0 ||
		value.EvidenceProjectedPC < 0 || value.ReservePercent < 0 {
		return "resource observation is missing, non-finite, or invalid"
	}
	for _, number := range []float64{value.EndpointCPUMean, value.EndpointCPUP95, value.EndpointRSSP95MiB,
		value.BridgeCPUMean, value.BridgeCPUP95, value.BridgeMemoryP95MiB, value.HelperRSSP95MiB,
		value.AdapterRSSP95MiB, value.EvidenceProjectedPC, value.ReservePercent} {
		if number != number {
			return "resource observation is missing, non-finite, or invalid"
		}
	}
	return ""
}

func resourceGateFailures(value finalResourceObservation, strong bool, socketLimit uint16) []string {
	bridgeMean, bridgeP95, memoryP95, helperRSS, fdLimit := 1.12, 1.28, 896.0, 128.0, uint16(64)
	if strong {
		bridgeMean, bridgeP95, memoryP95, helperRSS, fdLimit = 4.48, 5.12, 3_584, 512, 256
	}
	var reasons []string
	if value.EndpointCPUMean > .5 || value.EndpointCPUP95 > 1 || value.EndpointRSSP95MiB > 512 {
		reasons = append(reasons, "endpoint resource gate failed")
	}
	if value.AdapterRSSP95MiB > 64 || value.AdapterFDPeak > 16 || value.AdapterSocketPeak > 4 ||
		value.AdapterStateBytes > 1<<20 || value.AdapterStateEntries > 32 || value.QueueBytesPeak > 256<<10 ||
		value.DurableMembers > 4 || value.DurableContacts > 4 || value.DurableAttempts > 1 ||
		value.DurableRegimes > 1 || value.DurableStateBytes > 256<<10 || value.EvidenceBytes > 16<<20 ||
		value.EvidenceProjectedPC > 80 || value.EvidenceDropped != 0 || value.Descendants != 0 ||
		value.Capabilities != 0 {
		reasons = append(reasons, "adapter, queue, durable-state, or evidence resource gate failed")
	}
	if value.BridgeCPUMean > bridgeMean || value.BridgeCPUP95 > bridgeP95 ||
		value.BridgeMemoryP95MiB > memoryP95 || value.HelperRSSP95MiB > helperRSS ||
		value.HelperFDPeak > fdLimit || value.HelperSocketPeak > socketLimit || value.SwapEvents != 0 ||
		value.OOMEvents != 0 || value.ReservePercent < 20 {
		reasons = append(reasons, "bridge resource gate failed")
	}
	return reasons
}
