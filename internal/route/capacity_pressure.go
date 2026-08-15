package route

import (
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type capacityResourceRecord struct {
	lastCPU, windowCPU uint64
	window             time.Duration
	initialized        bool
}

func capacityGuard(input Actor, observation *Evidence, record *capacityResourceRecord,
	interval time.Duration) (*resource.Guard, string, error) {
	if input.ResourceProfile == "" {
		return nil, "NORMAL", nil
	}
	guard, err := resource.New(resource.Config{Profile: input.ResourceProfile, Interval: interval,
		Measure: input.ResourceMeasure})
	if err != nil {
		return nil, "", err
	}
	check := input.ResourceCheck
	if check == nil {
		check = guard.Check
	}
	if err := check(); err != nil {
		return nil, "", fmt.Errorf("check Route resource placement: %w", err)
	}
	initial, observeErr := guard.Observe(1, 0, 0)
	if observeErr != nil || initial.Drain {
		return nil, "", errors.Join(errors.New("route resource placement starts above its emergency bound"), observeErr)
	}
	observation.Resource = &initial.Sample
	record.record(observation, initial.Sample, 0)
	if initial.Protect {
		return guard, "PROTECT", nil
	}
	return guard, "NORMAL", nil
}

func (record *capacityResourceRecord) record(observation *Evidence, sample resource.Sample, elapsed time.Duration) {
	observation.ResourceSamples++
	if observation.ResourceMaximum == nil {
		observation.ResourceMaximum = &resource.Sample{}
	}
	maximum := observation.ResourceMaximum
	if record.initialized && sample.CPUUsageUsec >= record.lastCPU {
		record.windowCPU += sample.CPUUsageUsec - record.lastCPU
		record.window += elapsed
	}
	record.lastCPU, record.initialized = sample.CPUUsageUsec, true
	if record.window >= time.Second {
		record.flushCPU(maximum)
	}
	maximum.MemoryBytes = max(maximum.MemoryBytes, sample.MemoryBytes)
	maximum.GoMemoryBytes = max(maximum.GoMemoryBytes, sample.GoMemoryBytes)
	maximum.SocketMemoryBytes = max(maximum.SocketMemoryBytes, sample.SocketMemoryBytes)
	maximum.FDs, maximum.Goroutines = max(maximum.FDs, sample.FDs), max(maximum.Goroutines, sample.Goroutines)
	maximum.Threads, maximum.Timers = max(maximum.Threads, sample.Threads), max(maximum.Timers, sample.Timers)
	maximum.QueueItems, maximum.QueueBytes = max(maximum.QueueItems, sample.QueueItems), max(maximum.QueueBytes, sample.QueueBytes)
	maximum.CPUPressure = max(maximum.CPUPressure, sample.CPUPressure)
	maximum.MemoryPressure = max(maximum.MemoryPressure, sample.MemoryPressure)
	maximum.IOPressure = max(maximum.IOPressure, sample.IOPressure)
	maximum.HighEvents = max(maximum.HighEvents, sample.HighEvents)
	maximum.EmergencyEvents = max(maximum.EmergencyEvents, sample.EmergencyEvents)
}

func (record *capacityResourceRecord) finish(observation *Evidence) {
	if observation.ResourceMaximum != nil {
		record.flushCPU(observation.ResourceMaximum)
	}
}

func (record *capacityResourceRecord) flushCPU(maximum *resource.Sample) {
	if record.window > 0 {
		rate := record.windowCPU * uint64(time.Second) / uint64(record.window)
		maximum.CPUUsageUsec = max(maximum.CPUUsageUsec, rate)
	}
	record.window, record.windowCPU = 0, 0
}

func capacityEvent(input Actor, observation Evidence, kind, state string, sample *resource.Sample) Evidence {
	return Evidence{Schema: observationSchema, Kind: kind, State: state, Role: input.Role, PID: observation.PID,
		NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.NodeID,
		PreviousPin: input.UpstreamPin, NextNodeID: input.NextNodeID,
		DeadlineMillis: uint32(input.Deadline / time.Millisecond),
		LifetimeMillis: uint32(input.Lifetime / time.Millisecond), Resource: sample}
}
