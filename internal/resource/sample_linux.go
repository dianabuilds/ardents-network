//go:build linux

package resource

import (
	"errors"
	"io"
	"os"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
)

func sampleProcess(profile profile) (Sample, error) {
	files := make(map[string]string, 7)
	for _, name := range []string{"cpu.pressure", "cpu.stat", "io.pressure", "memory.current", "memory.events.local", "memory.pressure", "memory.stat"} {
		raw, err := boundedFile("/sys/fs/cgroup/"+name, 4096)
		if err != nil {
			return Sample{}, err
		}
		files[name] = raw
	}
	cpu, cpuErr := counter(files["cpu.stat"], "usage_usec")
	memory, memoryErr := strconv.ParseUint(strings.TrimSpace(files["memory.current"]), 10, 64)
	socket, socketErr := counter(files["memory.stat"], "sock")
	high, highErr := counter(files["memory.events.local"], "high")
	maximum, maxErr := counter(files["memory.events.local"], "max")
	oom, oomErr := counter(files["memory.events.local"], "oom")
	kill, killErr := counter(files["memory.events.local"], "oom_kill")
	fds, fdErr := directoryCount("/proc/self/fd", profile.maximumFDs)
	threads, threadErr := directoryCount("/proc/self/task", profile.maximumThreads)
	goMemory, goErr := managedMemory()
	cpuPSI, cpuPSIErr := pressureAverage(files["cpu.pressure"], "some")
	memoryPSI, memoryPSIErr := pressureAverage(files["memory.pressure"], "some")
	ioPSI, ioPSIErr := pressureAverage(files["io.pressure"], "full")
	err := errors.Join(cpuErr, memoryErr, socketErr, highErr, maxErr, oomErr, killErr, fdErr, threadErr,
		goErr, cpuPSIErr, memoryPSIErr, ioPSIErr)
	return Sample{CPUUsageUsec: cpu, MemoryBytes: memory, GoMemoryBytes: goMemory, SocketMemoryBytes: socket,
		FDs: fds, Goroutines: uint64(runtime.NumGoroutine()), Threads: threads, CPUPressure: cpuPSI,
		MemoryPressure: memoryPSI, IOPressure: ioPSI, HighEvents: high, EmergencyEvents: maximum + oom + kill}, err
}

func counter(raw, name string) (uint64, error) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, errors.New("resource guard counter is missing")
}

func pressureAverage(raw, kind string) (float64, error) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == kind && strings.HasPrefix(fields[1], "avg10=") {
			return strconv.ParseFloat(strings.TrimPrefix(fields[1], "avg10="), 64)
		}
	}
	return 0, errors.New("resource guard pressure counter is missing")
}

func directoryCount(path string, maximum int) (uint64, error) {
	directory, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	names, readErr := directory.Readdirnames(maximum + 1)
	closeErr := directory.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if len(names) > maximum {
		return 0, errors.New("process resource exceeds its profile bound")
	}
	return uint64(len(names)), errors.Join(readErr, closeErr)
}

func managedMemory() (uint64, error) {
	samples := []metrics.Sample{{Name: "/memory/classes/total:bytes"}, {Name: "/memory/classes/heap/released:bytes"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindUint64 || samples[1].Value.Kind() != metrics.KindUint64 {
		return 0, errors.New("Go memory counter is unavailable")
	}
	total, released := samples[0].Value.Uint64(), samples[1].Value.Uint64()
	if released > total {
		return 0, errors.New("Go memory counter is invalid")
	}
	return total - released, nil
}

func boundedFile(path string, maximum int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	buffer := make([]byte, maximum+1)
	count, readErr := file.Read(buffer)
	closeErr := file.Close()
	if readErr != nil && count == 0 {
		return "", errors.Join(readErr, closeErr)
	}
	if count > maximum {
		return "", errors.New("resource counter exceeds its bound")
	}
	return string(buffer[:count]), closeErr
}
