//go:build live

package network_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type finalResourceEvidence struct {
	EndpointCPUMean     float64  `json:"endpoint_cpu_mean_cores"`
	EndpointCPUP95      float64  `json:"endpoint_cpu_p95_cores"`
	EndpointRSSP95MiB   float64  `json:"endpoint_rss_p95_mib"`
	BridgeCPUMean       float64  `json:"bridge_cpu_mean_cores"`
	BridgeCPUP95        float64  `json:"bridge_cpu_p95_cores"`
	BridgeMemoryP95MiB  float64  `json:"bridge_memory_p95_mib"`
	HelperRSSP95MiB     float64  `json:"helper_rss_p95_mib"`
	ReservePercent      float64  `json:"reserve_percent"`
	HelperFDPeak        uint16   `json:"helper_fd_peak"`
	HelperSocketPeak    uint16   `json:"helper_socket_peak"`
	SwapEvents          uint16   `json:"swap_events"`
	OOMEvents           uint16   `json:"oom_events"`
	Samples             uint16   `json:"samples"`
	SamplesComplete     bool     `json:"samples_complete"`
	AdapterRSSP95MiB    float64  `json:"adapter_rss_p95_mib"`
	AdapterFDPeak       uint16   `json:"adapter_fd_peak"`
	AdapterSocketPeak   uint16   `json:"adapter_socket_peak"`
	AdapterStateBytes   uint32   `json:"adapter_state_bytes"`
	AdapterStateEntries uint16   `json:"adapter_state_entries"`
	ThreadsPeak         uint16   `json:"threads_peak"`
	GoroutinesPeak      uint16   `json:"goroutines_peak"`
	TimersPeak          uint16   `json:"timers_peak"`
	QueueItemsPeak      uint16   `json:"queue_items_peak"`
	QueueBytesPeak      uint32   `json:"queue_bytes_peak"`
	DurableMembers      uint16   `json:"durable_members"`
	DurableContacts     uint16   `json:"durable_contacts"`
	DurableAttempts     uint16   `json:"durable_attempts"`
	DurableRegimes      uint16   `json:"durable_regimes"`
	DurableStateBytes   uint32   `json:"durable_state_bytes"`
	EvidenceBytes       uint64   `json:"evidence_bytes"`
	EvidenceProjectedPC float64  `json:"evidence_projected_percent"`
	EvidenceDropped     uint16   `json:"evidence_dropped"`
	Descendants         uint16   `json:"descendants"`
	Capabilities        uint16   `json:"capabilities"`
	Collected           []string `json:"collected"`
}

type finalSustainedRunEvidence struct {
	StartedOffsetMillis  uint64                `json:"started_offset_millis"`
	FinishedOffsetMillis uint64                `json:"finished_offset_millis"`
	WindowEndsMillis     []uint64              `json:"window_ends_millis"`
	WindowsMbit          []float64             `json:"windows_mbit"`
	Resources            finalResourceEvidence `json:"resources"`
	Complete             bool                  `json:"complete"`
	DeliveredBytes       uint64                `json:"delivered_bytes"`
	Digest               string                `json:"digest"`
}

type finalRuntimeSample struct {
	at                                             time.Time
	endpointCPU, endpointRSS, bridgeCPU, bridgeRSS float64
	emergency                                      uint64
}

type finalProgressPoint struct {
	at       time.Time
	received uint32
}

