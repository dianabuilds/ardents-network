//go:build linux

package streamqualification

import (
	"testing"
	"time"
)

func completeReaderReport() Report {
	start := time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)
	report := Report{Started: start, Stopped: start.Add(10 * time.Minute), Finished: start.Add(10*time.Minute + time.Second)}
	for index := 0; index < 64; index++ {
		id := uint32(index*2 + 1)
		if index >= 16 {
			id = 129 + uint32(index-16)*2
		}
		stream := StreamMeasurement{ID: id, Tx: 46_875_000, MaximumTxGap: 20 * time.Millisecond, LastTx: report.Stopped, LastRx: report.Stopped}
		if index >= 16 {
			stream.Tx, stream.Rx = 32*600, 32*600
		}
		report.Streams = append(report.Streams, stream)
	}
	return report
}

func TestWorkloadVerdictCannotHideAnIndividualFailureInAggregate(t *testing.T) {
	if !CriteriaPassed(EvaluateConditionWorkload(completeReaderReport(), ReaderRole, ClientToPublisher, NormalNetwork)) {
		t.Fatal("complete measured workload refused")
	}
	cases := []struct {
		name   string
		change func(*Report)
	}{
		{"missing retained stream", func(r *Report) { r.Streams = r.Streams[:63] }},
		{"lost active stream", func(r *Report) { r.Streams[0].Tx = 0; r.Streams[1].Tx *= 2 }},
		{"progress gap", func(r *Report) { r.Streams[0].MaximumTxGap = 3 * time.Second }},
		{"tail stall", func(r *Report) { r.Streams[0].LastTx = r.Stopped.Add(-3 * time.Second) }},
		{"forged duplicate", func(r *Report) { r.Streams[1].ID = r.Streams[0].ID }},
		{"missed canary", func(r *Report) { r.Streams[63].Rx -= 32 }},
		{"short run", func(r *Report) { r.Stopped = r.Started.Add(time.Minute) }},
		{"cleanup failure", func(r *Report) { r.Failure = "cleanup timeout" }},
		{"wrong direction", func(r *Report) { r.Streams[0].Tx, r.Streams[0].Rx = r.Streams[0].Rx, r.Streams[0].Tx }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report := completeReaderReport()
			test.change(&report)
			if CriteriaPassed(EvaluateConditionWorkload(report, ReaderRole, ClientToPublisher, NormalNetwork)) {
				t.Fatal("incomplete workload passed")
			}
		})
	}
}

func TestMissingWorkloadAndUnknownProfileCannotPass(t *testing.T) {
	if CriteriaPassed(nil) || CriteriaPassed(EvaluateConditionWorkload(Report{}, ReaderRole, ClientToPublisher, NormalNetwork)) ||
		CriteriaPassed(EvaluateConditionWorkload(completeReaderReport(), ReaderRole, Profile(99), NormalNetwork)) {
		t.Fatal("missing evidence passed")
	}
}

func TestWorkloadBitrateUsesActualElapsedInterval(t *testing.T) {
	if got := measuredBitrate(750_000_000, 601*time.Second); got >= 10_000_000 {
		t.Fatalf("late completion reported passing throughput: %v", got)
	}
	if got := measuredBitrate(750_000_000, 600*time.Second); got != 10_000_000 {
		t.Fatalf("exact window bitrate: %v", got)
	}
	if measuredBitrate(1, 0) != 0 || measuredBitrate(1, -time.Second) != 0 {
		t.Fatal("invalid interval supplied throughput")
	}
}
