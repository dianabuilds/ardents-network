//go:build linux && endpoint_launch_native

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// NET-32 user-idle component measurement (M2 slice 3a, #652). It provisions ONE
// platform-led successor Endpoint to Service readiness and leaves it idle — no
// publication, no Application job — then observes the required State/time refresh
// traffic and whole-client RSS over a bounded ten-minute window with a conservative
// 24-hour projection (privacy-qualification.md:80,112-132). This is component
// evidence for one fixture operator, NOT a 24-hour observed run and NOT whole-host
// privacy qualification. It reuses the #391 platform-led launch stand; the product
// Hosting API is unchanged — the harness reads the persisted Hosting reading and
// the unit cgroups directly for the interface/resource reconciliation.
const net32IdleWindow = 10 * time.Minute

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

func TestNativeEndpointPlatformLedIdle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("NET-32 idle qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	// Normal idle refresh cadence (60 s), not the launch fixture's fast 1000 ms
	// polling: NET-32 measures ordinary user-idle refresh traffic.
	stand := provisionPlatformLedLaunchRefresh(t, 60000)
	t.Cleanup(func() {
		if t.Failed() {
			journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "40", "-o", "cat").CombinedOutput()
			t.Logf("participant journal:\n%s", journal)
		}
	})
	original := stand.startPlatformLed(t)
	stand.grantPermissions(t, original)
	stand.waitServiceReady(t, original)
	t.Logf("NET-32 idle Endpoint ready; MainPID=%s InvocationID=%s", original[0], original[1])

	hostingRoot := filepath.Join(stand.base, "entry", "hosting")
	series, report, err := observeNet32Idle(t.Context(), hostingRoot, original[0], net32IdleWindow)
	if err != nil {
		t.Fatalf("NET-32 idle observation: %v", err)
	}
	stateBytes, stateErr := net32StateProfileBytes(filepath.Join(stand.base, "state"))
	verdict := evaluateNet32Idle(report, series, stateBytes, stateErr)
	raw, _ := json.MarshalIndent(verdict, "", "  ")
	t.Logf("NET-32 short projection verdict:\n%s", raw)
	for _, criterion := range verdict.Criteria {
		if !criterion.Passed {
			t.Errorf("NET-32 criterion failed: %s observed=%v %s bound=%v", criterion.Name, criterion.Observed, criterion.Relation, criterion.Bound)
		}
	}
	stand.stopParticipant(t, original)
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