func monitorFinalSustained(t *testing.T, ctx context.Context, compose composeCall, receiver string,
	logical uint32, started, timeline time.Time,
) finalSustainedRunEvidence {
	t.Helper()
	services := []string{"endpoint", "client-service", "client-app", "bridge", "publisher",
		"publisher-service", "publisher-app"}
	identities := make(map[string]string, len(services))
	for _, service := range services {
		output, err := compose(ctx, "ps", "-q", service)
		if err != nil || strings.TrimSpace(string(output)) == "" {
			t.Fatalf("resolve final sustained %s: %v\n%s", service, err, output)
		}
		identities[service] = strings.TrimSpace(string(output))
	}
	result := finalSustainedRunEvidence{StartedOffsetMillis: uint64(started.Sub(timeline).Milliseconds())}
	previousBytes := float64(0)
	current := uint32(0)
	boundary := started.Add(time.Minute)
	samples := []finalRuntimeSample{readFinalRuntimeSample(t, ctx, compose, identities)}
	before := finalProgressPoint{at: started}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for len(result.WindowsMbit) < 10 || current < logical {
		select {
		case <-ctx.Done():
			t.Fatalf("monitor final sustained run: %v", ctx.Err())
		case <-ticker.C:
			point := finalProgressPointAt(t, ctx, compose, receiver)
			samples = append(samples, readFinalRuntimeSample(t, ctx, compose, identities))
			current = point.received
			if !point.at.After(boundary) {
				before = point
				continue
			}
			if len(result.WindowsMbit) < 10 {
				atBoundary := interpolateFinalProgress(t, before, point, boundary)
				result.WindowsMbit = append(result.WindowsMbit,
					(atBoundary-previousBytes)*8/time.Minute.Seconds()/1e6)
				result.WindowEndsMillis = append(result.WindowEndsMillis,
					uint64(boundary.Sub(timeline).Milliseconds()))
				previousBytes, before, boundary = atBoundary, point, boundary.Add(time.Minute)
			}
		}
	}
	if previousBytes > float64(logical) {
		t.Fatalf("final sustained progress exceeded workload: %.0f > %d", previousBytes, logical)
	}
	result.Resources = summarizeFinalResources(t, samples)
	result.FinishedOffsetMillis = uint64(time.Since(timeline).Milliseconds())
	result.Complete = len(result.WindowsMbit) == 10
	return result
}

func finalProgressPointAt(t *testing.T, ctx context.Context, compose composeCall, service string) finalProgressPoint {
	t.Helper()
	output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "--tail", "256", service)
	if err != nil {
		t.Fatalf("read final sustained progress: %v\n%s", err, output)
	}
	return finalProgressPoint{at: time.Now(), received: latestLiveProgress(output)}
}

func interpolateFinalProgress(t *testing.T, before, after finalProgressPoint, boundary time.Time) float64 {
	t.Helper()
	if before.at.IsZero() || !before.at.Before(boundary) || !after.at.After(boundary) ||
		after.received < before.received {
		t.Fatal("final sustained progress does not bracket an exact window boundary")
	}
	fraction := float64(boundary.Sub(before.at)) / float64(after.at.Sub(before.at))
	return float64(before.received) + fraction*float64(after.received-before.received)
}

func readFinalRuntimeSample(t *testing.T, ctx context.Context, compose composeCall,
	identities map[string]string,
) finalRuntimeSample {
	t.Helper()
	arguments := []string{"stats", "--no-stream", "--format", "{{.ID}}|{{.CPUPerc}}|{{.MemUsage}}"}
	for _, service := range []string{"endpoint", "client-service", "client-app", "bridge", "publisher",
		"publisher-service", "publisher-app"} {
		arguments = append(arguments, identities[service])
	}
	output, err := dockerOutput(ctx, arguments...)
	if err != nil {
		t.Fatalf("sample final sustained resources: %v\n%s", err, output)
	}
	type value struct{ cpu, rss float64 }
	values := make(map[string]value, len(identities))
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 3 {
			t.Fatalf("invalid final resource sample %q", line)
		}
		cpu, cpuErr := strconv.ParseFloat(strings.TrimSuffix(fields[1], "%"), 64)
		rss, rssErr := liveQuantity(strings.TrimSpace(strings.Split(fields[2], "/")[0]))
		if cpuErr != nil || rssErr != nil {
			t.Fatalf("invalid final resource values %q", line)
		}
		for service, identity := range identities {
			if strings.HasPrefix(identity, fields[0]) || strings.HasPrefix(fields[0], identity) {
				values[service] = value{cpu / 100, rss}
			}
		}
	}
	if len(values) != len(identities) {
		t.Fatalf("final resource sample omitted a process tree: %d/%d", len(values), len(identities))
	}
	endpointCPU, endpointRSS := 0.0, 0.0
	for _, service := range []string{"endpoint", "client-service", "client-app"} {
		endpointCPU, endpointRSS = endpointCPU+values[service].cpu, endpointRSS+values[service].rss
	}
	bridge := values["bridge"]
	return finalRuntimeSample{at: time.Now(), endpointCPU: endpointCPU, endpointRSS: endpointRSS,
		bridgeCPU: bridge.cpu, bridgeRSS: bridge.rss, emergency: latestBridgeEmergency(t, ctx, compose)}
}

