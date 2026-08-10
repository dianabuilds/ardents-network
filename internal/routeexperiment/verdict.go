package routeexperiment

import (
	"slices"
	"time"
)

const (
	directionUpload   = "user-to-service"
	directionDownload = "service-to-user"
	decisionAdvance   = "advance"
	decisionRedesign  = "redesign"
	decisionStop      = "stop"
)

type setupSample struct {
	Passed  bool          `json:"passed"`
	Elapsed time.Duration `json:"elapsed_ns"`
}

type streamSample struct {
	Direction     string            `json:"direction"`
	Verified      bool              `json:"verified"`
	BitsPerSecond float64           `json:"bits_per_second"`
	LinkWireBytes map[string]uint64 `json:"link_wire_bytes,omitempty"`
}

type resourceSample struct {
	Role           string  `json:"role"`
	Endpoint       bool    `json:"endpoint"`
	PeakRSSBytes   uint64  `json:"peak_rss_bytes"`
	MeanCPUCores   float64 `json:"mean_cpu_cores"`
	QueueHighBytes uint64  `json:"queue_high_bytes"`
}

type conditionResult struct {
	Name          string           `json:"name"`
	Setups        []setupSample    `json:"setups"`
	Streams       []streamSample   `json:"streams"`
	Resources     []resourceSample `json:"resources"`
	Disclosures   []string         `json:"disclosures,omitempty"`
	CleanupPassed bool             `json:"cleanup_passed"`
}

type negativeCase struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

type negativeResult struct {
	Cases          []negativeCase `json:"cases"`
	FailureElapsed time.Duration  `json:"failure_elapsed_ns"`
}

type candidateVerdict struct {
	Decision string   `json:"decision"`
	Failures []string `json:"failures"`
}

func evaluateCandidate(direct, candidate conditionResult, negatives negativeResult) candidateVerdict {
	failures := make([]string, 0, 8)
	if len(candidate.Disclosures) > 0 {
		return candidateVerdict{Decision: decisionStop, Failures: []string{"forbidden role-view or cleartext disclosure"}}
	}
	passed := 0
	setupTimes := setupDurations(candidate.Setups)
	for _, sample := range candidate.Setups {
		if sample.Passed {
			passed++
		}
	}
	if len(candidate.Setups) != 20 || passed < 19 {
		failures = append(failures, "fewer than 19 of 20 setup attempts passed")
	}
	if len(setupTimes) != 20 || nearestRank(setupTimes, 95) > 3*time.Second {
		failures = append(failures, "setup p95 exceeds 3 seconds")
	}
	for _, direction := range []string{directionUpload, directionDownload} {
		candidateRates, candidateOK := verifiedRates(candidate.Streams, direction)
		directRates, directOK := verifiedRates(direct.Streams, direction)
		if !candidateOK || !directOK || minimum(candidateRates) < goodputThreshold(median(directRates)) {
			failures = append(failures, direction+" minimum goodput misses threshold")
		}
	}
	observedResources := make(map[string]bool, len(candidate.Resources))
	for _, sample := range candidate.Resources {
		if observedResources[sample.Role] {
			failures = append(failures, sample.Role+" resource evidence is duplicated")
		}
		observedResources[sample.Role] = true
		limit := uint64(256 << 20)
		if sample.Endpoint {
			limit = 512 << 20
		}
		if sample.PeakRSSBytes > limit {
			failures = append(failures, sample.Role+" RSS exceeds limit")
		}
		if sample.MeanCPUCores >= 1 {
			failures = append(failures, sample.Role+" mean CPU is not below one core")
		}
		if sample.QueueHighBytes > 256<<10 {
			failures = append(failures, sample.Role+" logical queue exceeds 256 KiB")
		}
	}
	for _, role := range []string{
		"user", "service", "user-entry", "user-interior", "rendezvous", "service-interior", "data-service-entry",
		"introduction-forwarder", "introduction-node", "introduction-interior", "introduction-entry",
	} {
		if !observedResources[role] {
			failures = append(failures, role+" resource evidence is missing")
		}
	}
	for _, sample := range negatives.Cases {
		if !sample.Passed {
			failures = append(failures, "negative case did not fail closed: "+sample.Name)
		}
	}
	if len(negatives.Cases) != 7 {
		failures = append(failures, "mandatory negative case set is incomplete")
	}
	if negatives.FailureElapsed > 15*time.Second {
		failures = append(failures, "rendezvous failure exceeded 15 seconds")
	}
	if !candidate.CleanupPassed {
		failures = append(failures, "cleanup did not remove owned resources")
	}
	decision := decisionAdvance
	if len(failures) > 0 {
		decision = decisionRedesign
	}
	return candidateVerdict{Decision: decision, Failures: failures}
}

func setupDurations(samples []setupSample) []time.Duration {
	result := make([]time.Duration, len(samples))
	for index, sample := range samples {
		result[index] = sample.Elapsed
		if !sample.Passed {
			result[index] = time.Duration(1<<63 - 1)
		}
	}
	return result
}

func verifiedRates(samples []streamSample, direction string) ([]float64, bool) {
	rates := make([]float64, 0, 3)
	for _, sample := range samples {
		if sample.Direction != direction {
			continue
		}
		if !sample.Verified || sample.BitsPerSecond <= 0 {
			return nil, false
		}
		rates = append(rates, sample.BitsPerSecond)
	}
	return rates, len(rates) == 3
}

func nearestRank(values []time.Duration, percentile int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	ordered := slices.Clone(values)
	slices.Sort(ordered)
	rank := (percentile*len(ordered) + 99) / 100
	return ordered[rank-1]
}

func goodputThreshold(directMedian float64) float64 {
	return min(10_000_000, directMedian/2)
}

func minimum(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return slices.Min(values)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := slices.Clone(values)
	slices.Sort(ordered)
	return ordered[len(ordered)/2]
}
