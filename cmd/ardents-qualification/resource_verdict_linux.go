//go:build linux

package main

import (
	"math"
	"slices"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

type resourceObservation struct {
	at          time.Duration
	memory, cpu uint64
	host        resource.HostingSample
}
type resourceMeasurements struct{ samples []resourceObservation }

type ownerNetworkVerdict struct {
	Started, Stopped                                         time.Time
	InterfaceTx, InterfaceRx                                 uint64
	CountedBytes, HostingLedgerDelta                         uint64
	UsefulTx, UsefulRx                                       uint64
	DirectionalWire, DirectionalUseful                       uint64
	DirectionalOverhead                                      uint64
	DirectionalCarrierRatio                                  float64
	TxP95BitsPerSecond                                       float64
	RxP95BitsPerSecond                                       float64
	OneSecondSampleCount                                     int
	P95RSSBytes                                              uint64
	MeanCPUPercent                                           float64
	HostingProvider                                          string
	HostingPeriodStart, HostingPeriodEnd                     time.Time
	HostingUnit, HostingDirections                           string
	HostingQuantity, HostingInitialUsed, HostingLowWatermark uint64
}

func (series *resourceMeasurements) observe(event endpoint.StreamQualificationEvent) {
	if event.Kind == "resource-sample" && event.Host != nil && event.Usage != nil {
		host := *event.Host
		host.Interfaces = append([]resource.HostingInterfaceSample(nil), event.Host.Interfaces...)
		series.samples = append(series.samples, resourceObservation{
			at: event.Elapsed, memory: event.Usage.RSSBytes, cpu: event.Usage.CPUUsageUsec, host: host,
		})
	}
}

func sameHostingPolicy(left, right resource.HostingPolicy) bool {
	return left.Provider == right.Provider && left.Start.Equal(right.Start) && left.End.Equal(right.End) &&
		left.Unit == right.Unit && left.Quantity == right.Quantity && left.Directions == right.Directions &&
		left.InitialUsedBytes == right.InitialUsedBytes && left.LowWatermarkBytes == right.LowWatermarkBytes &&
		slices.Equal(left.Interfaces, right.Interfaces)
}

// evaluateOwnerNetwork reconciles the shared host ledger and the useful
// direction across every participant owned by this one local runner.
func evaluateOwnerNetwork(series []*resourceMeasurements, reports []streamqualification.Report, role streamqualification.Role, profile streamqualification.Profile, condition streamqualification.NetworkCondition) (ownerNetworkVerdict, []streamqualification.Criterion) {
	var verdict ownerNetworkVerdict
	criteria := []streamqualification.Criterion{}
	add := func(name string, observed float64, relation string, bound float64, passed bool) {
		criteria = append(criteria, streamqualification.Criterion{Name: name, Observed: observed, Relation: relation, Bound: bound, Passed: passed})
	}
	_, err := condition.CarrierRatioLimit()
	if err != nil {
		add("owner-network-condition", 0, "valid", 1, false)
		return verdict, criteria
	}
	if len(series) == 0 || len(series) != len(reports) {
		add("owner-interface-window", 0, "complete", 1, false)
		return verdict, criteria
	}
	started, stopped := time.Time{}, time.Time{}
	var usefulTx, usefulRx uint64
	for _, report := range reports {
		if report.Started.IsZero() || report.Stopped.IsZero() {
			add("owner-interface-window", 0, "complete", 1, false)
			return verdict, criteria
		}
		// Enclose every participant's measured interval. Extra interface bytes
		// make the ratio conservative; omitting a participant's edge would not.
		if started.IsZero() || report.Started.Before(started) {
			started = report.Started
		}
		if stopped.IsZero() || report.Stopped.After(stopped) {
			stopped = report.Stopped
		}
		for _, stream := range report.Streams {
			if stream.ID <= 127 {
				usefulTx += stream.Tx
				usefulRx += stream.Rx
			}
		}
	}
	verdict.Started, verdict.Stopped = started, stopped
	verdict.UsefulTx, verdict.UsefulRx = usefulTx, usefulRx
	var samples []resource.HostingSample
	for _, measurements := range series {
		for _, sample := range measurements.samples {
			samples = append(samples, sample.host)
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].At.Before(samples[j].At) })
	var before, after *resource.HostingSample
	for index := range samples {
		sample := &samples[index]
		if !sample.At.After(started) {
			before = sample
		}
		if after == nil && !sample.At.Before(stopped) {
			after = sample
		}
	}
	complete := before != nil && after != nil && started.Before(stopped) && sameHostingPolicy(before.Policy, after.Policy) && before.Boot == after.Boot && len(before.Interfaces) == len(after.Interfaces)
	var tx, rx uint64
	if complete {
		for index, first := range before.Interfaces {
			last := after.Interfaces[index]
			txDelta, rxDelta := last.Tx-first.Tx, last.Rx-first.Rx
			if first.Name != last.Name || first.Index != last.Index || last.Tx < first.Tx || last.Rx < first.Rx || txDelta > math.MaxUint64-tx || rxDelta > math.MaxUint64-rx {
				complete = false
				break
			}
			tx += txDelta
			rx += rxDelta
		}
	}
	add("owner-interface-window", float64(len(samples)), "complete", 2, complete)
	if !complete {
		return verdict, criteria
	}
	verdict.InterfaceTx, verdict.InterfaceRx = tx, rx
	counted := tx
	switch before.Policy.Directions {
	case "rx":
		counted = rx
	case "tx+rx":
		if tx > math.MaxUint64-rx {
			add("owner-hosting-ledger-bytes", 0, "available", 1, false)
			return verdict, criteria
		}
		counted = tx + rx
	}
	usedDelta := uint64(0)
	if after.Observation.UsedBytes >= before.Observation.UsedBytes {
		usedDelta = after.Observation.UsedBytes - before.Observation.UsedBytes
	} else {
		complete = false
	}
	verdict.CountedBytes, verdict.HostingLedgerDelta = counted, usedDelta
	verdict.HostingProvider = before.Policy.Provider
	verdict.HostingPeriodStart, verdict.HostingPeriodEnd = before.Policy.Start, before.Policy.End
	verdict.HostingUnit, verdict.HostingDirections = before.Policy.Unit, before.Policy.Directions
	verdict.HostingQuantity, verdict.HostingInitialUsed = before.Policy.Quantity, before.Policy.InitialUsedBytes
	verdict.HostingLowWatermark = before.Policy.LowWatermarkBytes
	add("owner-hosting-ledger-bytes", float64(usedDelta), "= interface policy bytes", float64(counted), complete && usedDelta == counted)
	sender := role == streamqualification.ReaderRole && profile == streamqualification.ClientToPublisher || role == streamqualification.PublisherRole && profile == streamqualification.PublisherToClient
	useful := usefulRx
	if sender {
		useful = usefulTx
	}
	if tx > math.MaxUint64-rx {
		add("owner-directional-carrier-ratio", 0, "available", 1, false)
		return verdict, criteria
	}
	wire := tx + rx
	verdict.DirectionalUseful, verdict.DirectionalWire = useful, wire
	if wire >= useful {
		verdict.DirectionalOverhead = wire - useful
	}
	ratio := float64(0)
	if useful != 0 {
		ratio = float64(wire) / float64(useful)
	}
	verdict.DirectionalCarrierRatio = ratio
	txP95, txCount, txComplete := directionalCarrierP95(samples, *before, started, stopped, true)
	rxP95, rxCount, rxComplete := directionalCarrierP95(samples, *before, started, stopped, false)
	count := min(txCount, rxCount)
	verdict.TxP95BitsPerSecond, verdict.RxP95BitsPerSecond, verdict.OneSecondSampleCount = txP95, rxP95, count
	rateComplete := txComplete && rxComplete
	add("owner-one-second-carrier-samples", float64(count), ">=", 598, rateComplete && count >= 598)
	return verdict, criteria
}

