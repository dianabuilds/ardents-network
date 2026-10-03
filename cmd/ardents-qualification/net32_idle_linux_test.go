//go:build linux

package main

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
)

func TestNET32ProjectionCannotPassAfterForwardWallStep(t *testing.T) {
	for _, wallElapsed := range []time.Duration{600 * time.Second, 660 * time.Second} {
		for _, bytes := range []uint64{6_000_000, 7_000_000} {
			started := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			report := endpoint.StreamQualificationIdleReport{
				Started: started, Stopped: started.Add(wallElapsed), MeasuredDuration: 600 * time.Second, Samples: 601,
			}
			series := &resourceMeasurements{}
			for second := 0; second <= 600; second++ {
				host := hostingbudget.Sample{
					At: started.Add(time.Duration(second) * time.Second), Boot: "same-boot",
					Policy:     hostingbudget.Policy{Directions: "tx+rx"},
					Interfaces: []hostingbudget.InterfaceSample{{Name: "eth0", Index: 2}},
				}
				if second == 600 {
					host.At = report.Stopped
					host.Interfaces[0].Tx = bytes
					host.Observation.UsedBytes = bytes
				}
				series.samples = append(series.samples, resourceObservation{
					at: time.Duration(second) * time.Second, memory: 1 << 20, host: host,
				})
			}
			verdict, criteria := evaluateNET32Idle(report, series, 0, 0)
			expected := uint64(864_000_000)
			if bytes == 7_000_000 {
				expected = 1_008_000_000
			}
			if verdict.Projected24HourBytes != expected || streamqualification.CriteriaPassed(criteria) != (bytes == 6_000_000) {
				t.Fatalf("wall=%v bytes=%d projection=%d criteria=%+v", wallElapsed, bytes, verdict.Projected24HourBytes, criteria)
			}
			report.MeasuredDuration = 599 * time.Second
			if _, criteria := evaluateNET32Idle(report, series, 0, 0); streamqualification.CriteriaPassed(criteria) {
				t.Fatal("wall timestamps substituted for actual ten-minute minimum")
			}
			report.MeasuredDuration = 600 * time.Second
			series.samples = series.samples[:599]
			if _, criteria := evaluateNET32Idle(report, series, 0, 0); streamqualification.CriteriaPassed(criteria) {
				t.Fatal("incomplete raw series accepted")
			}
		}
	}
}
