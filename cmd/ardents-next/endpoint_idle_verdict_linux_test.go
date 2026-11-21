//go:build linux

package main

import (
	"context"
	"errors"
	"math"
	"math/big"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
)

type net32IdleSample struct {
	at        time.Time // harness monotonic sampling instant (series cadence)
	hostingAt time.Time // Hosting wall observation (log correlation only)
	used      uint64    // Hosting ledger used bytes (policy-counted)
	boot      string
	ifaces    []net32IdleInterface
	rss       uint64 // whole-client cgroup memory.current sum
	cpuUsec   uint64 // whole-client cgroup cpu.stat usage_usec sum
}

type net32IdleInterface struct {
	name  string
	index uint64
	tx    uint64
	rx    uint64
}

// net32IdleReport records the fixed short observation; wall timestamps correlate
// logs only and grant no elapsed-time proof.
type net32IdleReport struct {
	Started          time.Time
	Stopped          time.Time
	MeasuredDuration time.Duration
	Samples          uint32
}

type net32Criterion struct {
	Name     string
	Observed float64
	Relation string
	Bound    float64
	Passed   bool
}

type net32IdleVerdict struct {
	Kind                   string
	Observation            string
	MeasuredSeconds        float64
	Samples                uint32
	StateBytes             uint64
	InterfaceTx            uint64
	InterfaceRx            uint64
	ObservedAggregateBytes uint64
	Projected24HourBytes   uint64
	HostingLedgerDelta     uint64
	ResourceMaxGapMs       float64
	ResourceZeroRSS        int
	ResourceCPURegressions int
	ResourceNonMonotonic   int
	ResourceWideGaps       int
	Criteria               []net32Criterion
}

func net32IdleTraffic(window time.Duration) (work, termination hosting.Traffic) {
	proportional := uint64(window)/uint64(24*time.Hour/time.Second) + 1
	return hosting.Traffic{Tx: proportional, Rx: proportional}, hosting.Traffic{Tx: 8 << 20, Rx: 8 << 20}
}

func collectNet32Idle(ctx context.Context, window time.Duration, now func() time.Time, read func(bool) (net32IdleSample, error)) ([]net32IdleSample, net32IdleReport, error) {
	var report net32IdleReport
	var series []net32IdleSample
	// sample returns the monotonic start and completion instants so the projection
	// denominator can exclude both boundary sampling calls.
	sample := func(fresh, first bool) (began, finished time.Time, err error) {
		began = now()
		s, err := read(fresh)
		finished = now()
		if err != nil {
			return began, finished, err
		}
		if first {
			report.Started = s.hostingAt
		}
		// The series cadence is the harness's own monotonic sampling clock; the
		// Hosting ObservedAt may repeat for a cached reading and is correlation-only.
		s.at = began
		series = append(series, s)
		report.Samples++
		return began, finished, nil
	}
	_, firstFinished, err := sample(true, true)
	if err != nil {
		return nil, report, err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	timer := time.NewTimer(window)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, report, ctx.Err()
		case <-ticker.C:
			if _, _, err := sample(false, false); err != nil {
				return nil, report, err
			}
		case <-timer.C:
			lastBegan, _, err := sample(true, false)
			if err != nil {
				return nil, report, err
			}
			report.Stopped = series[len(series)-1].hostingAt
			report.MeasuredDuration = lastBegan.Sub(firstFinished)
			if report.MeasuredDuration <= 0 {
				return nil, report, errors.New("NET-32 monotonic counter interval unavailable")
			}
			return series, report, nil
		}
	}
}

