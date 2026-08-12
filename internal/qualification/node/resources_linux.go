//go:build linux

package node

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

// SampleContainerResources captures one bounded Linux snapshot from inside a candidate container.
func SampleContainerResources(at time.Time) ([]byte, error) {
	sample, err := readNodeProcessResources("", "", 1, at)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(sample)
	if err != nil || len(raw) > 64<<10 {
		return nil, errors.Join(err, errors.New("node container resource sample exceeds its bound"))
	}
	return raw, nil
}

func readNodeProcessResources(service, identity string, pid int, at time.Time) (nodeResourceSnapshot, error) {
	root := filepath.Join("/proc", strconv.Itoa(pid), "root", "sys", "fs", "cgroup")
	raw := make(map[string]string, 18)
	for _, name := range []string{"cpu.max", "cpu.max.burst", "cpu.pressure", "cpu.stat", "cpuset.cpus.effective",
		"io.pressure", "memory.current", "memory.events.local", "memory.low", "memory.max", "memory.peak",
		"memory.pressure", "memory.stat", "memory.swap.current", "memory.swap.max", "pids.current", "pids.max"} {
		value, err := byteio.ReadFile(filepath.Join(root, name), 16<<10)
		if err != nil {
			return nodeResourceSnapshot{}, err
		}
		raw[name] = string(value)
	}
	usage, err := nodeNamedCounter(raw["cpu.stat"], "usage_usec")
	memory, memoryErr := strconv.ParseUint(strings.TrimSpace(raw["memory.current"]), 10, 64)
	pids, pidsErr := strconv.ParseUint(strings.TrimSpace(raw["pids.current"]), 10, 64)
	events, eventErr := nodeCounterMap(raw["memory.events.local"])
	networkRaw, networkErr := byteio.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "net", "dev"), 16<<10)
	raw["net.dev"] = string(networkRaw)
	fds, sockets, threads, rss, start, processRaw, processErr := nodeProcessTree(pid)
	raw["process_tree"] = processRaw
	if err = errors.Join(err, memoryErr, pidsErr, eventErr, networkErr, processErr); err != nil {
		return nodeResourceSnapshot{}, err
	}
	return nodeResourceSnapshot{Service: service, At: at.UTC(), CPUUsageUsec: usage, Memory: memory,
		PIDs: pids, FDs: fds, Sockets: sockets, Threads: threads, RSS: rss, ContainerID: identity,
		HostPID: pid, ProcessStart: start, Raw: raw, Events: events}, nil
}

func nodeNamedCounter(raw, name string) (uint64, error) {
	values, err := nodeCounterMap(raw)
	value, found := values[name]
	if err != nil || !found {
		return 0, errors.New("node cgroup counter is missing or invalid")
	}
	return value, nil
}

func nodeCounterMap(raw string) (map[string]uint64, error) {
	values := make(map[string]uint64)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(values) >= 32 {
			return nil, errors.New("node cgroup counter set is invalid")
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, err
		}
		values[fields[0]] = value
	}
	return values, nil
}

func nodeFDCounts(pid int) (uint64, uint64, error) {
	root := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	directory, err := os.Open(root)
	if err != nil {
		return 0, 0, err
	}
	names, readErr := directory.Readdirnames(513)
	closeErr := directory.Close()
	if len(names) > 512 {
		return 0, 0, errors.New("node process exceeded its FD evidence bound")
	}
	var descriptors, sockets uint64
	for _, name := range names {
		target, linkErr := os.Readlink(filepath.Join(root, name))
		if errors.Is(linkErr, os.ErrNotExist) {
			continue
		}
		if linkErr != nil {
			return 0, 0, linkErr
		}
		descriptors++
		if strings.HasPrefix(target, "socket:[") {
			sockets++
		}
	}
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	return descriptors, sockets, errors.Join(readErr, closeErr)
}
