package serviceconn

import (
	"os"
	"runtime"
	"runtime/metrics"
	"time"
)

type resourceSample struct {
	memory, cpu       float64
	files, goroutines uint32
	queuedBytes       uint32
}

type resourceMonitor struct {
	stopSignal chan struct{}
	done       chan resourceSample
}

func startResourceMonitor(observe func(string, int) uint32) resourceMonitor {
	monitor := resourceMonitor{stopSignal: make(chan struct{}), done: make(chan resourceSample, 1)}
	go func() {
		releaseTimer := acquireResource(observe, "timer")
		defer releaseTimer()
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

func (endpoint *endpoint) observe(result *Result, input Request, resources resourceSample) {
	activeSessions := endpoint.admission.Active()
	switch input.Action {
	case "connect", "accept":
		result.GrantSurface = "connection"
	case "publish", "unpublish":
		result.GrantSurface = "administration"
	default:
		result.GrantSurface = input.Surface
	}
	result.MemoryHighWater = uint64(resources.memory)
	result.GoroutinesHighWater = resources.goroutines
	result.OpenFilesHighWater = resources.files
	result.CPUSeconds = resources.cpu
	result.TimerHighWater = endpoint.resources("timer", 0)
	result.AcceptedIPCHighWater = endpoint.resources("accepted-ipc", 0)
	result.ServiceConnectionsHighWater = endpoint.resources("service-connection", 0)
	result.ControlFilesHighWater = endpoint.resources("control-file", 0)
	if resources.queuedBytes > result.QueueHighWater {
		result.QueueHighWater = resources.queuedBytes
	}
	result.ActiveSessions = activeSessions
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
	if right.queuedBytes > left.queuedBytes {
		left.queuedBytes = right.queuedBytes
	}
	return left
}

func processCPUSeconds() float64 {
	samples := []metrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() == metrics.KindFloat64 {
		return samples[0].Value.Float64()
	}
	return 0
}
