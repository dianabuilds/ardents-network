package node

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type nodeResourceSnapshot struct {
	Service        string            `json:"service"`
	At             time.Time         `json:"at"`
	CPUUsageUsec   uint64            `json:"cpu_usage_usec"`
	Memory         uint64            `json:"memory_current"`
	PIDs           uint64            `json:"pids_current"`
	FDs            uint64            `json:"fds"`
	Sockets        uint64            `json:"sockets"`
	Threads        uint64            `json:"threads"`
	RSS            uint64            `json:"rss_bytes"`
	ContainerID    string            `json:"container_id"`
	HostPID        int               `json:"host_pid"`
	ProcessStart   string            `json:"process_start"`
	TickDelayNanos int64             `json:"tick_delay_ns"`
	Raw            map[string]string `json:"raw"`
	Events         map[string]uint64 `json:"memory_events"`
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

func (observer *nodeObserver) observeResources(samples []nodeResourceSnapshot, faults map[string]bool) {
	present := make(map[string]bool, len(samples))
	for _, sample := range samples {
		present[sample.Service] = true
	}
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		series := observer.resources[service]
		if series == nil {
			series = new(nodeResourceSeries)
			observer.resources[service] = series
		}
		if faults["absence:"+service] {
			series.lastAt = time.Time{}
		} else if !present[service] {
			series.cadenceFailure = true
		}
	}
	for _, sample := range samples {
		observer.observeResource(sample, faults)
	}
}

func (observer *nodeObserver) observeResource(sample nodeResourceSnapshot, faults map[string]bool) {
	series := observer.resources[sample.Service]
	if series == nil {
		series = new(nodeResourceSeries)
		observer.resources[sample.Service] = series
	}
	delay := time.Duration(sample.TickDelayNanos)
	series.cadenceFailure = series.cadenceFailure || delay < -50*time.Millisecond || delay > 50*time.Millisecond
	identity := sample.ContainerID + ":" + sample.ProcessStart
	sameProcess := series.identity == "" || series.identity == identity
	if !sameProcess {
		if !faults["absence:"+sample.Service] && !series.lastAt.IsZero() {
			series.cadenceFailure = true
		}
		series.lastAt, series.lastCPU = time.Time{}, 0
	}
	if !series.lastAt.IsZero() && sameProcess {
		gap := sample.At.Sub(series.lastAt)
		series.cadenceFailure = series.cadenceFailure || gap < 950*time.Millisecond || gap > 1050*time.Millisecond ||
			sample.CPUUsageUsec < series.lastCPU
		if elapsed := gap.Microseconds(); elapsed > 0 && sample.CPUUsageUsec >= series.lastCPU {
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
	series.eventFailure = series.eventFailure || sample.Events["max"] > 0 || sample.Events["oom"] > 0 || sample.Events["oom_kill"] > 0
	if !validNodeResourceProfile(sample) && !faults["cgroup-drift:"+sample.Service] {
		series.profileFailure = true
	}
}

func (observer *nodeObserver) verifyResourceEvidence() error {
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		if err := verifyNodeResourceSeries(service, observer.resources[service]); err != nil {
			return err
		}
	}
	return nil
}

func verifyNodeResourceSeries(service string, series *nodeResourceSeries) error {
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
