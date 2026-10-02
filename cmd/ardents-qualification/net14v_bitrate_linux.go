//go:build linux

package main

import (
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

// net14vDirectionalCriteria uses the access segment at each endpoint, rather
// than the end-to-end path bottleneck or the shared host's unrelated traffic.
func net14vDirectionalCriteria(manifest qualificationNetworkManifest, relay relayTrafficVerdict, reader, publisher ownerNetworkVerdict) []streamqualification.Criterion {
	var criteria []streamqualification.Criterion
	for _, owner := range []struct {
		name, id string
		network  ownerNetworkVerdict
	}{{"reader", userEndpoint, reader}, {"publisher", publisherEndpoint, publisher}} {
		for _, direction := range []string{"tx", "rx"} {
			var selected networkSegment
			matches := 0
			for _, path := range manifest.Paths {
				for _, segment := range path.Segments {
					if direction == "tx" && segment.From == owner.id || direction == "rx" && segment.To == owner.id {
						selected = segment
						matches++
					}
				}
			}
			var budget uint64
			budgets := 0
			for _, link := range manifest.Relays {
				if link.UpstreamSegment == selected.ID {
					budget = link.UpstreamRate
					budgets++
				}
				if link.ClientSegment == selected.ID {
					budget = link.ClientRate
					budgets++
				}
			}
			var observed relaySegmentTraffic
			observations := 0
			for _, segment := range relay.Segments {
				if segment.ID == selected.ID {
					observed = segment
					observations++
				}
			}
			rate, count, complete := relayDirectionalP95(observed, owner.network.Started, owner.network.Stopped)
			complete = complete && matches == 1 && budgets == 1 && budget != 0 && observations == 1 && observed.From == selected.From && observed.To == selected.To
			bound := min(25_000_000.0, float64(budget)*0.8)
			prefix := owner.name + "-" + direction
			criteria = append(criteria,
				streamqualification.Criterion{Name: prefix + "-one-second-carrier-samples", Observed: float64(count), Relation: ">=", Bound: 598, Passed: complete},
				streamqualification.Criterion{Name: prefix + "-p95-one-second-carrier-bitrate", Observed: rate, Relation: "<=", Bound: bound, Passed: complete && rate <= bound},
			)
		}
	}
	return criteria
}

// Wall timestamps locate the workload window; only a single sampler's monotonic
// elapsed values measure rate. Retain raw adjacent observations: never smooth,
// resample, sort timestamps, or discard a bad interval to obtain a percentile.
// The nominal one-second cadence permits only 100ms of scheduling jitter;
// longer windows could dilute bursts and are not one-second evidence.
func relayDirectionalP95(segment relaySegmentTraffic, started, stopped time.Time) (float64, int, bool) {
	samples := segment.Samples
	if started.IsZero() || stopped.Sub(started) < 600*time.Second || len(samples) < 2 {
		return 0, 0, false
	}
	first, last := -1, -1
	for index, sample := range samples {
		if sample.At.IsZero() || sample.Elapsed < 0 || sample.Bytes > segment.Bytes {
			return 0, 0, false
		}
		if index > 0 {
			prior := samples[index-1]
			interval := sample.Elapsed - prior.Elapsed
			if !sample.At.After(prior.At) || sample.Bytes < prior.Bytes || interval < 900*time.Millisecond || interval > 1100*time.Millisecond {
				return 0, 0, false
			}
		}
		if !sample.At.After(started) {
			first = index
		}
		if last < 0 && !sample.At.Before(stopped) {
			last = index
		}
	}
	if first < 0 || last <= first || started.Sub(samples[first].At) > 1500*time.Millisecond || samples[last].At.Sub(stopped) > 1500*time.Millisecond {
		return 0, 0, false
	}
	// A shortened monotonic window cannot pass because wall time moved forward.
	if samples[last].Elapsed-samples[first].Elapsed < 600*time.Second {
		return 0, 0, false
	}
	rates := make([]float64, 0, last-first)
	for index := first + 1; index <= last; index++ {
		current, prior := samples[index], samples[index-1]
		rates = append(rates, float64(current.Bytes-prior.Bytes)*8/(current.Elapsed-prior.Elapsed).Seconds())
	}
	sort.Float64s(rates)
	return rates[(95*len(rates)+99)/100-1], len(rates), len(rates) >= 598
}
