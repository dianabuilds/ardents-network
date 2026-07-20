package execution

import "fmt"

const (
	defaultContainerMemory = 512 * 1024 * 1024
	defaultContainerCPU    = 1_000_000_000
	defaultContainerPIDs   = 128
	defaultContainerTmpfs  = 64 * 1024 * 1024
	minimumContainerMemory = 32 * 1024 * 1024
	maximumContainerMemory = 4 * 1024 * 1024 * 1024
	minimumContainerCPU    = 100_000_000
	maximumContainerCPU    = 4_000_000_000
	minimumContainerPIDs   = 16
	maximumContainerPIDs   = 512
	minimumContainerTmpfs  = 1024 * 1024
	maximumContainerTmpfs  = 512 * 1024 * 1024
)

func applyResourceDefaults(resources *containerResources) error {
	if resources.MemoryBytes == 0 {
		resources.MemoryBytes = defaultContainerMemory
	}
	if resources.NanoCPUs == 0 {
		resources.NanoCPUs = defaultContainerCPU
	}
	if resources.PIDs == 0 {
		resources.PIDs = defaultContainerPIDs
	}
	if resources.TmpfsBytes == 0 {
		resources.TmpfsBytes = defaultContainerTmpfs
	}
	checks := []struct {
		name       string
		value, min int64
		max        int64
	}{
		{"memory_bytes", resources.MemoryBytes, minimumContainerMemory, maximumContainerMemory},
		{"nano_cpus", resources.NanoCPUs, minimumContainerCPU, maximumContainerCPU},
		{"pids", resources.PIDs, minimumContainerPIDs, maximumContainerPIDs},
		{"tmpfs_bytes", resources.TmpfsBytes, minimumContainerTmpfs, maximumContainerTmpfs},
	}
	for _, check := range checks {
		if check.value < check.min || check.value > check.max {
			return fmt.Errorf("container resource %s is outside policy bounds", check.name)
		}
	}
	return nil
}

func tmpfsOptions(size int64) string {
	return fmt.Sprintf("rw,noexec,nosuid,nodev,size=%d", size)
}
