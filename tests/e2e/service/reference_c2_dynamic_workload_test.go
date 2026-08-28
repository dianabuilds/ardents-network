package service_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestReferenceC2CarriesConfiguredPacedDynamicWorkload(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, dynamicWorkload: shortReferenceC2DynamicWorkload(3)})
}

func TestReferenceC2LosesPublisherApplicationAfterConfiguredWarmup(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherApplicationReset,
		dynamicWorkload: shortReferenceC2DynamicWorkload(2)})
}

func TestReferenceC2LosesPublisherEndpointAfterConfiguredWarmup(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, publisherTerminal: referenceC2PublisherEndpointStop,
		dynamicWorkload: shortReferenceC2DynamicWorkload(2)})
}

type referenceC2DynamicWorkload struct {
	Cycles                    uint32
	IntervalMilliseconds      uint32
	CycleDeadlineMilliseconds uint32
	NoFallbackEvery           uint32
	BytesEachDirection        uint32
}

func shortReferenceC2DynamicWorkload(cycles uint32) referenceC2DynamicWorkload {
	return referenceC2DynamicWorkload{Cycles: cycles, IntervalMilliseconds: 50, CycleDeadlineMilliseconds: 1_000,
		NoFallbackEvery: 1, BytesEachDirection: 1 << 20}
}

func (workload referenceC2DynamicWorkload) configured() bool { return workload.Cycles != 0 }

func (workload referenceC2DynamicWorkload) addTo(fixture map[string]any) {
	if workload.configured() {
		fixture["DynamicWorkload"] = workload
	}
}

func (workload referenceC2DynamicWorkload) timeBudget(minimum time.Duration) time.Duration {
	if !workload.configured() {
		return minimum
	}
	required := time.Duration(workload.Cycles)*time.Duration(workload.IntervalMilliseconds)*time.Millisecond +
		time.Duration(workload.CycleDeadlineMilliseconds)*time.Millisecond + 30*time.Second
	required = ((required + time.Second - 1) / time.Second) * time.Second
	return max(minimum, required)
}

func assertReferenceC2PublisherApplicationCompletion(t *testing.T, scenario referenceC2Scenario, applicationResult commandResult,
	processes map[string]commandResult,
) {
	t.Helper()
	switch scenario.publisherTerminal {
	case referenceC2PublisherApplicationReset:
		expected := "simulated Publisher Application crash after partial response"
		if scenario.dynamicWorkload.configured() {
			expected = "simulated Publisher Application crash after configured warmup"
		}
		if applicationResult.err == nil || !strings.Contains(string(applicationResult.output), expected) {
			t.Fatalf("Publisher Application crash result = %v\n%s", applicationResult.err, applicationResult.output)
		}
	case referenceC2PublisherEndpointStop:
		if applicationResult.err == nil || !strings.Contains(string(applicationResult.output), "simulated Publisher Endpoint crash closed the local Application handoff") {
			t.Fatalf("Publisher Endpoint crash Application result = %v\n%s", applicationResult.err, applicationResult.output)
		}
	default:
		processes["publisher-app"] = applicationResult
	}
}

