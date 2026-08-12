package node

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (observer *nodeObserver) sampleNodeResources(ctx context.Context, _ time.Time) ([]nodeResourceSnapshot, error) {
	services := []string{"source1", "source2", "endpoint", "node1", "node2"}
	identities, err := observer.composeBounded(ctx, 4096, "ps", "--format", "{{.Service}}\t{{.ID}}")
	if err != nil {
		return nil, err
	}
	ids := make(map[string]string, len(services))
	for _, line := range strings.Split(string(identities), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && len(fields[1]) >= 12 && len(fields[1]) <= 64 {
			ids[fields[0]] = fields[1]
		}
	}
	arguments := []string{"inspect", "--format", "{{.Id}}\t{{.State.Pid}}"}
	for _, service := range services {
		if ids[service] != "" {
			arguments = append(arguments, ids[service])
		}
	}
	if len(arguments) == 3 {
		return nil, errors.New("node resource candidate set is empty")
	}
	inspected, err := observer.dockerBounded(ctx, 8192, 4096, arguments...)
	if err != nil {
		return nil, err
	}
	candidates := make([]nodeHostCandidate, 0, len(services))
	for _, service := range services {
		if ids[service] == "" {
			continue
		}
		candidate, found, candidateErr := nodeCandidateFromInspect(service, ids[service], string(inspected))
		if candidateErr != nil {
			return nil, candidateErr
		}
		if !found {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return []nodeResourceSnapshot{}, nil
	}
	payload, err := json.Marshal(candidates)
	if err != nil || len(payload) > 4096 {
		return nil, errors.Join(err, errors.New("node resource collector input exceeds its bound"))
	}
	if len(observer.collectorID) < 12 {
		return nil, errors.New("node resource collector is unavailable")
	}
	raw, err := observer.dockerBounded(ctx, 384<<10, 4096, "exec", observer.collectorID,
		"/usr/local/bin/ardents-qualify", "sample-node", string(payload))
	var samples []nodeResourceSnapshot
	if err != nil || json.Unmarshal(raw, &samples) != nil || len(samples) > len(candidates) {
		return nil, errors.Join(err, errors.New("node host resource sample is invalid"))
	}
	return samples, nil
}

type nodeResourceSnapshot struct {
	Service      string            `json:"service"`
	At           time.Time         `json:"at"`
	CPUUsageUsec uint64            `json:"cpu_usage_usec"`
	Memory       uint64            `json:"memory_current"`
	PIDs         uint64            `json:"pids_current"`
	FDs          uint64            `json:"fds"`
	Sockets      uint64            `json:"sockets"`
	Threads      uint64            `json:"threads"`
	RSS          uint64            `json:"rss_bytes"`
	ContainerID  string            `json:"container_id"`
	HostPID      int               `json:"host_pid"`
	ProcessStart string            `json:"process_start"`
	Raw          map[string]string `json:"raw"`
	Events       map[string]uint64 `json:"memory_events"`
}

type nodeHostCandidate struct {
	Service     string `json:"service"`
	ContainerID string `json:"container_id"`
	PID         int    `json:"pid"`
}

type nodeResourceSeries struct {
	lastAt         time.Time
	lastCPU        uint64
	cpuMilli       []uint64
	memory         []uint64
	maxPIDs        uint64
	maxFDs         uint64
	maxSockets     uint64
	maxThreads     uint64
	maxRSS         uint64
	identity       string
	eventFailure   bool
	profileFailure bool
	cadenceFailure bool
}

func (observer *nodeObserver) observeResources(samples []nodeResourceSnapshot) {
	present := make(map[string]bool, len(samples))
	for _, sample := range samples {
		present[sample.Service] = true
	}
	for service, series := range observer.resources {
		if !present[service] {
			series.lastAt = time.Time{}
		}
	}
	for _, sample := range samples {
		series := observer.resources[sample.Service]
		if series == nil {
			series = new(nodeResourceSeries)
			observer.resources[sample.Service] = series
		}
		identity := sample.ContainerID + ":" + sample.ProcessStart
		sameProcess := series.identity == "" || series.identity == identity
		if !sameProcess {
			series.lastAt, series.lastCPU = time.Time{}, 0
		}
		if !series.lastAt.IsZero() && sameProcess && sample.CPUUsageUsec >= series.lastCPU {
			gap := sample.At.Sub(series.lastAt)
			if gap > 1500*time.Millisecond {
				series.cadenceFailure = true
			}
			elapsed := gap.Microseconds()
			if elapsed > 0 {
				series.cpuMilli = append(series.cpuMilli, (sample.CPUUsageUsec-series.lastCPU)*1000/uint64(elapsed))
			}
		}
		series.lastAt, series.lastCPU, series.identity = sample.At, sample.CPUUsageUsec, identity
		series.memory = append(series.memory, sample.Memory)
		series.maxPIDs = max(series.maxPIDs, sample.PIDs)
		series.maxFDs = max(series.maxFDs, sample.FDs)
		series.maxSockets = max(series.maxSockets, sample.Sockets)
		series.maxThreads = max(series.maxThreads, sample.Threads)
		series.maxRSS = max(series.maxRSS, sample.RSS)
		if sample.Events["max"] > 0 || sample.Events["oom"] > 0 || sample.Events["oom_kill"] > 0 {
			series.eventFailure = true
		}
		if !validNodeResourceProfile(sample) && !observer.faultSnapshot()["cgroup-drift:"+sample.Service] {
			series.profileFailure = true
		}
	}
}

func (observer *nodeObserver) verifyResourceEvidence() error {
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		series := observer.resources[service]
		if series == nil || len(series.cpuMilli) < 3 || len(series.memory) < 3 {
			return invalidNodeCampaign(errors.New("node resource evidence is incomplete for " + service))
		}
		meanLimit, p95Limit := uint64(1120), uint64(1280)
		if service == "node1" || service == "node2" {
			meanLimit, p95Limit = 650, 800
		}
		var total uint64
		for _, value := range series.cpuMilli {
			total += value
		}
		if total/uint64(len(series.cpuMilli)) > meanLimit || nearestRank95(series.cpuMilli) > p95Limit {
			return errors.New("node CPU gate failed for " + service)
		}
		if service != "node1" && service != "node2" && nearestRank95(series.memory) > 896<<20 {
			return errors.New("node H3-S memory gate failed for " + service)
		}
		fdLimit, socketLimit, pidLimit := uint64(256), uint64(56), uint64(512)
		if service == "node1" || service == "node2" {
			fdLimit, socketLimit, pidLimit = 512, 256, 256
		}
		if series.cadenceFailure {
			return invalidNodeCampaign(errors.New("node external sampling cadence failed for " + service))
		}
		if series.eventFailure || series.profileFailure || series.maxFDs > fdLimit ||
			series.maxSockets > socketLimit || series.maxPIDs > pidLimit || series.maxThreads > pidLimit {
			return errors.New("node process resource fuse failed for " + service)
		}
	}
	return nil
}

