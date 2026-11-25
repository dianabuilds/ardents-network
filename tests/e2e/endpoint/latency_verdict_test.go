package endpoint

import (
	"strings"
	"testing"
)

func TestLatencyReportChecksAttemptIdentityAndStatistics(t *testing.T) {
	const report = `{
		"kind":"cold-warm-component","seed":"seed-3",
		"cold":[{"kind":"cold","index":0,"success":true,"elapsed_ms":3001}],
		"warm":[{"kind":"warm","index":0,"success":true,"elapsed_ms":1000}],
		"cold_stats":{"p95_ms":3001},"warm_stats":{"p95_ms":1000},
		"cold_p95_ok":false,"warm_p95_ok":true
	}`
	CheckLatencyVerdict(t, []byte(report), 1, 1, "seed-3")
	for _, tc := range []struct{ name, original, substituted string }{
		{"foreign seed", "seed-3", "seed-2"},
		{"missing attempt", `[{"kind":"cold","index":0,"success":true,"elapsed_ms":3001}]`, "[]"},
		{"wrong attempt index", `"index":0`, `"index":1`},
		{"failed attempt", `"success":true`, `"success":false,"detail":"timed out"`},
		{"missing duration", `,"elapsed_ms":3001`, ""},
		{"wrong p95", `"p95_ms":3001`, `"p95_ms":3000`},
		{"wrong bound flag", `"cold_p95_ok":false`, `"cold_p95_ok":true`},
		{"missing bound flag", `"cold_p95_ok":false,`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modified := strings.Replace(report, tc.original, tc.substituted, 1)
			if modified == report {
				t.Fatal("test did not change the participant report")
			}
			if _, _, err := validateLatencyVerdict([]byte(modified), 1, 1, "seed-3"); err == nil {
				t.Fatal("admitted malformed or inconsistent participant report")
			}
		})
	}
}

func TestLatencyReportUsesNearestRankAtTwentyAttempts(t *testing.T) {
	outcomes := make([]latencyFixtureOutcome, 20)
	for index := range outcomes {
		elapsed := float64(len(outcomes) - index)
		outcomes[index] = latencyFixtureOutcome{Kind: "warm", Index: &index, Success: true, Elapsed: &elapsed}
	}
	withinBound := true
	if got, err := validateLatencySeries("warm", outcomes, []byte("19"), &withinBound, 1000); err != nil || got != 19 {
		t.Fatalf("p95 for descending durations 20 through 1 = %v, %v", got, err)
	}
	if _, err := validateLatencySeries("warm", outcomes, []byte("20"), &withinBound, 1000); err == nil {
		t.Fatal("admitted maximum as p95")
	}
}
