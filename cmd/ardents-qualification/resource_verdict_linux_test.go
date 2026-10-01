//go:build linux

package main

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestResourceVerdictRequiresWholeMonotonicWindow(t *testing.T) {
	origin := time.Unix(1, 0)
	report := streamqualification.Report{Started: origin, Stopped: origin.Add(600 * time.Second), StoppedElapsed: 600 * time.Second}
	fresh := func() *resourceMeasurements {
		series := &resourceMeasurements{}
		for second := 0; second <= 600; second++ {
			series.samples = append(series.samples, resourceObservation{at: time.Duration(second) * time.Second, memory: 400 << 20, cpu: uint64(second) * 400_000})
		}
		return series
	}
	if !streamqualification.CriteriaPassed(fresh().evaluate(report, streamqualification.ReaderRole)) {
		t.Fatal("bounded complete observation refused")
	}
	for _, test := range []struct {
		name   string
		change func(*resourceMeasurements)
	}{
		{"CPU-over-limit", func(s *resourceMeasurements) {
			for i := range s.samples {
				s.samples[i].cpu = uint64(i) * 600_000
			}
		}},
		{"RSS-over-limit", func(s *resourceMeasurements) {
			for i := range s.samples {
				s.samples[i].memory = 513 << 20
			}
		}},
		{"counter-reset", func(s *resourceMeasurements) { s.samples[400].cpu = 0 }},
		{"missing-sample", func(s *resourceMeasurements) { s.samples = append(s.samples[:400], s.samples[401:]...) }},
		{"truncated-tail", func(s *resourceMeasurements) { s.samples = s.samples[:600] }},
		{"duplicate-timestamp", func(s *resourceMeasurements) { s.samples[200].at = s.samples[199].at }},
	} {
		t.Run(test.name, func(t *testing.T) {
			series := fresh()
			test.change(series)
			if streamqualification.CriteriaPassed(series.evaluate(report, streamqualification.ReaderRole)) {
				t.Fatal("incomplete or over-budget evidence passed")
			}
		})
	}
}

func TestOwnerNetworkVerdictAggregatesParticipantsAndReconcilesLedger(t *testing.T) {
	origin := time.Unix(100, 0)
	policy := resource.HostingPolicy{
		Provider: "provider", Start: origin.Add(-time.Hour), End: origin.Add(time.Hour), Unit: "GiB", Quantity: 1,
		Directions: "tx", Interfaces: []string{"eth0"}, LowWatermarkBytes: 1,
	}
	reports := []streamqualification.Report{
		{Started: origin, Stopped: origin.Add(10 * time.Minute), Streams: []streamqualification.StreamMeasurement{{ID: 1, Tx: 400}}},
		{Started: origin.Add(time.Second), Stopped: origin.Add(10 * time.Minute), Streams: []streamqualification.StreamMeasurement{{ID: 3, Tx: 400}}},
	}
	host := func(at time.Time, tx, used uint64) resource.HostingSample {
		return resource.HostingSample{
			At: at, Policy: policy, Boot: "boot", Interfaces: []resource.HostingInterfaceSample{{Name: "eth0", Index: 2, Tx: tx}},
			Observation: resource.HostingObservation{UsedBytes: used},
		}
	}
	series := []*resourceMeasurements{{}, {}}
	for second := 0; second <= 600; second++ {
		counter := uint64(100 + second*2)
		series[0].samples = append(series[0].samples, resourceObservation{host: host(origin.Add(time.Duration(second)*time.Second), counter, counter)})
	}
	verdict, criteria := evaluateOwnerNetwork(series, reports, streamqualification.ReaderRole, streamqualification.ClientToPublisher, streamqualification.NormalNetwork)
	if !streamqualification.CriteriaPassed(criteria) {
		t.Fatalf("bounded aggregate network evidence refused: %+v", criteria)
	}
	if verdict.DirectionalUseful != 800 || verdict.DirectionalWire != 1200 || verdict.DirectionalOverhead != 400 || verdict.HostingLedgerDelta != 1200 {
		t.Fatalf("owner network verdict = %+v", verdict)
	}

	last := len(series[0].samples) - 1
	series[0].samples[last].host.Interfaces[0].Tx++
	_, criteria = evaluateOwnerNetwork(series, reports, streamqualification.ReaderRole, streamqualification.ClientToPublisher, streamqualification.NormalNetwork)
	if streamqualification.CriteriaPassed(criteria) {
		t.Fatal("carrier ratio above the bound accepted")
	}
	series[0].samples[last].host.Interfaces[0].Tx--
	series[0].samples[last].host.Observation.UsedBytes++
	_, criteria = evaluateOwnerNetwork(series, reports, streamqualification.ReaderRole, streamqualification.ClientToPublisher, streamqualification.NormalNetwork)
	if streamqualification.CriteriaPassed(criteria) {
		t.Fatal("interface and durable ledger disagreement accepted")
	}
}