func assertReferenceC2DynamicWorkloadResult(t *testing.T, scenario referenceC2Scenario, process commandResult) {
	t.Helper()
	if !scenario.dynamicWorkload.configured() {
		return
	}
	line := strings.TrimSpace(string(process.output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	var observed struct {
		Workload struct {
			InstrumentationBoundary                                        string
			ExpectedCycles, CompletedCycles, PeriodicNoFallbackProbeRounds uint32
			ProxyTCPDialCount, RejectedProxyRedials                        uint32
			TerminalNoFallback                                             bool
			MinimumCycleLatencyMicros, P50CycleLatencyMicros               int64
			P95CycleLatencyMicros, P99CycleLatencyMicros                   int64
			MaximumCycleLatencyMicros, MeanCycleLatencyMicros              int64
			MaximumStartLagMicros, TerminalLatencyMicros                   int64
		}
		Runtime *endpointapi.RuntimeResult
	}
	if err := json.Unmarshal([]byte(line), &observed); err != nil {
		t.Fatalf("decode configured dynamic workload result: %v\n%s", err, process.output)
	}
	expectedProbes := scenario.dynamicWorkload.Cycles / scenario.dynamicWorkload.NoFallbackEvery
	workload := observed.Workload
	if workload.InstrumentationBoundary != "direct-module-runtime; command IPC and Route-attachment counters are not applicable" ||
		workload.ExpectedCycles != scenario.dynamicWorkload.Cycles || workload.CompletedCycles != scenario.dynamicWorkload.Cycles ||
		workload.PeriodicNoFallbackProbeRounds != expectedProbes || workload.MinimumCycleLatencyMicros <= 0 ||
		workload.ProxyTCPDialCount != 1 ||
		workload.MeanCycleLatencyMicros <= 0 || workload.MinimumCycleLatencyMicros > workload.P50CycleLatencyMicros ||
		workload.P50CycleLatencyMicros > workload.P95CycleLatencyMicros || workload.P95CycleLatencyMicros > workload.P99CycleLatencyMicros ||
		workload.P99CycleLatencyMicros > workload.MaximumCycleLatencyMicros ||
		workload.MaximumCycleLatencyMicros > int64(scenario.dynamicWorkload.CycleDeadlineMilliseconds)*1_000 ||
		workload.MaximumStartLagMicros >= int64(scenario.dynamicWorkload.IntervalMilliseconds)*1_000 {
		t.Fatalf("configured dynamic workload result = %+v", workload)
	}
	terminal := scenario.publisherTerminal == referenceC2PublisherApplicationReset || scenario.publisherTerminal == referenceC2PublisherEndpointStop || scenario.transitFault != ""
	if workload.TerminalNoFallback != terminal || terminal && (workload.TerminalLatencyMicros <= 0 || workload.TerminalLatencyMicros > 15_000_000 || workload.RejectedProxyRedials == 0) ||
		!terminal && workload.RejectedProxyRedials != 0 {
		t.Fatalf("configured dynamic workload terminal result = %+v", workload)
	}
	assertReferenceC2EndpointRuntime(t, scenario, "user", observed.Runtime)
}

func assertReferenceC2PublisherDynamicRuntime(t *testing.T, scenario referenceC2Scenario, process commandResult) {
	t.Helper()
	if !scenario.dynamicWorkload.configured() {
		return
	}
	line := strings.TrimSpace(string(process.output))
	if index := strings.LastIndex(line, "\n"); index >= 0 {
		line = line[index+1:]
	}
	var observed struct{ Runtime *endpointapi.RuntimeResult }
	if err := json.Unmarshal([]byte(line), &observed); err != nil {
		t.Fatalf("decode configured Publisher runtime: %v\n%s", err, process.output)
	}
	assertReferenceC2EndpointRuntime(t, scenario, "publisher", observed.Runtime)
}

func assertReferenceC2EndpointRuntime(t *testing.T, scenario referenceC2Scenario, role string, runtimeResult *endpointapi.RuntimeResult) {
	t.Helper()
	expectedClass := "clean service connection close"
	if scenario.publisherTerminal == referenceC2PublisherApplicationReset || scenario.publisherTerminal == referenceC2PublisherEndpointStop || scenario.transitFault != "" {
		expectedClass = "abrupt connection loss"
	}
	if runtimeResult == nil || runtimeResult.Class == "" || runtimeResult.Generation != 1 || runtimeResult.RouteGeneration != 1 ||
		runtimeResult.RecoveryCount != 0 || runtimeResult.AuthenticatedTarget == [32]byte{} ||
		runtimeResult.AcceptedBytes == 0 || runtimeResult.AcceptedBytes > scenario.dynamicWorkload.BytesEachDirection ||
		runtimeResult.ReceivedBytes == 0 || runtimeResult.ReceivedBytes > scenario.dynamicWorkload.BytesEachDirection ||
		runtimeResult.AcknowledgedBytes > runtimeResult.AcceptedBytes || runtimeResult.Class != expectedClass {
		t.Fatalf("configured dynamic %s Endpoint runtime = %+v", role, runtimeResult)
	}
}
