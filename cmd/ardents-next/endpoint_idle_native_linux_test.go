//go:build linux && endpoint_launch_native

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	work, termination := net32IdleTraffic(window)
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
	return collectNet32Idle(lifetime, window, time.Now, func(fresh bool) (net32IdleSample, error) {
		return readNet32Sample(lifetime, budget, hostingRoot, cgroups, fresh)
	})
}

// readNet32Sample forces a fresh charged measurement at either boundary. It
// reads the confirmed persisted counters for the interface/ledger comparison.
func readNet32Sample(ctx context.Context, budget *hosting.Budget, hostingRoot string, cgroups []string, fresh bool) (net32IdleSample, error) {
	// The runtime's periodic refresh briefly holds the same per-transact lease.
	var observation hosting.Observation
	var err error
	for attempt := 0; ; attempt++ {
		if fresh {
			observation, err = budget.Observe(ctx)
		} else {
			var snapshot hosting.Snapshot
			snapshot, err = budget.Sample(ctx, time.Second)
			observation = snapshot.Observation()
		}
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
	if observation.Drain {
		return net32IdleSample{}, errors.New("NET-32 hosting allowance requires drain")
	}
	state, err := readNet32HostingState(hostingRoot)
	if err != nil {
		return net32IdleSample{}, err
	}
	s := net32IdleSample{hostingAt: state.Observed, used: state.Used, boot: state.Reading.Boot}
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
	Schema   string    `json:"schema"`
	Observed time.Time `json:"observed"`
	Used     uint64    `json:"used"`
	Reserved uint64    `json:"reserved"`
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
	if state.Schema != "ardents-hosting-budget-v1" || state.Observed.IsZero() || state.Reading.Boot == "" || len(state.Reading.Interfaces) == 0 {
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

func TestNET32IdleFreshBoundaryBypassesRecentHostingSample(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hosting")
	now := time.Now().UTC().Truncate(time.Second)
	if err := hosting.Initialize(root, hosting.Policy{
		Provider: "NET-32 fresh boundary fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "MiB", Quantity: 64, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1,
	}); err != nil {
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
	initial, err := budget.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := readNet32Sample(t.Context(), budget, root, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.hostingAt.After(initial.ObservedAt()) {
		t.Fatalf("boundary reused the recent persisted sample: initial %v fresh %v", initial.ObservedAt(), fresh.hostingAt)
	}
}