// observeNet32Idle reserves the proportional NET-32 ceiling plus separate
// termination capacity in the live shared Hosting budget, then brackets fresh
// counter reads conservatively: the first completion is after every first
// interface read and the last start before every final read, so neither
// provisioning nor the final callback enlarges the monotonic denominator.
func observeNet32Idle(ctx context.Context, hostingRoot, mainPID string, window time.Duration) ([]net32IdleSample, net32IdleReport, error) {
	var report net32IdleReport
	budget, err := hosting.Open(hostingRoot)
	if err != nil {
		return nil, report, err
	}
	defer budget.Close()
	lifetime, cancel := context.WithTimeout(ctx, window+2*time.Minute)
	defer cancel()
	proportional := uint64(window)/uint64(24*time.Hour/time.Second) + 1
	work := hosting.Traffic{Tx: proportional, Rx: proportional}
	termination := hosting.Traffic{Tx: 8 << 20, Rx: 8 << 20}
	reservation, err := budget.Reserve(lifetime, hosting.ReservationRequest{
		Work: work, Termination: termination,
		WorkUntil: time.Now().Add(window + 2*time.Minute), HoldUntil: time.Now().Add(window + 2*time.Minute),
	})
	if err != nil {
		return nil, report, fmt.Errorf("reserve NET-32 idle floor: %w", err)
	}
	defer func() {
		release, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = reservation.Release(release)
	}()
	cgroups, err := net32ClientCgroups(mainPID)
	if err != nil {
		return nil, report, err
	}
	// Let the Endpoint's initial refresh burst settle so the timed window opens
	// against a sampleable budget; startup lease contention is transient and is
	// excluded from the measured window.
	warmupDeadline := time.Now().Add(30 * time.Second)
	for {
		_, sampleErr := budget.Sample(lifetime, time.Second)
		if sampleErr == nil {
			break
		}
		if time.Now().After(warmupDeadline) || lifetime.Err() != nil {
			return nil, report, fmt.Errorf("NET-32 budget not sampleable after startup: %w", sampleErr)
		}
		select {
		case <-lifetime.Done():
			return nil, report, lifetime.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	var series []net32IdleSample
	// sample returns the monotonic start and completion instants so the projection
	// denominator can exclude both boundary sampling calls.
	sample := func(fresh, first bool) (began, finished time.Time, err error) {
		began = time.Now()
		s, err := readNet32Sample(lifetime, budget, hostingRoot, cgroups, fresh)
		finished = time.Now()
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
		case <-lifetime.Done():
			return nil, report, lifetime.Err()
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

// readNet32Sample forces a fresh Hosting measure when requested (Sample writes the
// persisted reading under the per-transact lease), then reads that exact reading
// back so the interface counters reconcile with the ledger, plus whole-client
// cgroup RSS/CPU. The persisted budget.json is the Hosting owner's evidence file.
func readNet32Sample(ctx context.Context, budget *hosting.Budget, hostingRoot string, cgroups []string, fresh bool) (net32IdleSample, error) {
	// Sample confirms a reading at most one second old, forcing a fresh interface
	// measure under the per-transact lease when stale (the same path as the
	// `hosting observe` consumer). The monotonic projection denominator excludes
	// the boundary sampling calls regardless of reading freshness.
	_ = fresh
	// The runtime's periodic refresh briefly holds the same per-transact lease;
	// a one-second Sample can transiently lose the transfer race. Retry briefly
	// rather than drop a cadence sample (the series gap bound is 1500 ms).
	var snapshot hosting.Snapshot
	var err error
	for attempt := 0; ; attempt++ {
		snapshot, err = budget.Sample(ctx, time.Second)
		if err == nil || ctx.Err() != nil || attempt >= 4 {
			break
		}
		select {
		case <-ctx.Done():
			return net32IdleSample{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if err != nil {
		return net32IdleSample{}, err
	}
	if snapshot.Observation().Drain {
		return net32IdleSample{}, errors.New("NET-32 hosting allowance requires drain")
	}
	state, err := readNet32HostingState(hostingRoot)
	if err != nil {
		return net32IdleSample{}, err
	}
	s := net32IdleSample{at: snapshot.ObservedAt(), hostingAt: snapshot.ObservedAt(), used: state.Used, boot: state.Reading.Boot}
	for _, iface := range state.Reading.Interfaces {
		s.ifaces = append(s.ifaces, net32IdleInterface{name: iface.Name, index: iface.Index, tx: iface.Tx, rx: iface.Rx})
	}
	for _, cgroup := range cgroups {
		rss, cpu, err := net32CgroupUsage(cgroup)
		if err != nil {
			return net32IdleSample{}, err
		}
		s.rss += rss
		s.cpuUsec += cpu
	}
	return s, nil
}

type net32HostingState struct {
	Schema   string `json:"schema"`
	Used     uint64 `json:"used"`
	Reserved uint64 `json:"reserved"`
	Reading  struct {
		Boot       string `json:"boot"`
		Interfaces []struct {
			Name  string `json:"name"`
			Index uint64 `json:"index"`
			Tx    uint64 `json:"tx"`
			Rx    uint64 `json:"rx"`
		} `json:"interfaces"`
	} `json:"reading"`
}

func readNet32HostingState(hostingRoot string) (net32HostingState, error) {
	raw, err := os.ReadFile(filepath.Join(hostingRoot, "budget.json"))
	if err != nil {
		return net32HostingState{}, err
	}
	// budget.json is the Hosting owner's checksummed envelope {state, sha256}.
	var envelope struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return net32HostingState{}, err
	}
	digest := sha256.Sum256(envelope.State)
	if envelope.SHA256 != hex.EncodeToString(digest[:]) {
		return net32HostingState{}, errors.New("NET-32 hosting state checksum differs")
	}
	var state net32HostingState
	if err := json.Unmarshal(envelope.State, &state); err != nil {
		return net32HostingState{}, err
	}
	if state.Schema != "ardents-hosting-budget-v1" || state.Reading.Boot == "" || len(state.Reading.Interfaces) == 0 {
		return net32HostingState{}, errors.New("NET-32 hosting reading unavailable")
	}
	return state, nil
}

// net32ClientCgroups returns the whole-client cgroup directories: the Endpoint
// service cgroup (from MainPID) plus every active confined text worker unit. The
// 512 MiB gate is whole-client, so confinement helpers/workers are included.
func net32ClientCgroups(mainPID string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join("/proc", mainPID, "cgroup"))
	if err != nil {
		return nil, err
	}
	var endpoint string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			endpoint = filepath.Join("/sys/fs/cgroup", parts[2])
		}
	}
	if endpoint == "" {
		return nil, errors.New("NET-32 Endpoint cgroup unavailable")
	}
	cgroups := []string{endpoint}
	entries, err := os.ReadDir("/sys/fs/cgroup/system.slice")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "ardents-text-reader@") || strings.HasPrefix(name, "ardents-text-publisher@") {
			cgroups = append(cgroups, filepath.Join("/sys/fs/cgroup/system.slice", name))
		}
	}
	return cgroups, nil
}

func net32CgroupUsage(cgroup string) (rss, cpuUsec uint64, err error) {
	memory, err := os.ReadFile(filepath.Join(cgroup, "memory.current"))
	if err != nil {
		return 0, 0, err
	}
	rss, err = strconv.ParseUint(strings.TrimSpace(string(memory)), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	cpu, err := os.ReadFile(filepath.Join(cgroup, "cpu.stat"))
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(cpu), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "usage_usec" {
			cpuUsec, err = strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		}
	}
	return rss, cpuUsec, nil
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
	complete := report.MeasuredDuration >= 10*time.Minute && len(series) >= 600
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
		if projected <= math.MaxUint64 {
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

func net32StateProfileBytes(root string) (uint64, error) {
	var total uint64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("NET-32 State profile contains a non-regular entry")
		}
		if info.Mode().IsRegular() {
			if info.Size() < 0 || uint64(info.Size()) > (64<<10)-total {
				return errors.New("NET-32 State profile exceeds 64 KiB")
			}
			total += uint64(info.Size())
		}
		return nil
	})
	return total, err
}
