package serviceconn

import (
	"crypto/sha256"
	"os"
	"runtime"
	"runtime/metrics"
	"strings"
	"time"
)

type resourceSample struct {
	memory, cpu                      float64
	files, goroutines                uint32
	timers, queuedBytes, tempEntries uint32
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
	endpoint.mu.Lock()
	session, consumed := endpoint.consumed[input.Session]
	delete(endpoint.consumed, input.Session)
	activeSessions := uint32(len(endpoint.sessions))
	endpoint.mu.Unlock()
	if consumed {
		result.PrincipalCommitment = commitment("principal", session.principal)
		result.SessionCommitment = commitment("session", input.Session)
		result.BrokerCommitment = commitment("broker", session.broker)
		result.GrantCommitment = grantCommitment(session.broker, session.principal, session.surface)
		result.GrantSurface = session.surface
		result.SessionConsumed = true
		result.SessionIssuedAt, result.SessionExpiresAt = session.issued, session.expires
	}
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
	result.TimerHighWater = resources.timers
	if resources.queuedBytes > result.QueueHighWater {
		result.QueueHighWater = resources.queuedBytes
	}
	result.TempEntries = resources.tempEntries
	result.ActiveSessions = activeSessions
}

func sampleResources() resourceSample {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	sample := resourceSample{memory: float64(memory.Sys), cpu: processCPUSeconds(),
		goroutines: uint32(runtime.NumGoroutine()), timers: 1, tempEntries: ownedTempEntries()}
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
	if right.timers > left.timers {
		left.timers = right.timers
	}
	if right.queuedBytes > left.queuedBytes {
		left.queuedBytes = right.queuedBytes
	}
	if right.tempEntries > left.tempEntries {
		left.tempEntries = right.tempEntries
	}
	return left
}

func ownedTempEntries() uint32 {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	var count uint32
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ardents-service-") {
			count++
		}
	}
	return count
}

func commitment(kind string, value [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-service-"+kind+"-v1\x00"), value[:]...))
}

func grantCommitment(broker, principal [32]byte, surface string) [32]byte {
	value := append([]byte("ardents-h3-service-grant-v1\x00"), broker[:]...)
	value = append(value, principal[:]...)
	value = append(value, surface...)
	return sha256.Sum256(value)
}

func processCPUSeconds() float64 {
	samples := []metrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() == metrics.KindFloat64 {
		return samples[0].Value.Float64()
	}
	return 0
}
