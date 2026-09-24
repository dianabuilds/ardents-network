//go:build linux

package resource

import (
	"errors"
	"path/filepath"
	"strings"
)

// MeasureOwnerCgroups includes the current Endpoint cgroup and its verified
// worker cgroups. memory.current includes kernel and cache charges too, so it
// is retained separately from the sum of resident pages of every process in
// these cgroups and descendants. A disappearing process triggers one complete
// re-inventory; a second disappearance invalidates the sample.
func MeasureOwnerCgroups(workers []string) (Sample, error) {
	current, err := currentCgroupDirectory()
	if err != nil {
		return Sample{}, err
	}
	paths := []string{current}
	for _, worker := range workers {
		if !validOwnerWorkerCgroup(worker) {
			return Sample{}, errors.New("worker cgroup path invalid")
		}
		path := filepath.Join("/sys/fs/cgroup", worker)
		duplicate := false
		for _, prior := range paths {
			if path == prior || strings.HasPrefix(path, prior+"/") {
				duplicate = true
			}
		}
		if !duplicate {
			paths = append(paths, path)
		}
	}
	var sample Sample
	for _, path := range paths {
		memory, err := boundedFile(filepath.Join(path, "memory.current"), 64)
		if err != nil {
			return Sample{}, err
		}
		value, err := counter("memory "+strings.TrimSpace(memory), "memory")
		if err != nil {
			return Sample{}, err
		}
		sample.MemoryBytes += value
		cpu, err := boundedFile(filepath.Join(path, "cpu.stat"), 4096)
		if err != nil {
			return Sample{}, err
		}
		usage, err := counter(cpu, "usage_usec")
		if err != nil {
			return Sample{}, err
		}
		sample.CPUUsageUsec += usage
	}
	sample.RSSBytes, err = ownerResidentBytes(paths)
	return sample, err
}
