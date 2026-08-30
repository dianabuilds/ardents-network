package resource

import "time"

type level byte

const (
	levelNormal level = iota
	levelProtect
	levelDrain
)

type limits struct {
	cpu, memory, goMemory, socketMemory     uint64
	storage                                 uint64
	sockets                                 uint64
	fds, goroutines, threads, timers        uint64
	queueItems, queueBytes                  uint64
	cpuPressure, memoryPressure, ioPressure float64
}

type profile struct {
	name                       string
	maximumFDs, maximumThreads int
	high, low, emergency       limits
	goMaxProcs                 int
	goMemory                   uint64
	exactGoMemory              bool
	noFile                     uint64
	placementWait              time.Duration
	maximumStorageBytes        uint64
	maximumStorageFiles        int
	maximumStorageDirectories  int
	maximumStorageDepth        int
	cgroup                     map[string]string
}

var profiles = map[string]profile{
	RendezvousDedicatedHostProfile: {
		name: RendezvousDedicatedHostProfile, maximumFDs: 256, maximumThreads: 64,
		goMaxProcs: 1, goMemory: 128 << 20, exactGoMemory: true, noFile: 256, placementWait: 10 * time.Second,
		maximumStorageBytes: 384 << 20, maximumStorageFiles: 5000,
		maximumStorageDirectories: 5000, maximumStorageDepth: 32,
		cgroup: map[string]string{"cpu.max": "100000 100000", "memory.high": "201326592", "memory.max": "268435456", "pids.max": "64"},
		high: limits{cpu: 800_000, memory: 192 << 20, goMemory: 115 << 20, sockets: 16, fds: 205,
			goroutines: 128, threads: 32, timers: 8, socketMemory: 32 << 20, storage: 320 << 20,
			cpuPressure: 20, memoryPressure: 5, ioPressure: 1},
		low: limits{cpu: 600_000, memory: 160 << 20, goMemory: 96 << 20, sockets: 12, fds: 154,
			goroutines: 96, threads: 24, timers: 6, socketMemory: 16 << 20, storage: 256 << 20,
			cpuPressure: 10, memoryPressure: 2.5, ioPressure: .5},
		emergency: limits{memory: 240 << 20, sockets: 24, fds: 231, goroutines: 192, threads: 64,
			timers: 16, socketMemory: 64 << 20, storage: 384 << 20},
	},
	"h3-np1-v1": {
		name: "h3-np1-v1", maximumFDs: 512, maximumThreads: 256,
		goMaxProcs: 1, goMemory: 320 << 20, noFile: 512,
		cgroup: map[string]string{"cpu.max": "100000 100000", "memory.max": "536870912", "pids.max": "256"},
		high: limits{cpu: 800_000, memory: 384 << 20, goMemory: 288 << 20, fds: 410, goroutines: 410,
			threads: 64, timers: 410, queueItems: 410, queueBytes: (8 << 20) * 8 / 10, socketMemory: 128 << 20,
			cpuPressure: 20, memoryPressure: 5, ioPressure: 1},
		low: limits{cpu: 600_000, memory: 307 << 20, goMemory: 256 << 20, fds: 307, goroutines: 307,
			threads: 48, timers: 307, queueItems: 307, queueBytes: (8 << 20) * 6 / 10, socketMemory: 64 << 20,
			cpuPressure: 10, memoryPressure: 2.5, ioPressure: .5},
		emergency: limits{memory: 460 << 20, fds: 461, goroutines: 461, threads: 256,
			timers: 461, queueItems: 461, queueBytes: 8 << 20},
	},
	"h3-s-v1": {
		name: "h3-s-v1", maximumFDs: 4096, maximumThreads: 512,
		goMaxProcs: 2, goMemory: 768 << 20, exactGoMemory: true, noFile: 4096, placementWait: 10 * time.Second,
		cgroup: map[string]string{"cpu.max": "160000 100000", "memory.low": "1207959552", "memory.max": "1342177280", "pids.max": "512"},
		high: limits{cpu: 1_280_000, memory: 960 << 20, goMemory: 691 << 20, sockets: 26, fds: 205, goroutines: 820,
			threads: 64, timers: 205, socketMemory: 128 << 20, cpuPressure: 20, memoryPressure: 5, ioPressure: 1},
		low: limits{cpu: 960_000, memory: 896 << 20, goMemory: 614 << 20, sockets: 20, fds: 154, goroutines: 614,
			threads: 48, timers: 154, socketMemory: 64 << 20, cpuPressure: 10, memoryPressure: 2.5, ioPressure: .5},
		emergency: limits{memory: 1152 << 20, sockets: 29, fds: 231, goroutines: 922, threads: 256,
			timers: 231, socketMemory: 192 << 20},
	},
	"h3-s-v1-strong": {
		name: "h3-s-v1-strong", maximumFDs: 4096, maximumThreads: 512,
		goMaxProcs: 6, goMemory: 3072 << 20, exactGoMemory: true, noFile: 4096, placementWait: 10 * time.Second,
		cgroup: map[string]string{"cpu.max": "640000 100000", "memory.low": "4831838208",
			"memory.max": "5368709120", "pids.max": "2048"},
		high: limits{cpu: 5_120_000, memory: 3840 << 20, goMemory: 2764 << 20, sockets: 104, fds: 820,
			goroutines: 3280, threads: 256, timers: 820, socketMemory: 512 << 20,
			cpuPressure: 20, memoryPressure: 5, ioPressure: 1},
		low: limits{cpu: 3_840_000, memory: 3584 << 20, goMemory: 2456 << 20, sockets: 80, fds: 616,
			goroutines: 2456, threads: 192, timers: 616, socketMemory: 256 << 20,
			cpuPressure: 10, memoryPressure: 2.5, ioPressure: .5},
		emergency: limits{memory: 4608 << 20, sockets: 116, fds: 924, goroutines: 3688, threads: 1024,
			timers: 924, socketMemory: 768 << 20},
	},
}

