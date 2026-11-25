package endpoint

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
)

type latencyFixtureSeries struct {
	P95 json.RawMessage `json:"p95_ms"`
}

type latencyFixtureOutcome struct {
	Kind    string   `json:"kind"`
	Index   *int     `json:"index"`
	Success bool     `json:"success"`
	Elapsed *float64 `json:"elapsed_ms"`
	Detail  string   `json:"detail"`
}

// CheckLatencyVerdict validates an actual short-run report and records any
// bound exceedance as a finding for the full qualification campaign.
func CheckLatencyVerdict(t testing.TB, raw []byte, cold, warm int, seed string) {
	t.Helper()
	coldP95, warmP95, err := validateLatencyVerdict(raw, cold, warm, seed)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cold/warm component verdict: %d cold outcomes, p95=%v ms; %d warm outcomes, p95=%v ms", cold, coldP95, warm, warmP95)
	if coldP95 > 3000 {
		t.Logf("FINDING: cold p95 exceeds the 3 s bound on this fixture stand: %v ms", coldP95)
	}
	if warmP95 > 1000 {
		t.Logf("FINDING: warm p95 exceeds the 1 s bound on this fixture stand: %v ms", warmP95)
	}
}

func validateLatencyVerdict(raw []byte, cold, warm int, seed string) (float64, float64, error) {
	var report struct {
		Kind      string                  `json:"kind"`
		Seed      string                  `json:"seed"`
		Cold      []latencyFixtureOutcome `json:"cold"`
		Warm      []latencyFixtureOutcome `json:"warm"`
		ColdStats latencyFixtureSeries    `json:"cold_stats"`
		WarmStats latencyFixtureSeries    `json:"warm_stats"`
		ColdOK    *bool                   `json:"cold_p95_ok"`
		WarmOK    *bool                   `json:"warm_p95_ok"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return 0, 0, fmt.Errorf("decode cold/warm component report: %w", err)
	}
	if report.Kind != "cold-warm-component" || report.Seed != seed || cold <= 0 || warm <= 0 ||
		len(report.Cold) != cold || len(report.Warm) != warm {
		return 0, 0, fmt.Errorf("cold/warm report does not match declared run (kind=%q seed=%q cold=%d warm=%d)", report.Kind, report.Seed, len(report.Cold), len(report.Warm))
	}
	coldP95, err := validateLatencySeries("cold", report.Cold, report.ColdStats.P95, report.ColdOK, 3000)
	if err != nil {
		return 0, 0, err
	}
	warmP95, err := validateLatencySeries("warm", report.Warm, report.WarmStats.P95, report.WarmOK, 1000)
	return coldP95, warmP95, err
}

func validateLatencySeries(kind string, outcomes []latencyFixtureOutcome, reported json.RawMessage, withinBound *bool, bound float64) (float64, error) {
	durations := make([]float64, len(outcomes))
	for i, outcome := range outcomes {
		if outcome.Kind != kind || outcome.Index == nil || *outcome.Index != i {
			return 0, fmt.Errorf("%s run did not report the original attempt at index %d", kind, i)
		}
		if !outcome.Success {
			return 0, fmt.Errorf("%s attempt %d failed (infinite completion time): %s", kind, i, outcome.Detail)
		}
		if outcome.Elapsed == nil || *outcome.Elapsed < 0 {
			return 0, fmt.Errorf("%s attempt %d has no valid duration", kind, i)
		}
		durations[i] = *outcome.Elapsed
	}
	sort.Float64s(durations)
	// Nearest rank for p95 is ceil(0.95*n), equivalently n-floor(n/20).
	expected := durations[len(durations)-len(durations)/20-1]
	var p95 *float64
	if err := json.Unmarshal(reported, &p95); err != nil || p95 == nil || *p95 != expected ||
		withinBound == nil || *withinBound != (expected <= bound) {
		return 0, fmt.Errorf("%s reported p95 or bound flag differs from recorded attempts (expected %v ms)", kind, expected)
	}
	return expected, nil
}