func latestBridgeEmergency(t *testing.T, ctx context.Context, compose composeCall) uint64 {
	t.Helper()
	output, err := compose(ctx, "logs", "--no-color", "--no-log-prefix", "--tail", "16", "bridge")
	if err != nil {
		return math.MaxUint64
	}
	var latest uint64
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		var event struct {
			Kind   string           `json:"kind"`
			Sample *resource.Sample `json:"resource"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &event) == nil && event.Kind == "resource-sample" && event.Sample != nil {
			latest = event.Sample.EmergencyEvents
		}
	}
	return latest
}

func summarizeFinalResources(t *testing.T, samples []finalRuntimeSample) finalResourceEvidence {
	t.Helper()
	if len(samples) < 600 || !completeFinalRuntimeCadence(samples) {
		t.Fatalf("final sustained resource stream has only %d samples", len(samples))
	}
	endpointCPU, endpointRSS, bridgeCPU, bridgeRSS := make([]float64, 0, len(samples)),
		make([]float64, 0, len(samples)), make([]float64, 0, len(samples)), make([]float64, 0, len(samples))
	var emergency uint64
	for _, sample := range samples {
		endpointCPU, endpointRSS = append(endpointCPU, sample.endpointCPU), append(endpointRSS, sample.endpointRSS)
		bridgeCPU, bridgeRSS = append(bridgeCPU, sample.bridgeCPU), append(bridgeRSS, sample.bridgeRSS)
		emergency = max(emergency, sample.emergency)
	}
	cpuReserve := 100 * (1 - percentile(bridgeCPU, .95)/1.6)
	memoryReserve := 100 * (1 - percentile(bridgeRSS, .95)/(1280<<20))
	return finalResourceEvidence{EndpointCPUMean: mean(endpointCPU), EndpointCPUP95: percentile(endpointCPU, .95),
		EndpointRSSP95MiB: percentile(endpointRSS, .95) / (1 << 20), BridgeCPUMean: mean(bridgeCPU),
		BridgeCPUP95: percentile(bridgeCPU, .95), BridgeMemoryP95MiB: percentile(bridgeRSS, .95) / (1 << 20),
		ReservePercent: math.Min(cpuReserve, memoryReserve), OOMEvents: uint16(emergency),
		Samples: uint16(len(samples)), SamplesComplete: true}
}

func completeFinalRuntimeCadence(samples []finalRuntimeSample) bool {
	for index := 1; index < len(samples); index++ {
		gap := samples[index].at.Sub(samples[index-1].at)
		if gap < 750*time.Millisecond || gap > 1250*time.Millisecond {
			return false
		}
	}
	return true
}

func finalResourceObservations() []string {
	return []string{"endpoint-cpu", "endpoint-rss", "adapter-rss", "adapter-fds", "adapter-sockets",
		"adapter-state", "bridge-cpu", "bridge-memory", "helper-rss", "helper-fds", "helper-sockets",
		"swap-oom", "threads", "goroutines", "timers", "queues", "durable-state", "evidence",
		"traffic", "descendants", "capabilities", "reserve"}
}

func assertFiniteRatio(t *testing.T, name string, value float64) {
	t.Helper()
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		t.Fatalf("%s carrier ratio is invalid: %v", name, value)
	}
	if value > 1.5 {
		t.Fatalf("%s carrier ratio exceeds 1.5: %.3f", name, value)
	}
}