type monitor struct {
	lastCPU, windowCPU, lastHigh, lastEmergency uint64
	initialized                                 bool
	highSamples, lowSamples                     int
	interval, window                            time.Duration
	level                                       level
	maximum                                     Sample
}

func (monitor *monitor) observe(profile profile, sample Sample) level {
	highEvent := sample.HighEvents > monitor.lastHigh
	emergencyEvent := sample.EmergencyEvents > monitor.lastEmergency
	monitor.lastHigh, monitor.lastEmergency = sample.HighEvents, sample.EmergencyEvents
	if monitor.level == levelDrain {
		return levelDrain
	}
	if emergencyEvent || crossesMaximum(sample, profile.emergency) {
		monitor.level = levelDrain
		return levelDrain
	}
	delta := uint64(0)
	if monitor.initialized && sample.CPUUsageUsec >= monitor.lastCPU {
		delta = sample.CPUUsageUsec - monitor.lastCPU
	}
	monitor.lastCPU, monitor.initialized = sample.CPUUsageUsec, true
	monitor.window += monitor.interval
	monitor.windowCPU += delta
	mergeMaximum(&monitor.maximum, sample)
	if highEvent {
		monitor.level, monitor.highSamples, monitor.lowSamples = levelProtect, 0, 0
	}
	if monitor.window < time.Second {
		return monitor.level
	}
	monitor.maximum.CPUUsageUsec = monitor.windowCPU
	high := crossesHigh(monitor.maximum, profile.high)
	low := crossesLow(monitor.maximum, profile.low)
	monitor.window, monitor.windowCPU, monitor.maximum = 0, 0, Sample{}
	if high {
		monitor.highSamples++
		monitor.lowSamples = 0
		if monitor.highSamples >= 3 {
			monitor.level = levelProtect
		}
		return monitor.level
	}
	monitor.highSamples = 0
	if monitor.level == levelProtect && low {
		monitor.lowSamples++
		if monitor.lowSamples >= 120 {
			monitor.level, monitor.lowSamples = levelNormal, 0
		}
	}
	return monitor.level
}

