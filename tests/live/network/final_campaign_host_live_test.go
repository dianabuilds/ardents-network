//go:build live

package network_test

import (
	"fmt"
	"strings"
)

const (
	finalHostVCPU      = 102
	finalHostMemoryMiB = 190_464
)

func validFinalRunnerHostAllocation(hosts []finalRunnerObservedHost) bool {
	if len(hosts) != 1 {
		return false
	}
	host := hosts[0]
	if host.ID == "" || host.Runtime != nil || host.LogicalCPUs < finalHostVCPU || host.MemoryMiB < finalHostMemoryMiB ||
		host.AllocatedVCPU != finalHostVCPU || host.AllocatedMemoryMiB != finalHostMemoryMiB ||
		!host.DedicatedThreads || !host.CgroupV2 || host.SwapEvents != 0 || len(host.Allocations) != 24 {
		return false
	}
	wanted := make(map[string]finalRunnerObservedAllocation, 24)
	for index := range 16 {
		id := fmt.Sprintf("endpoint-%02d", index)
		wanted[id] = finalRunnerObservedAllocation{ID: id, Class: "endpoint-reference", VCPU: 4, MemoryMiB: 8_192}
	}
	wanted["publisher"] = finalRunnerObservedAllocation{ID: "publisher", Class: "publisher-reference", VCPU: 4,
		MemoryMiB: 8_192}
	wanted["bridge"] = finalRunnerObservedAllocation{ID: "bridge", Class: "h3-s5-b1-v1-strong", VCPU: 8,
		MemoryMiB: 8_192}
	wanted["harness"] = finalRunnerObservedAllocation{ID: "harness", Class: "collector", VCPU: 16,
		MemoryMiB: 32_768}
	for _, role := range []string{"ordinary-entry", "initiator", "introduction", "rendezvous", "responder"} {
		wanted[role] = finalRunnerObservedAllocation{ID: role, Class: "route-reference", VCPU: 2, MemoryMiB: 2_048}
	}
	seen, processes, networks := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	for _, allocation := range host.Allocations {
		expected, ok := wanted[allocation.ID]
		if !ok || seen[allocation.ID] || allocation.Class != expected.Class || allocation.VCPU != expected.VCPU ||
			allocation.MemoryMiB != expected.MemoryMiB || allocation.ProcessNamespace == "" ||
			allocation.NetworkNamespace == "" || strings.HasPrefix(allocation.ProcessNamespace, "allocation:") ||
			strings.HasPrefix(allocation.NetworkNamespace, "allocation:") || processes[allocation.ProcessNamespace] ||
			networks[allocation.NetworkNamespace] {
			return false
		}
		seen[allocation.ID], processes[allocation.ProcessNamespace], networks[allocation.NetworkNamespace] = true, true, true
	}
	return len(seen) == len(wanted)
}
