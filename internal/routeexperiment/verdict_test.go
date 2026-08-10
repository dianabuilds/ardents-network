package routeexperiment

import (
	"testing"
	"time"
)

func TestNearestRank(t *testing.T) {
	t.Parallel()
	values := []time.Duration{9 * time.Second, time.Second, 5 * time.Second, 3 * time.Second}
	if got := nearestRank(values, 50); got != 3*time.Second {
		t.Fatalf("p50 = %s, want 3s", got)
	}
	if got := nearestRank(values, 95); got != 9*time.Second {
		t.Fatalf("p95 = %s, want 9s", got)
	}
}

func TestCandidateAdvancesOnlyWhenEveryGatePasses(t *testing.T) {
	t.Parallel()
	direct := passingCondition("direct", 20_000_000)
	candidate := passingCondition("c5-c2", 10_000_000)
	result := evaluateCandidate(direct, candidate, passingNegatives())
	if result.Decision != decisionAdvance || len(result.Failures) != 0 {
		t.Fatalf("decision = %q, failures = %v", result.Decision, result.Failures)
	}
}

func TestCandidateVerdictIsConjunctive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*conditionResult, *negativeResult)
	}{
		{"setup count", func(c *conditionResult, _ *negativeResult) {
			c.Setups[18].Passed = false
			c.Setups[19].Passed = false
		}},
		{"setup p95", func(c *conditionResult, _ *negativeResult) {
			c.Setups[18].Elapsed = 4 * time.Second
			c.Setups[19].Elapsed = 4 * time.Second
		}},
		{"failed setup counts as infinity", func(c *conditionResult, _ *negativeResult) {
			c.Setups[0].Passed = false
			c.Setups[1].Elapsed = 4 * time.Second
		}},
		{"upload goodput", func(c *conditionResult, _ *negativeResult) { c.Streams[0].BitsPerSecond = 9_999_999 }},
		{"endpoint rss", func(c *conditionResult, _ *negativeResult) { c.Resources[0].PeakRSSBytes = 512<<20 + 1 }},
		{"node rss", func(c *conditionResult, _ *negativeResult) { c.Resources[2].PeakRSSBytes = 256<<20 + 1 }},
		{"cpu", func(c *conditionResult, _ *negativeResult) { c.Resources[0].MeanCPUCores = 1 }},
		{"queue", func(c *conditionResult, _ *negativeResult) { c.Resources[0].QueueHighBytes = 256<<10 + 1 }},
		{"missing resource evidence", func(c *conditionResult, _ *negativeResult) { c.Resources = c.Resources[1:] }},
		{"disclosure", func(c *conditionResult, _ *negativeResult) { c.Disclosures = []string{"marker"} }},
		{"negative", func(_ *conditionResult, n *negativeResult) { n.Cases[0].Passed = false }},
		{"failure deadline", func(_ *conditionResult, n *negativeResult) { n.FailureElapsed = 15*time.Second + 1 }},
		{"cleanup", func(c *conditionResult, _ *negativeResult) { c.CleanupPassed = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			direct := passingCondition("direct", 20_000_000)
			candidate := passingCondition("c5-c2", 10_000_000)
			negatives := passingNegatives()
			test.mutate(&candidate, &negatives)
			result := evaluateCandidate(direct, candidate, negatives)
			if result.Decision == decisionAdvance || len(result.Failures) == 0 {
				t.Fatalf("invalid candidate advanced: %+v", result)
			}
		})
	}
}

func TestGoodputThresholdUsesHalfDirectUntilTenMegabits(t *testing.T) {
	t.Parallel()
	if got := goodputThreshold(12_000_000); got != 6_000_000 {
		t.Fatalf("threshold = %v", got)
	}
	if got := goodputThreshold(40_000_000); got != 10_000_000 {
		t.Fatalf("capped threshold = %v", got)
	}
}

func passingCondition(name string, goodput float64) conditionResult {
	setups := make([]setupSample, 20)
	for index := range setups {
		setups[index] = setupSample{Passed: true, Elapsed: time.Second}
	}
	streams := make([]streamSample, 6)
	for index := range streams {
		direction := directionUpload
		if index >= 3 {
			direction = directionDownload
		}
		streams[index] = streamSample{Direction: direction, Verified: true, BitsPerSecond: goodput}
	}
	return conditionResult{
		Name: name, Setups: setups, Streams: streams, CleanupPassed: true,
		Resources: []resourceSample{
			{Role: "user", Endpoint: true, PeakRSSBytes: 64 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "service", Endpoint: true, PeakRSSBytes: 64 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "user-entry", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "user-interior", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "rendezvous", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "service-interior", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "data-service-entry", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "introduction-forwarder", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "introduction-node", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "introduction-interior", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
			{Role: "introduction-entry", PeakRSSBytes: 32 << 20, MeanCPUCores: .5, QueueHighBytes: 16 << 10},
		},
	}
}

func passingNegatives() negativeResult {
	names := []string{"wrong-instance", "modified-record", "replay", "wrong-binding", "oversized-frame", "invalid-state", "rendezvous-process"}
	cases := make([]negativeCase, len(names))
	for index, name := range names {
		cases[index] = negativeCase{Name: name, Passed: true}
	}
	return negativeResult{Cases: cases, FailureElapsed: time.Second}
}
