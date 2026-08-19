//go:build live

package network_test

type finalWorkerCapacity struct {
	Profile              string                `json:"profile"`
	Terminal             string                `json:"terminal"`
	Batch                uint16                `json:"batch"`
	Offered              uint16                `json:"offered"`
	Accepted             uint16                `json:"accepted"`
	Refused              uint16                `json:"refused"`
	MaximumRefusalMillis uint32                `json:"maximum_refusal_millis"`
	EstablishedProgress  bool                  `json:"established_progress"`
	Cleanup              bool                  `json:"cleanup"`
	SecurityExact        bool                  `json:"security_exact"`
	ReservePercent       float64               `json:"reserve_percent"`
	ResponseP95Millis    uint32                `json:"response_p95_millis"`
	Resources            finalResourceEvidence `json:"resources"`
}

type finalWorkerRecovery struct {
	Schema                 string `json:"schema,omitempty"`
	Episode                uint16 `json:"episode"`
	ServiceClass           string `json:"service_class"`
	RecoveryClass          string `json:"recovery_class"`
	Attempt                string `json:"attempt"`
	ContactStarts          uint16 `json:"contact_starts"`
	LaterOrdinals          uint16 `json:"later_ordinals"`
	Cleanup                bool   `json:"cleanup"`
	PublishedDelayMillis   uint32 `json:"published_delay_millis"`
	ApplicationDelayMillis uint32 `json:"application_delay_millis"`
	Residuals              uint16 `json:"residuals"`
}

func finalRecoveryOutcome(value finalWorkerRecovery) (bool, bool, bool) {
	connectionLoss := value.ServiceClass == "abrupt connection loss" &&
		value.RecoveryClass == "bridge-deadline-exceeded"
	attemptStable := len(value.Attempt) == 64 && value.ContactStarts == 1 && value.LaterOrdinals == 0
	deadlineStable := value.Cleanup && value.PublishedDelayMillis <= 15_000 &&
		value.ApplicationDelayMillis <= 15_000
	return connectionLoss, attemptStable, deadlineStable
}

type finalRunnerRecovery struct {
	Attempts              uint16 `json:"attempts"`
	ConnectionLoss        uint16 `json:"connection_loss"`
	LaterStarts           uint16 `json:"later_starts"`
	Residuals             uint16 `json:"residuals"`
	AttemptIdentityStable bool   `json:"attempt_identity_stable"`
	DeadlineStable        bool   `json:"deadline_stable"`
}

type finalRunnerObservedHost struct {
	ID                 string                          `json:"id"`
	LogicalCPUs        uint16                          `json:"logical_cpus"`
	MemoryMiB          uint32                          `json:"memory_mib"`
	AllocatedVCPU      uint16                          `json:"allocated_vcpu"`
	AllocatedMemoryMiB uint32                          `json:"allocated_memory_mib"`
	DedicatedThreads   bool                            `json:"dedicated_threads"`
	CgroupV2           bool                            `json:"cgroup_v2"`
	SwapEvents         uint16                          `json:"swap_events"`
	Allocations        []finalRunnerObservedAllocation `json:"allocations"`
	Runtime            *finalRunnerRuntimeHost         `json:"runtime,omitempty"`
}

type finalRunnerRuntimeHost struct {
	Schema                 string                         `json:"schema"`
	CapturedMonotonicNanos uint64                         `json:"captured_monotonic_nanos"`
	Allocations            []finalRunnerRuntimeAllocation `json:"allocations"`
}

type finalRunnerRuntimeAllocation struct {
	ID                     string   `json:"id"`
	CPUSet                 []uint16 `json:"cpu_set"`
	Cgroups                []string `json:"cgroups"`
	NetworkNamespaceInodes []uint64 `json:"network_namespace_inodes"`
	MemoryMaxBytes         uint64   `json:"memory_max_bytes"`
}

type finalRunnerObservedAllocation struct {
	ID               string `json:"id"`
	Class            string `json:"class"`
	ProcessNamespace string `json:"process_namespace"`
	NetworkNamespace string `json:"network_namespace"`
	VCPU             uint16 `json:"vcpu"`
	MemoryMiB        uint32 `json:"memory_mib"`
}
