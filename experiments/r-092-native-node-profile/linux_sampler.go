//go:build ignore

package main

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type hostIdentity struct {
	OperatingSystem string `json:"operating_system"`
	Architecture    string `json:"architecture"`
	GoVersion       string `json:"go_version"`
	KernelRelease   string `json:"kernel_release,omitempty"`
	LogicalCPUs     int    `json:"logical_cpus"`
	MemoryBytes     int64  `json:"memory_bytes,omitempty"`
}

func currentHostIdentity() (hostIdentity, error) {
	result := hostIdentity{OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, GoVersion: runtime.Version(),
		LogicalCPUs: runtime.NumCPU()}
	if runtime.GOOS != "linux" {
		return result, nil
	}
	kernel, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return hostIdentity{}, err
	}
	result.KernelRelease = strings.TrimSpace(string(kernel))
	if result.KernelRelease == "" {
		return hostIdentity{}, errors.New("Linux kernel release is absent")
	}
	meminfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return hostIdentity{}, err
	}
	for _, line := range strings.Split(string(meminfo), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "kB" {
			return hostIdentity{}, errors.New("Linux memory observation is invalid")
		}
		memory, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr != nil || memory <= 0 {
			return hostIdentity{}, errors.New("Linux memory observation is invalid")
		}
		result.MemoryBytes = memory * 1024
		return result, nil
	}
	return hostIdentity{}, errors.New("Linux memory observation is absent")
}

type linuxSampler struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	samples []linuxSample
	err     error
	stop    sync.Once
}

func newLinuxSampler(parent context.Context, interval time.Duration) *linuxSampler {
	ctx, cancel := context.WithCancel(parent)
	result := &linuxSampler{cancel: cancel, done: make(chan struct{})}
	go result.run(ctx, interval)
	return result
}

func (sampler *linuxSampler) run(ctx context.Context, interval time.Duration) {
	defer close(sampler.done)
	sampler.capture()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sampler.capture()
		case <-ctx.Done():
			sampler.capture()
			return
		}
	}
}

func (sampler *linuxSampler) capture() {
	sample, err := sampleLinux()
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	if err != nil && sampler.err == nil {
		sampler.err = err
	}
	if sample != nil {
		sampler.samples = append(sampler.samples, *sample)
	}
}

func (sampler *linuxSampler) Stop() ([]linuxSample, error) {
	sampler.stop.Do(sampler.cancel)
	<-sampler.done
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	return append([]linuxSample(nil), sampler.samples...), sampler.err
}
