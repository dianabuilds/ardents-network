package blockedverify

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRecomputeFinalSummaryUsesCommittedMeasurements(t *testing.T) {
	published := validFinalSummary()
	snapshots := finalMeasurementSnapshots(t, &published)
	recomputed, reasons := recomputeFinalSummary(snapshots, &published)
	if len(reasons) != 0 || !reflect.DeepEqual(recomputed, &published) {
		t.Fatalf("recomputed=%+v reasons=%v", recomputed, reasons)
	}
	published.Capacity[0].Offered++
	_, reasons = recomputeFinalSummary(snapshots, &published)
	if len(reasons) == 0 || !strings.Contains(strings.Join(reasons, " "), "differs") {
		t.Fatalf("mutated published aggregate was accepted: %v", reasons)
	}
}

func finalMeasurementSnapshots(t *testing.T, summary *finalSummary) map[string][]byte {
	t.Helper()
	resources, traffic := summaryMeasurementRecords(summary)
	host := finalHostRecord{Schema: "ardents-h3-final-host-v1", Hosts: summary.Hosts}
	encodedHost, err := json.MarshalIndent(host, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"measurements/profiles.jsonl":  finalMeasurementLines(t, summary.Profiles),
		"measurements/cells.jsonl":     finalMeasurementLines(t, summary.Cells),
		"measurements/capacity.jsonl":  finalMeasurementLines(t, summary.Capacity),
		"measurements/sustained.jsonl": finalMeasurementLines(t, summary.Sustained),
		"measurements/pressure.jsonl":  finalMeasurementLines(t, summary.Pressure),
		"measurements/recovery.jsonl":  finalMeasurementLines(t, []finalRecovery{summary.Recovery}),
		"measurements/resources.jsonl": finalMeasurementLines(t, resources),
		"measurements/traffic.jsonl":   finalMeasurementLines(t, traffic),
		"measurements/host.json":       append(encodedHost, '\n'),
	}
}

func finalMeasurementLines[T any](t *testing.T, values []T) []byte {
	t.Helper()
	lines := make([]string, 0, len(values))
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