func directionalCarrierP95(samples []resource.HostingSample, before resource.HostingSample, started, stopped time.Time, sender bool) (float64, int, bool) {
	prior, ok := directionalCounter(before, sender)
	if !ok {
		return 0, 0, false
	}
	priorAt := before.At
	var rates []float64
	complete := true
	for _, sample := range samples {
		if !sample.At.After(started) || sample.At.After(stopped) || sample.At.Sub(priorAt) < 900*time.Millisecond {
			continue
		}
		current, valid := directionalCounter(sample, sender)
		interval := sample.At.Sub(priorAt)
		if !valid || current < prior || interval > 1500*time.Millisecond {
			complete = false
			break
		}
		rates = append(rates, float64(current-prior)*8/interval.Seconds())
		prior, priorAt = current, sample.At
	}
	if stopped.Sub(priorAt) > 1500*time.Millisecond || len(rates) == 0 {
		complete = false
	}
	sort.Float64s(rates)
	if len(rates) == 0 {
		return 0, 0, complete
	}
	return rates[(95*len(rates)+99)/100-1], len(rates), complete
}

func directionalCounter(sample resource.HostingSample, sender bool) (uint64, bool) {
	var total uint64
	for _, counter := range sample.Interfaces {
		value := counter.Rx
		if sender {
			value = counter.Tx
		}
		if value > math.MaxUint64-total {
			return 0, false
		}
		total += value
	}
	return total, true
}
func (series *resourceMeasurements) evaluate(report streamqualification.Report, role streamqualification.Role) []streamqualification.Criterion {
	var samples []resourceObservation
	// Include the immediately preceding sample so startup CPU cannot disappear
	// into an unobserved partial interval at the start of the ten-minute window.
	for index, sample := range series.samples {
		if sample.at < report.StartedElapsed && index+1 < len(series.samples) && series.samples[index+1].at < report.StartedElapsed {
			continue
		}
		samples = append(samples, sample)
		if sample.at >= report.StoppedElapsed {
			break
		}
	}
	complete := len(samples) >= 600 && !report.Started.IsZero() && !report.Stopped.IsZero()
	if complete {
		complete = samples[0].at <= report.StartedElapsed && samples[len(samples)-1].at >= report.StoppedElapsed
	}
	var memory []uint64
	for index, sample := range samples {
		memory = append(memory, sample.memory)
		if sample.memory == 0 {
			complete = false
		}
		if index > 0 && (sample.cpu < samples[index-1].cpu || sample.at <= samples[index-1].at || (sample.at-samples[index-1].at) > 1500*time.Millisecond) {
			complete = false
		}
	}
	p95, mean := float64(0), float64(0)
	if len(memory) > 0 {
		sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
		p95 = float64(memory[(95*len(memory)+99)/100-1])
	}
	if len(samples) > 1 {
		first, last := samples[0], samples[len(samples)-1]
		if last.cpu >= first.cpu && last.at > first.at {
			mean = float64(last.cpu-first.cpu) / (last.at - first.at).Seconds() / 10_000
		}
	}
	memoryLimit, cpuLimit := float64(512<<20), float64(50)
	if role == streamqualification.PublisherRole {
		memoryLimit, cpuLimit = 1<<30, 100
	}
	return []streamqualification.Criterion{
		{Name: "one-second-owner-sampling", Observed: float64(len(samples)), Relation: ">=", Bound: 600, Passed: complete},
		{Name: "p95-owner-RSS-bytes", Observed: p95, Relation: "<=", Bound: memoryLimit, Passed: complete && p95 <= memoryLimit},
		{Name: "mean-owner-CPU-percent-one-core", Observed: mean, Relation: "<=", Bound: cpuLimit, Passed: complete && mean <= cpuLimit},
	}
}