func crossesMaximum(sample Sample, limit limits) bool {
	return atLeast(sample.MemoryBytes, limit.memory) || atLeast(sample.SocketMemoryBytes, limit.socketMemory) ||
		atLeast(sample.StorageBytes, limit.storage) ||
		atLeast(sample.Sockets, limit.sockets) ||
		atLeast(sample.FDs, limit.fds) || atLeast(sample.Goroutines, limit.goroutines) || atLeast(sample.Threads, limit.threads) ||
		atLeast(sample.Timers, limit.timers) || atLeast(sample.QueueItems, limit.queueItems) ||
		atLeast(sample.QueueBytes, limit.queueBytes)
}

func atLeast(value, limit uint64) bool { return limit > 0 && value >= limit }

func crossesHigh(sample Sample, limit limits) bool {
	return sample.CPUUsageUsec >= limit.cpu || sample.MemoryBytes >= limit.memory || sample.GoMemoryBytes >= limit.goMemory ||
		sample.SocketMemoryBytes >= limit.socketMemory || limit.sockets > 0 && sample.Sockets >= limit.sockets ||
		limit.storage > 0 && sample.StorageBytes >= limit.storage ||
		sample.FDs >= limit.fds || sample.Goroutines >= limit.goroutines ||
		sample.Threads >= limit.threads || sample.Timers >= limit.timers || limit.queueItems > 0 && sample.QueueItems >= limit.queueItems ||
		limit.queueBytes > 0 && sample.QueueBytes >= limit.queueBytes ||
		sample.CPUPressure >= limit.cpuPressure || sample.MemoryPressure >= limit.memoryPressure || sample.IOPressure >= limit.ioPressure
}

func crossesLow(sample Sample, limit limits) bool {
	return sample.CPUUsageUsec < limit.cpu && sample.MemoryBytes <= limit.memory && sample.GoMemoryBytes < limit.goMemory &&
		sample.SocketMemoryBytes < limit.socketMemory && (limit.sockets == 0 || sample.Sockets < limit.sockets) &&
		(limit.storage == 0 || sample.StorageBytes < limit.storage) &&
		sample.FDs < limit.fds && sample.Goroutines < limit.goroutines &&
		sample.Threads < limit.threads && sample.Timers < limit.timers && (limit.queueItems == 0 || sample.QueueItems < limit.queueItems) &&
		(limit.queueBytes == 0 || sample.QueueBytes < limit.queueBytes) &&
		sample.CPUPressure < limit.cpuPressure && sample.MemoryPressure < limit.memoryPressure && sample.IOPressure < limit.ioPressure
}

func mergeMaximum(target *Sample, value Sample) {
	target.MemoryBytes, target.GoMemoryBytes = max(target.MemoryBytes, value.MemoryBytes), max(target.GoMemoryBytes, value.GoMemoryBytes)
	target.SocketMemoryBytes, target.FDs = max(target.SocketMemoryBytes, value.SocketMemoryBytes), max(target.FDs, value.FDs)
	target.StorageBytes = max(target.StorageBytes, value.StorageBytes)
	target.StorageFiles = max(target.StorageFiles, value.StorageFiles)
	target.Sockets = max(target.Sockets, value.Sockets)
	target.Goroutines, target.Threads = max(target.Goroutines, value.Goroutines), max(target.Threads, value.Threads)
	target.Timers, target.QueueItems = max(target.Timers, value.Timers), max(target.QueueItems, value.QueueItems)
	target.QueueBytes = max(target.QueueBytes, value.QueueBytes)
	target.CPUPressure, target.MemoryPressure = max(target.CPUPressure, value.CPUPressure), max(target.MemoryPressure, value.MemoryPressure)
	target.IOPressure = max(target.IOPressure, value.IOPressure)
}
