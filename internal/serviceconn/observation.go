package serviceconn

import (
	"crypto/sha256"
	"os"
	"runtime"
	"runtime/metrics"
	"time"
)

type resourceSample struct {
	memory, cpu       float64
	files, goroutines uint32
}

type resourceMonitor struct {
	stopSignal chan struct{}
	done       chan resourceSample
}

func startResourceMonitor() resourceMonitor {
	monitor := resourceMonitor{stopSignal: make(chan struct{}), done: make(chan resourceSample, 1)}
	go func() {
		started, peak := sampleResources(), sampleResources()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				peak = higherResources(peak, sampleResources())
			case <-monitor.stopSignal:
				peak = higherResources(peak, sampleResources())
				peak.cpu -= started.cpu
				if peak.cpu < 0 {
					peak.cpu = 0
				}
				monitor.done <- peak
				return
			}
		}
	}()
	return monitor
}

func (monitor resourceMonitor) stop() resourceSample {
	close(monitor.stopSignal)
	return <-monitor.done
}

func (endpoint *Endpoint) observe(result *Result, input Request, resources resourceSample) {
	result.PrincipalCommitment = commitment("principal", input.Principal)
	if input.Session != [32]byte{} {
		result.SessionCommitment = commitment("session", input.Session)
	}
	switch input.Action {
	case "connect", "accept":
		result.GrantSurface = "connection"
	case "publish", "unpublish":
		result.GrantSurface = "administration"
	default:
		result.GrantSurface = input.Surface
	}
	result.SessionConsumed = input.Action != "admit" && input.Session != [32]byte{}
	result.MemoryHighWater = uint64(resources.memory)
	result.GoroutinesHighWater = resources.goroutines
	result.OpenFilesHighWater = resources.files
	result.CPUSeconds = resources.cpu
	result.TimerHighWater = 1
	if input.Action == "connect" || input.Action == "accept" {
		result.QueueHighWater = 2
	}
	endpoint.mu.Lock()
	result.ActiveSessions = uint32(len(endpoint.sessions))
	endpoint.mu.Unlock()
}

func sampleResources() resourceSample {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	sample := resourceSample{memory: float64(memory.Sys), cpu: processCPUSeconds(),
		goroutines: uint32(runtime.NumGoroutine())}
	if descriptors, err := os.ReadDir("/proc/self/fd"); err == nil {
		sample.files = uint32(len(descriptors))
	}
	return sample
}

func higherResources(left, right resourceSample) resourceSample {
	if right.memory > left.memory {
		left.memory = right.memory
	}
	if right.cpu > left.cpu {
		left.cpu = right.cpu
	}
	if right.files > left.files {
		left.files = right.files
	}
	if right.goroutines > left.goroutines {
		left.goroutines = right.goroutines
	}
	return left
}

func commitment(kind string, value [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-service-"+kind+"-v1\x00"), value[:]...))
}

func processCPUSeconds() float64 {
	samples := []metrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() == metrics.KindFloat64 {
		return samples[0].Value.Float64()
	}
	return 0
}
