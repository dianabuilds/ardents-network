package blockedverify

type finalObservedHost struct {
	ID                 string                    `json:"id"`
	LogicalCPUs        uint16                    `json:"logical_cpus"`
	MemoryMiB          uint32                    `json:"memory_mib"`
	AllocatedVCPU      uint16                    `json:"allocated_vcpu"`
	AllocatedMemoryMiB uint32                    `json:"allocated_memory_mib"`
	DedicatedThreads   bool                      `json:"dedicated_threads"`
	CgroupV2           bool                      `json:"cgroup_v2"`
	SwapEvents         uint16                    `json:"swap_events"`
	Allocations        []finalObservedAllocation `json:"allocations"`
	Runtime            *finalRuntimeHost         `json:"runtime,omitempty"`
}

type finalRuntimeHost struct {
	Schema                 string                   `json:"schema"`
	CapturedMonotonicNanos uint64                   `json:"captured_monotonic_nanos"`
	Allocations            []finalRuntimeAllocation `json:"allocations"`
}

type finalRuntimeAllocation struct {
	ID                     string   `json:"id"`
	CPUSet                 []uint16 `json:"cpu_set"`
	Cgroups                []string `json:"cgroups"`
	NetworkNamespaceInodes []uint64 `json:"network_namespace_inodes"`
	MemoryMaxBytes         uint64   `json:"memory_max_bytes"`
}

type finalObservedAllocation struct {
	ID               string `json:"id"`
	Class            string `json:"class"`
	ProcessNamespace string `json:"process_namespace"`
	NetworkNamespace string `json:"network_namespace"`
	VCPU             uint16 `json:"vcpu"`
	MemoryMiB        uint32 `json:"memory_mib"`
}
