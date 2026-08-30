//go:build browsercompat

package main

import (
	"errors"
	"sort"
	"time"
)

const (
	maximumDynamicWorkloadCycles = uint32(1800)
	maximumDynamicWorkloadBytes  = uint32(64 << 20)
	// The Publisher Application becomes connected before the local Endpoint
	// exposes the Browser origin. This bounded window covers that assembly;
	// it is deliberately distinct from the per-HTTP-cycle deadline.
	dynamicWorkloadInitialRequestGrace = 15 * time.Second
)

// dynamicWorkloadConfig is the bounded A11 HTTP workload carried by one
// already-selected Service Connection. For terminal scenarios Cycles is the
// number of successful warmup cycles before the declared fault is injected.
type dynamicWorkloadConfig struct {
	Cycles                      uint32
	IntervalMilliseconds        uint32
	CycleDeadlineMilliseconds   uint32
	MaximumStartLagMilliseconds uint32 `json:",omitempty"`
	NoFallbackEvery             uint32
	BytesEachDirection          uint32
}

type dynamicWorkloadPlan struct {
	cycles              uint32
	interval            time.Duration
	cycleDeadline       time.Duration
	maximumStartLag     time.Duration
	initialRequestGrace time.Duration
	noFallbackEvery     uint32
}

type dynamicWorkloadControls struct {
	endpointCrashReady, applicationFaultReady, applicationFaultRelease, transitFaultReady string
}

// dynamicWorkloadResult exposes aggregate latency metrics. Its unexported
// samples are bounded to the contract's 1,800 cycles and never enter evidence.
type dynamicWorkloadResult struct {
	InstrumentationBoundary       string
	PlannedStartAtUTC             time.Time
	ActualStartAtUTC              time.Time
	ElapsedMicros                 int64
	ExpectedCycles                uint32
	CompletedCycles               uint32
	PeriodicNoFallbackProbeRounds uint32
	ProxyTCPDialCount             uint32
	RejectedProxyRedials          uint32
	TerminalNoFallback            bool
	MinimumCycleLatencyMicros     int64
	P50CycleLatencyMicros         int64
	P95CycleLatencyMicros         int64
	P99CycleLatencyMicros         int64
	MaximumCycleLatencyMicros     int64
	MeanCycleLatencyMicros        int64
	MaximumStartLagMicros         int64
	TerminalLatencyMicros         int64
	latencyTotal                  time.Duration
	latencySamples                []int64
}

func (workload dynamicWorkloadConfig) configured() bool {
	return workload.Cycles != 0 || workload.IntervalMilliseconds != 0 || workload.CycleDeadlineMilliseconds != 0 ||
		workload.MaximumStartLagMilliseconds != 0 || workload.NoFallbackEvery != 0 || workload.BytesEachDirection != 0
}

func (workload dynamicWorkloadConfig) validate(input config) error {
	if !workload.configured() {
		return nil
	}
	if !input.TransparentApplication || input.PublisherOffline || input.FirefoxExecutable != "" || input.BrowserEntryStatePath != "" ||
		input.HeldRouteReady != "" || input.HeldRouteRelease != "" {
		return errors.New("C2 configured dynamic workload requires the internal transparent Application path")
	}
	maximumStartLag := workload.maximumStartLagMilliseconds()
	explicitStartLagInvalid := workload.MaximumStartLagMilliseconds != 0 &&
		(maximumStartLag < workload.IntervalMilliseconds || maximumStartLag > workload.CycleDeadlineMilliseconds)
	if workload.Cycles == 0 || workload.Cycles > maximumDynamicWorkloadCycles || workload.IntervalMilliseconds == 0 ||
		workload.IntervalMilliseconds > 60_000 || workload.CycleDeadlineMilliseconds < 10 || workload.CycleDeadlineMilliseconds > 5_000 ||
		explicitStartLagInvalid ||
		workload.NoFallbackEvery == 0 || workload.NoFallbackEvery > workload.Cycles ||
		workload.BytesEachDirection < 1<<20 || workload.BytesEachDirection > maximumDynamicWorkloadBytes {
		return errors.New("C2 configured dynamic workload is incomplete or outside its bound")
	}
	deadline, err := input.deadline()
	minimumRuntime := time.Duration(workload.Cycles)*time.Duration(workload.IntervalMilliseconds)*time.Millisecond +
		time.Duration(max(workload.CycleDeadlineMilliseconds, maximumStartLag))*time.Millisecond + dynamicWorkloadInitialRequestGrace
	if err != nil || time.Until(deadline) < minimumRuntime {
		return errors.New("C2 configured dynamic workload does not fit its fixture deadline")
	}
	return nil
}

