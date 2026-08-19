//go:build live

package network_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func finalComposeContainerIDs(t *testing.T, ctx context.Context, compose composeCall,
	services ...string,
) []string {
	t.Helper()
	var result []string
	for _, service := range services {
		output, err := compose(ctx, "ps", "-q", service)
		ids := strings.Fields(string(output))
		if err != nil || len(ids) == 0 {
			t.Fatalf("resolve final capacity %s tree: %v\n%s", service, err, output)
		}
		result = append(result, ids...)
	}
	return result
}

func finalContainerDuration(t *testing.T, ctx context.Context, name string, started time.Time) time.Duration {
	t.Helper()
	output, err := dockerOutput(ctx, "inspect", "--format", "{{.State.FinishedAt}}", name)
	finished, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(output)))
	if err != nil || parseErr != nil || finished.Before(started) {
		t.Fatalf("inspect final capacity duration %s: %v %v\n%s", name, err, parseErr, output)
	}
	return finished.Sub(started)
}

func finalCapacityMeasurement(t *testing.T, root, profile string, batch int, admission blockedAdmissionResult,
	endpoints []finalTreeSample, bridge, publisher finalTreeSample, durations []time.Duration,
	runtimeSamples []resource.Sample,
) finalWorkerCapacity {
	t.Helper()
	writeFinalBridgeRuntime(t, root, runtimeSamples)
	resources := finalResourceEvidence{Samples: uint16(len(endpoints) + 2), SamplesComplete: true,
		ReservePercent: 100, Collected: finalResourceObservations()}
	endpointCPU, endpointRSS := make([]float64, 0, len(endpoints)), make([]float64, 0, len(endpoints))
	var adapterRSS, helperRSS []float64
	for index, sample := range endpoints {
		endpointCPU = append(endpointCPU, sample.CPUCores)
		endpointRSS = append(endpointRSS, float64(sample.RSSBytes)/(1<<20))
		mergeFinalCapacityProcess(t, &resources,
			filepath.Join(root, "sync", capacityRole(index), "resource.jsonl"), false, &adapterRSS)
	}
	resources.EndpointCPUMean, resources.EndpointCPUP95 = mean(endpointCPU), percentile(endpointCPU, .95)
	resources.EndpointRSSP95MiB = percentile(endpointRSS, .95)
	resources.BridgeCPUMean, resources.BridgeCPUP95 = bridge.CPUCores, bridge.CPUCores
	resources.BridgeMemoryP95MiB = float64(bridge.RSSBytes) / (1 << 20)
	mergeFinalCapacityProcess(t, &resources, filepath.Join(root, "sync", "bridge", "resource.jsonl"), true, &helperRSS)
	resources.AdapterRSSP95MiB, resources.HelperRSSP95MiB = percentile(adapterRSS, .95), percentile(helperRSS, .95)
	if samples, err := readBlockedProcessSamples(filepath.Join(root, "sync", "publisher", "resource.jsonl")); err != nil || len(samples) == 0 || publisher.RSSBytes == 0 {
		t.Fatalf("publisher capacity resource stream is invalid: %v", err)
	}
	for _, sample := range runtimeSamples {
		resources.GoroutinesPeak = max(resources.GoroutinesPeak, uint16(sample.Goroutines))
		resources.TimersPeak = max(resources.TimersPeak, uint16(sample.Timers))
		resources.QueueItemsPeak = max(resources.QueueItemsPeak, uint16(sample.QueueItems))
		resources.QueueBytesPeak = max(resources.QueueBytesPeak, uint32(sample.QueueBytes))
	}
	cpuMax, memoryMax := 1.6, float64(1_280<<20)
	if profile == "h3-s5-b1-v1-strong" {
		cpuMax, memoryMax = 6.4, float64(5_120<<20)
	}
	resources.ReservePercent = math.Min(100*(1-bridge.CPUCores/cpuMax),
		100*(1-float64(bridge.RSSBytes)/memoryMax))
	carrier, err := readFinalCapacityCarrier(filepath.Join(root, "sync", "bridge", "carrier.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	link := 100.0
	if profile == "h3-s5-b1-v1-strong" {
		link = 400
	}
	resources = mergeFinalCarrierReserveAt(resources, carrier, link)
	response := percentileDurations(durations, .95)
	return finalWorkerCapacity{Profile: profile, Terminal: "complete", Batch: uint16(batch),
		Offered: uint16(len(endpoints) + 1), Accepted: uint16(len(endpoints)), Refused: admission.Refused,
		MaximumRefusalMillis: admission.MaximumMillis, EstablishedProgress: true, Cleanup: true,
		SecurityExact: true, ReservePercent: resources.ReservePercent,
		ResponseP95Millis: uint32(response.Milliseconds()), Resources: resources}
}

func mergeFinalCapacityProcess(t *testing.T, value *finalResourceEvidence, path string, bridge bool,
	rss *[]float64,
) {
	t.Helper()
	samples, err := readBlockedProcessSamples(path)
	if err != nil || len(samples) == 0 {
		t.Fatalf("capacity process stream is invalid: %v", err)
	}
	for _, sample := range samples {
		*rss = append(*rss, float64(sample.RSSBytes)/(1<<20))
		if bridge {
			value.HelperFDPeak, value.HelperSocketPeak = max(value.HelperFDPeak, sample.FDs),
				max(value.HelperSocketPeak, sample.Sockets)
			value.ThreadsPeak = max(value.ThreadsPeak, sample.Threads)
			value.DurableMembers, value.DurableContacts = max(value.DurableMembers, sample.DurableMembers),
				max(value.DurableContacts, sample.DurableContacts)
			value.DurableAttempts, value.DurableRegimes = max(value.DurableAttempts, sample.DurableAttempts),
				max(value.DurableRegimes, sample.DurableRegimes)
			value.DurableStateBytes = max(value.DurableStateBytes, uint32(sample.StateBytes))
			value.EvidenceBytes = max(value.EvidenceBytes, sample.EvidenceBytes)
			if sample.SwapBytes != 0 {
				value.SwapEvents++
			}
			value.OOMEvents = uint16(max(uint64(value.OOMEvents), sample.EmergencyEvents))
		} else {
			value.AdapterFDPeak, value.AdapterSocketPeak = max(value.AdapterFDPeak, sample.FDs),
				max(value.AdapterSocketPeak, sample.Sockets)
			value.AdapterStateBytes = max(value.AdapterStateBytes, uint32(sample.StateBytes))
			value.AdapterStateEntries = max(value.AdapterStateEntries, sample.StateEntries)
		}
		value.Capabilities = max(value.Capabilities, sample.Capabilities)
		value.Descendants = max(value.Descendants, unexpectedProcesses(sample.Processes))
	}
	value.EvidenceProjectedPC = float64(value.EvidenceBytes) * 100 / (16 << 20)
}

func capacityRole(index int) string { return fmt.Sprintf("capacity-%02d", index) }

func percentileDurations(values []time.Duration, fraction float64) time.Duration {
	ordered := append([]time.Duration(nil), values...)
	slices.Sort(ordered)
	if len(ordered) == 0 {
		return 0
	}
	return ordered[int(math.Ceil(fraction*float64(len(ordered))))-1]
}

func readFinalCapacityCarrier(path string) ([]blockedCarrierSample, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	var result []blockedCarrierSample
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var value blockedCarrierSample
		if json.Unmarshal(scanner.Bytes(), &value) != nil || value.Schema != "ardents-h3-carrier-counter-v1" ||
			value.Ordinal != uint16(len(result)) {
			return nil, fmt.Errorf("capacity carrier stream is invalid")
		}
		result = append(result, value)
	}
	if err := scanner.Err(); err != nil || len(result) < 3 || result[0].Boundary != "before" ||
		result[len(result)-1].Boundary != "after" {
		return nil, fmt.Errorf("capacity carrier stream is incomplete: %w", err)
	}
	return result, nil
}
