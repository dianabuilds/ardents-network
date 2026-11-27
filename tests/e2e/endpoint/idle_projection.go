package endpoint

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

type net32IdleSample struct {
	at                              time.Time // harness monotonic sampling instant (series cadence)
	hostingAt                       time.Time // Hosting wall observation (log correlation only)
	used                            uint64    // Hosting ledger used bytes (policy-counted)
	boot                            string
	ifaces                          []net32IdleInterface
	resourceBegan, resourceFinished time.Time // monotonic bounds around the cgroup reads
	memoryBytes                     uint64    // whole-client cgroup memory.current sum
	cpuUsec                         uint64    // whole-client cgroup cpu.stat usage_usec sum
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
	FailedAttempts   []net32FailedAttempt
}

type net32FailedAttempt struct {
	Stage   string
	Sample  int
	Attempt int
	Error   string
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
	Status                 string
	UnavailableReason      string `json:",omitempty"`
	FailedAttempts         []net32FailedAttempt
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
	ResourceZeroMemory     int
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
		Status:          "observed",
		FailedAttempts:  report.FailedAttempts,
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
	interfaceComplete := first.boot != "" && first.boot == last.boot && len(first.ifaces) != 0 && len(first.ifaces) == len(last.ifaces)
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
	var zeroMemory, cpuRegressions, nonMonotonic, wideGaps int
	for index, sample := range series {
		memory = append(memory, sample.memoryBytes)
		if sample.memoryBytes == 0 {
			zeroMemory++
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
	verdict.ResourceZeroMemory = zeroMemory
	verdict.ResourceCPURegressions = cpuRegressions
	verdict.ResourceNonMonotonic = nonMonotonic
	verdict.ResourceWideGaps = wideGaps
	sort.Slice(memory, func(i, j int) bool { return memory[i] < memory[j] })
	p95 := memory[(95*len(memory)+99)/100-1]
	cpu := float64(0)
	cpuInterval := last.resourceBegan.Sub(first.resourceFinished)
	if !first.resourceFinished.IsZero() && cpuInterval > 0 && last.cpuUsec >= first.cpuUsec {
		cpu = float64(last.cpuUsec-first.cpuUsec) / cpuInterval.Seconds() / 10_000
	} else {
		resourceComplete = false
	}
	add("p95-whole-client-cgroup-memory-bytes", float64(p95), "<=", 512<<20, resourceComplete && p95 <= 512<<20)
	add("mean-whole-client-CPU-percent-one-core", cpu, "<=", 1, resourceComplete && cpu <= 1)
	return verdict
}
