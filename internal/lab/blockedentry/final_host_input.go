package blockedentry

import (
	"errors"
	"fmt"
	"strings"
)

type finalHostAllocationRecord struct {
	Schema string              `json:"schema"`
	Hosts  []finalObservedHost `json:"hosts"`
}

func loadFinalHostAllocation(path string) ([]finalObservedHost, error) {
	if path == "" {
		return nil, errors.New("final qualifying-stand allocation record is required")
	}
	var record finalHostAllocationRecord
	if err := decodeCanonical(path, &record); err != nil ||
		record.Schema != "ardents-h3-final-host-allocation-v1" || !validFinalHostAllocation(record.Hosts) {
		return nil, errors.Join(err, errors.New("final qualifying-stand allocation record is invalid"))
	}
	return record.Hosts, nil
}

func validFinalHostAllocation(hosts []finalObservedHost) bool {
	if len(hosts) == 0 {
		return false
	}
	wanted := expectedFinalHostAllocations()
	seenHosts := make(map[string]bool, len(hosts))
	seenAllocations := make(map[string]bool, len(wanted))
	seenProcesses, seenNetworks := make(map[string]bool), make(map[string]bool)
	for _, host := range hosts {
		if host.ID == "" || seenHosts[host.ID] || host.Runtime != nil || !host.DedicatedThreads || !host.CgroupV2 ||
			host.SwapEvents != 0 || uint32(host.AllocatedVCPU) > uint32(host.LogicalCPUs) ||
			host.AllocatedMemoryMiB > host.MemoryMiB {
			return false
		}
		seenHosts[host.ID] = true
		var cpu uint32
		var memory uint64
		for _, allocation := range host.Allocations {
			expected, exists := wanted[allocation.ID]
			if !exists || seenAllocations[allocation.ID] || allocation.Class != expected.Class ||
				allocation.VCPU != expected.VCPU || allocation.MemoryMiB != expected.MemoryMiB ||
				allocation.ProcessNamespace == "" || allocation.NetworkNamespace == "" ||
				strings.HasPrefix(allocation.ProcessNamespace, "allocation:") ||
				strings.HasPrefix(allocation.NetworkNamespace, "allocation:") ||
				seenProcesses[allocation.ProcessNamespace] || seenNetworks[allocation.NetworkNamespace] {
				return false
			}
			seenAllocations[allocation.ID] = true
			seenProcesses[allocation.ProcessNamespace] = true
			seenNetworks[allocation.NetworkNamespace] = true
			cpu += uint32(allocation.VCPU)
			memory += uint64(allocation.MemoryMiB)
		}
		if cpu != uint32(host.AllocatedVCPU) || memory != uint64(host.AllocatedMemoryMiB) {
			return false
		}
	}
	return len(seenAllocations) == len(wanted)
}

func expectedFinalHostAllocations() map[string]finalObservedAllocation {
	result := make(map[string]finalObservedAllocation, 24)
	for index := range 16 {
		id := fmt.Sprintf("endpoint-%02d", index)
		result[id] = finalObservedAllocation{ID: id, Class: "endpoint-reference", VCPU: 4, MemoryMiB: 8_192}
	}
	result["publisher"] = finalObservedAllocation{ID: "publisher", Class: "publisher-reference", VCPU: 4,
		MemoryMiB: 8_192}
	result["bridge"] = finalObservedAllocation{ID: "bridge", Class: "h3-s5-b1-v1-strong", VCPU: 8,
		MemoryMiB: 8_192}
	result["harness"] = finalObservedAllocation{ID: "harness", Class: "collector", VCPU: 16,
		MemoryMiB: 32_768}
	for _, id := range []string{"ordinary-entry", "initiator", "introduction", "rendezvous", "responder"} {
		result[id] = finalObservedAllocation{ID: id, Class: "route-reference", VCPU: 2, MemoryMiB: 2_048}
	}
	return result
}