func validNodeResourceProfile(sample nodeResourceSnapshot) bool {
	expected := map[string]string{"memory.swap.current": "0", "memory.swap.max": "0"}
	if sample.Service == "node1" || sample.Service == "node2" {
		expected["cpu.max"], expected["memory.max"], expected["pids.max"] = "100000 100000", "536870912", "256"
	} else {
		expected["cpu.max"], expected["memory.low"] = "160000 100000", "1207959552"
		expected["memory.max"], expected["pids.max"] = "1342177280", "512"
	}
	for name, want := range expected {
		if strings.TrimSpace(sample.Raw[name]) != want {
			return false
		}
	}
	ancestry := strings.TrimSpace(sample.Raw["proc.cgroup"])
	mounts := sample.Raw["proc.mountinfo"]
	return strings.HasPrefix(ancestry, "0::/") && strings.Contains(mounts, " - cgroup2 cgroup ") &&
		strings.TrimSpace(sample.Raw["cpuset.cpus.effective"]) != "" && nodeNetworkCountersHealthy(sample.Raw["net.dev"])
}

func nodeNetworkCountersHealthy(raw string) bool {
	seen := false
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 17 || !strings.HasSuffix(fields[0], ":") {
			continue
		}
		seen = true
		for _, index := range []int{3, 4, 11, 12} {
			value, err := strconv.ParseUint(fields[index], 10, 64)
			if err != nil || value != 0 {
				return false
			}
		}
	}
	return seen
}

func (observer *nodeObserver) captureInitialResources(ctx context.Context) error {
	samples, err := observer.sampleNodeResources(ctx, time.Now())
	if err != nil || len(samples) != 5 {
		return errors.Join(err, errors.New("node initial resource oracle did not observe every candidate"))
	}
	seen := make(map[string]bool, 5)
	for _, sample := range samples {
		seen[sample.Service] = true
	}
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		if !seen[service] {
			return errors.New("node initial resource oracle missed " + service)
		}
	}
	observer.observeResources(samples)
	return byteio.WriteJSON(filepath.Join(observer.input.EvidenceRoot, "resources-initial.json"), samples, 64<<10)
}