func evaluateNet32Idle(report net32IdleReport, series []net32IdleSample, stateBytes uint64, stateErr error) net32IdleVerdict {
	verdict := net32IdleVerdict{
		Kind:            "net32-short-projection",
		Observation:     "ten-minute observation with an upper 24-hour projection; not a 24-hour observed run",
		MeasuredSeconds: report.MeasuredDuration.Seconds(),
		Samples:         report.Samples,
		StateBytes:      stateBytes,
	}
	add := func(name string, observed float64, relation string, bound float64, passed bool) {
		verdict.Criteria = append(verdict.Criteria, net32Criterion{Name: name, Observed: observed, Relation: relation, Bound: bound, Passed: passed})
	}
	complete := report.MeasuredDuration >= 10*time.Minute && len(series) >= 600 && int(report.Samples) == len(series)
	add("idle-observation-seconds", report.MeasuredDuration.Seconds(), ">=", 600, complete)
	add("one-second-owner-sampling", float64(len(series)), ">=", 600, complete)
	add("state-profile-bytes", float64(stateBytes), "<=", 64<<10, stateErr == nil && stateBytes <= 64<<10)
	if !complete {
		return verdict
	}
	first, last := series[0], series[len(series)-1]
	interfaceComplete := first.boot == last.boot && len(first.ifaces) == len(last.ifaces)
	var tx, rx uint64
	for index, before := range first.ifaces {
		if !interfaceComplete {
			break
		}
		after := last.ifaces[index]
		if before.name != after.name || before.index != after.index || after.tx < before.tx || after.rx < before.rx || after.tx-before.tx > math.MaxUint64-tx || after.rx-before.rx > math.MaxUint64-rx {
			interfaceComplete = false
			break
		}
		tx += after.tx - before.tx
		rx += after.rx - before.rx
	}
	verdict.InterfaceTx, verdict.InterfaceRx = tx, rx
	aggregateOK := tx <= math.MaxUint64-rx
	if aggregateOK {
		verdict.ObservedAggregateBytes = tx + rx
	}
	if aggregateOK && report.MeasuredDuration > 0 {
		projected := math.Ceil(float64(verdict.ObservedAggregateBytes) * (24 * time.Hour).Seconds() / report.MeasuredDuration.Seconds())
		if projected < float64(math.MaxUint64) {
			verdict.Projected24HourBytes = uint64(projected)
		} else {
			aggregateOK = false
		}
	}
	add("interface-counter-window", float64(len(first.ifaces)), "complete", float64(len(last.ifaces)), interfaceComplete)
	add("projected-24-hour-aggregate-bytes", float64(verdict.Projected24HourBytes), "<=", 1_000_000_000, interfaceComplete && aggregateOK && verdict.Projected24HourBytes <= 1_000_000_000)
	// tx+rx provider policy: the ledger delta must equal the counted interface aggregate.
	ledgerOK := last.used >= first.used
	if ledgerOK {
		verdict.HostingLedgerDelta = last.used - first.used
	}
	add("hosting-ledger-bytes", float64(verdict.HostingLedgerDelta), "= interface tx+rx bytes", float64(verdict.ObservedAggregateBytes), interfaceComplete && aggregateOK && ledgerOK && verdict.HostingLedgerDelta == verdict.ObservedAggregateBytes)
	var memory []uint64
	resourceComplete := true
	var maxGap time.Duration
	var zeroRSS, cpuRegressions, nonMonotonic, wideGaps int
	for index, sample := range series {
		memory = append(memory, sample.rss)
		if sample.rss == 0 {
			zeroRSS++
			resourceComplete = false
		}
		if index > 0 {
			gap := sample.at.Sub(series[index-1].at)
			if gap > maxGap {
				maxGap = gap
			}
			if !sample.at.After(series[index-1].at) {
				nonMonotonic++
				resourceComplete = false
			}
			if gap > 1500*time.Millisecond {
				wideGaps++
				resourceComplete = false
			}
			if sample.cpuUsec < series[index-1].cpuUsec {
				cpuRegressions++
				resourceComplete = false
			}
		}
	}
	verdict.ResourceMaxGapMs = float64(maxGap.Milliseconds())
	verdict.ResourceZeroRSS = zeroRSS
	verdict.ResourceCPURegressions = cpuRegressions
	verdict.ResourceNonMonotonic = nonMonotonic
	verdict.ResourceWideGaps = wideGaps
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	p95 := memory[(95*len(memory)+99)/100-1]
	cpu := float64(0)
	if last.at.After(first.at) && last.cpuUsec >= first.cpuUsec {
		cpu = float64(last.cpuUsec-first.cpuUsec) / last.at.Sub(first.at).Seconds() / 10_000
	} else {
		resourceComplete = false
	}
	add("p95-whole-client-RSS-bytes", float64(p95), "<=", 512<<20, resourceComplete && p95 <= 512<<20)
	add("mean-whole-client-CPU-percent-one-core", cpu, "<=", 1, resourceComplete && cpu <= 1)
	return verdict
}

func TestNET32IdleReserveMatchesExactAllowance(t *testing.T) {
	for _, window := range []time.Duration{1, 86399, 86400, 86401, time.Millisecond, time.Second, 18 * time.Second, 19 * time.Second, time.Minute, 10*time.Minute - 1, 10 * time.Minute} {
		work, termination := net32IdleTraffic(window)
		exact := new(big.Int).Mul(big.NewInt(int64(window)), big.NewInt(1_000_000_000))
		exact.Quo(exact, big.NewInt(int64(24*time.Hour))).Add(exact, big.NewInt(1))
		if work.Tx != exact.Uint64() || work.Rx != exact.Uint64() || termination.Tx != 8<<20 || termination.Rx != 8<<20 {
			t.Fatalf("window %v: reserve work %+v, termination %+v, exact work %v", window, work, termination, exact)
		}
	}
}