func (workload dynamicWorkloadConfig) plan() dynamicWorkloadPlan {
	return dynamicWorkloadPlan{cycles: workload.Cycles, interval: time.Duration(workload.IntervalMilliseconds) * time.Millisecond,
		cycleDeadline:       time.Duration(workload.CycleDeadlineMilliseconds) * time.Millisecond,
		maximumStartLag:     time.Duration(workload.maximumStartLagMilliseconds()) * time.Millisecond,
		initialRequestGrace: dynamicWorkloadInitialRequestGrace, noFallbackEvery: workload.NoFallbackEvery}
}

func (workload dynamicWorkloadConfig) maximumStartLagMilliseconds() uint32 {
	if workload.MaximumStartLagMilliseconds != 0 {
		return workload.MaximumStartLagMilliseconds
	}
	return workload.IntervalMilliseconds
}

func (plan dynamicWorkloadPlan) startLagLimit() time.Duration {
	if plan.maximumStartLag > 0 {
		return plan.maximumStartLag
	}
	return plan.interval
}

func (plan dynamicWorkloadPlan) initialRequestDeadline() time.Duration {
	if plan.initialRequestGrace > 0 {
		return plan.initialRequestGrace
	}
	return plan.cycleDeadline
}

func (input config) streamBytesEachDirection() uint32 {
	if input.DynamicWorkload.configured() {
		return input.DynamicWorkload.BytesEachDirection
	}
	return 64 << 10
}

func newDynamicWorkloadResult(plan dynamicWorkloadPlan) dynamicWorkloadResult {
	return dynamicWorkloadResult{InstrumentationBoundary: "direct-module-runtime; command IPC and Route-attachment counters are not applicable",
		ExpectedCycles: plan.cycles}
}

func (result *dynamicWorkloadResult) recordCycle(latency, startLag time.Duration) {
	result.CompletedCycles++
	latencyMicros := max(int64(1), latency.Microseconds())
	if result.MinimumCycleLatencyMicros == 0 || latencyMicros < result.MinimumCycleLatencyMicros {
		result.MinimumCycleLatencyMicros = latencyMicros
	}
	if latencyMicros > result.MaximumCycleLatencyMicros {
		result.MaximumCycleLatencyMicros = latencyMicros
	}
	result.latencyTotal += latency
	result.latencySamples = append(result.latencySamples, latencyMicros)
	result.MeanCycleLatencyMicros = max(int64(1), (result.latencyTotal / time.Duration(result.CompletedCycles)).Microseconds())
	if lagMicros := max(int64(0), startLag.Microseconds()); lagMicros > result.MaximumStartLagMicros {
		result.MaximumStartLagMicros = lagMicros
	}
}

func (result *dynamicWorkloadResult) finalizeLatencyQuantiles() {
	if len(result.latencySamples) == 0 {
		return
	}
	samples := append([]int64(nil), result.latencySamples...)
	sort.Slice(samples, func(first, second int) bool { return samples[first] < samples[second] })
	result.P50CycleLatencyMicros = dynamicNearestRank(samples, 50)
	result.P95CycleLatencyMicros = dynamicNearestRank(samples, 95)
	result.P99CycleLatencyMicros = dynamicNearestRank(samples, 99)
}

func dynamicNearestRank(samples []int64, percentile int) int64 {
	index := (percentile*len(samples) + 99) / 100
	return samples[max(1, index)-1]
}
