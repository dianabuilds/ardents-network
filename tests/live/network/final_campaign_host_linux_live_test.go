//go:build live && linux

package network_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func finalCampaignHostAllocation(hosts []finalRunnerObservedHost) ([]finalRunnerObservedHost, error) {
	if !validFinalRunnerHostAllocation(hosts) {
		return nil, errors.New("frozen final campaign host allocation is invalid")
	}
	memory, err := finalLinuxMemoryMiB()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return nil, errors.New("final campaign requires cgroup v2")
	}
	swap, err := os.ReadFile("/sys/fs/cgroup/memory.swap.current")
	if err != nil {
		return nil, errors.New("final campaign swap state is unavailable")
	}
	swapBytes, err := strconv.ParseUint(strings.TrimSpace(string(swap)), 10, 64)
	if err != nil || swapBytes != 0 {
		return nil, errors.New("final campaign cgroup has nonzero swap")
	}
	machine, err := os.ReadFile("/etc/machine-id")
	if err != nil || strings.TrimSpace(string(machine)) == "" {
		return nil, errors.New("final campaign host identity is unavailable")
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(string(machine))))
	if hosts[0].ID != "sha256:"+hex.EncodeToString(digest[:]) || hosts[0].LogicalCPUs != uint16(runtime.NumCPU()) ||
		hosts[0].MemoryMiB != memory {
		return nil, errors.New("frozen final campaign host identity or capacity differs from the runner host")
	}
	return hosts, nil
}

func finalLinuxMemoryMiB() (uint32, error) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "MemTotal:" && fields[2] == "kB" {
			value, parseErr := strconv.ParseUint(fields[1], 10, 32)
			return uint32(value / 1_024), parseErr
		}
	}
	return 0, errors.New("final campaign host memory is unavailable")
}