func TestNET32IdleHostingReservationKeepsProviderAccounting(t *testing.T) {
	const oneDirection = uint64(6_944_445 + 8<<20)
	for _, provider := range []struct {
		directions string
		counted    uint64
	}{{"tx", oneDirection}, {"rx", oneDirection}, {"tx+rx", 2 * oneDirection}} {
		t.Run(provider.directions, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			work, termination := net32IdleTraffic(10 * time.Minute)
			deadline := now.Add(12 * time.Minute)
			request := hosting.ReservationRequest{Work: work, Termination: termination, WorkUntil: deadline, HoldUntil: deadline}
			policy := hosting.Policy{
				Provider: "NET-32 qualification fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
				Unit: "B", Directions: provider.directions, Interfaces: []string{"lo"}, LowWatermarkBytes: 1,
			}
			for _, amount := range []struct {
				name     string
				quantity uint64
			}{{"exact", provider.counted + 1}, {"short", provider.counted}} {
				t.Run(amount.name, func(t *testing.T) {
					policy.Quantity = amount.quantity
					root := filepath.Join(t.TempDir(), "hosting")
					if err := hosting.Initialize(root, policy); err != nil {
						t.Fatal(err)
					}
					budget, err := hosting.Open(root)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := budget.Close(); err != nil {
							t.Error(err)
						}
					})
					held, err := budget.Reserve(t.Context(), request)
					if amount.quantity == provider.counted {
						if !errors.Is(err, hosting.ErrCapacity) {
							t.Fatalf("one byte short: %v, want capacity refusal", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					observation, err := budget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != provider.counted {
						t.Fatalf("held %+v / %v, want %d counted bytes", observation, err, provider.counted)
					}
					if _, err := budget.Reserve(t.Context(), request); !errors.Is(err, hosting.ErrCapacity) {
						t.Fatalf("second reservation: %v, want capacity refusal", err)
					}
					if err := held.Release(t.Context()); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

func TestNET32IdleProjectionRequiresMonotonicAndCompleteSamples(t *testing.T) {
	for _, wallElapsed := range []time.Duration{600 * time.Second, 660 * time.Second} {
		for _, observed := range []uint64{6_000_000, 7_000_000} {
			started := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			report := net32IdleReport{Started: started, Stopped: started.Add(wallElapsed), MeasuredDuration: 600 * time.Second, Samples: 601}
			series := make([]net32IdleSample, int(report.Samples))
			for second := range series {
				at := started.Add(time.Duration(second) * time.Second)
				series[second] = net32IdleSample{at: at, hostingAt: at, boot: "same-boot", ifaces: []net32IdleInterface{{name: "eth0", index: 2}}, rss: 1 << 20}
			}
			last := &series[len(series)-1]
			last.hostingAt, last.ifaces[0].tx, last.used = report.Stopped, observed, observed
			verdict := evaluateNet32Idle(report, series, 4096, nil)
			if want := observed * 144; verdict.Projected24HourBytes != want || net32IdleCriteriaPassed(verdict) != (observed == 6_000_000) {
				t.Fatalf("wall %v bytes %d: projection and criteria = %+v, want %d", wallElapsed, observed, verdict, want)
			}
			if net32IdleCriteriaPassed(evaluateNet32Idle(report, series[:599], 4096, nil)) {
				t.Fatal("incomplete series passed")
			}
			short := report
			short.MeasuredDuration = 599 * time.Second
			if net32IdleCriteriaPassed(evaluateNet32Idle(short, series, 4096, nil)) {
				t.Fatal("short monotonic observation passed")
			}
			last.ifaces = nil
			if net32IdleCriteriaPassed(evaluateNet32Idle(report, series, 4096, nil)) {
				t.Fatal("disappeared interface passed")
			}
		}
	}
}

func net32IdleCriteriaPassed(verdict net32IdleVerdict) bool {
	for _, criterion := range verdict.Criteria {
		if !criterion.Passed {
			return false
		}
	}
	return len(verdict.Criteria) > 0
}

func TestNET32IdleBracketsFreshReads(t *testing.T) {
	started := time.Now()
	clock := []time.Time{started, started.Add(2 * time.Second), started.Add(602 * time.Second), started.Add(606 * time.Second)}
	wall := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	reads, ticks := 0, 0
	_, report, err := collectNet32Idle(t.Context(), time.Millisecond, func() time.Time {
		instant := clock[ticks]
		ticks++
		return instant
	}, func(fresh bool) (net32IdleSample, error) {
		if !fresh {
			t.Fatal("counter boundaries must request a fresh reading")
		}
		defer func() { reads++ }()
		return net32IdleSample{hostingAt: wall.Add(time.Duration(reads) * 660 * time.Second)}, nil
	})
	if err != nil || report.MeasuredDuration != 600*time.Second || report.Samples != 2 || !report.Started.Equal(wall) || !report.Stopped.Equal(wall.Add(660*time.Second)) {
		t.Fatalf("fresh monotonic boundary: %+v / %v", report, err)
	}
}

func TestNET32IdleRetainsSamplingFailure(t *testing.T) {
	failure := errors.New("counter continuity unavailable")
	reads := 0
	_, report, err := collectNet32Idle(t.Context(), time.Millisecond, time.Now, func(bool) (net32IdleSample, error) {
		reads++
		if reads == 2 {
			return net32IdleSample{}, failure
		}
		return net32IdleSample{}, nil
	})
	if !errors.Is(err, failure) || report.MeasuredDuration != 0 {
		t.Fatalf("failed sample produced an interval: %+v / %v", report, err)
	}
}
