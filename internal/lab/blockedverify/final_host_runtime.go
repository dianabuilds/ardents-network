package blockedverify

import "strings"

func validFinalRuntimeHost(host finalObservedHost) bool {
	if host.Runtime == nil || host.Runtime.Schema != "ardents-h3-final-host-runtime-v1" ||
		host.Runtime.CapturedMonotonicNanos == 0 || len(host.Runtime.Allocations) != len(host.Allocations) {
		return false
	}
	wanted := make(map[string]finalObservedAllocation, len(host.Allocations))
	for _, allocation := range host.Allocations {
		wanted[allocation.ID] = allocation
	}
	seen, cpus, cgroups, namespaces := make(map[string]bool), make(map[uint16]bool),
		make(map[string]bool), make(map[uint64]bool)
	for _, runtime := range host.Runtime.Allocations {
		allocation, ok := wanted[runtime.ID]
		if !ok || seen[runtime.ID] || len(runtime.CPUSet) != int(allocation.VCPU) ||
			runtime.MemoryMaxBytes != uint64(allocation.MemoryMiB)<<20 || len(runtime.Cgroups) == 0 ||
			len(runtime.NetworkNamespaceInodes) == 0 {
			return false
		}
		seen[runtime.ID] = true
		for _, cpu := range runtime.CPUSet {
			if cpu >= host.LogicalCPUs || cpus[cpu] {
				return false
			}
			cpus[cpu] = true
		}
		for _, cgroup := range runtime.Cgroups {
			if !strings.HasPrefix(cgroup, "/") || cgroups[cgroup] {
				return false
			}
			cgroups[cgroup] = true
		}
		for _, namespace := range runtime.NetworkNamespaceInodes {
			if namespace == 0 || namespaces[namespace] {
				return false
			}
			namespaces[namespace] = true
		}
	}
	return len(seen) == len(wanted) && len(cpus) == int(host.AllocatedVCPU)
}
