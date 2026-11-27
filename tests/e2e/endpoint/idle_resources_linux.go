//go:build linux && endpoint_launch_native

package endpoint

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type net32ClientResources struct {
	process *os.Root
	mount   string
	groups  []*os.Root
	lastCPU []uint64
}

func openNet32ClientResources(processPath, cgroupMount string) (*net32ClientResources, error) {
	process, err := os.OpenRoot(processPath)
	if err != nil {
		return nil, err
	}
	resources := &net32ClientResources{process: process, mount: cgroupMount}
	paths, err := net32ClientCgroups(process, cgroupMount)
	if err != nil {
		return nil, errors.Join(err, resources.close())
	}
	for _, path := range paths {
		group, err := os.OpenRoot(path)
		if err != nil {
			return nil, errors.Join(err, resources.close())
		}
		resources.groups = append(resources.groups, group)
	}
	return resources, nil
}

func (r *net32ClientResources) close() error {
	err := r.process.Close()
	for _, group := range r.groups {
		err = errors.Join(err, group.Close())
	}
	return err
}

// Reading through the original directories binds both counters to the same
// cgroups even if systemd later reuses a unit path. Check the live cohort on
// both sides of the reads so churn during a sample cannot authorize a delta.
func (r *net32ClientResources) read() (memoryBytes, cpuUsec uint64, err error) {
	if err := r.checkCohort(); err != nil {
		return 0, 0, err
	}
	counters := make([]uint64, len(r.groups))
	for i, group := range r.groups {
		memory, cpu, err := net32CgroupUsage(group)
		if err != nil {
			return 0, 0, err
		}
		if r.lastCPU != nil && cpu < r.lastCPU[i] {
			return 0, 0, errors.New("cgroup CPU counter regressed")
		}
		if memory > math.MaxUint64-memoryBytes || cpu > math.MaxUint64-cpuUsec {
			return 0, 0, errors.New("cgroup resource sum overflowed")
		}
		counters[i] = cpu
		memoryBytes += memory
		cpuUsec += cpu
	}
	if err := r.checkCohort(); err != nil {
		return 0, 0, err
	}
	r.lastCPU = counters
	return memoryBytes, cpuUsec, nil
}

func (r *net32ClientResources) checkCohort() error {
	paths, err := net32ClientCgroups(r.process, r.mount)
	if err != nil {
		return err
	}
	if len(paths) != len(r.groups) {
		return errors.New("Endpoint or worker cgroup membership changed")
	}
	for i, group := range r.groups {
		if paths[i] != group.Name() {
			return errors.New("Endpoint or worker cgroup membership changed")
		}
		original, originalErr := group.Stat(".")
		current, currentErr := os.Stat(paths[i])
		if err := errors.Join(originalErr, currentErr); err != nil {
			return err
		}
		if !os.SameFile(original, current) {
			return errors.New("Endpoint or worker cgroup identity changed")
		}
	}
	return nil
}

func net32ClientCgroups(process *os.Root, mount string) ([]string, error) {
	raw, err := process.ReadFile("cgroup")
	if err != nil {
		return nil, err
	}
	const endpoint = "/system.slice/ardents-endpoint.service"
	found := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		if line == "0::"+endpoint {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("original Endpoint cgroup unavailable")
	}
	paths := []string{filepath.Join(mount, endpoint)}
	slice := filepath.Join(mount, "system.slice")
	entries, err := os.ReadDir(slice)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() && strings.HasSuffix(name, ".service") &&
			(strings.HasPrefix(name, "ardents-text-reader@") || strings.HasPrefix(name, "ardents-text-publisher@")) {
			paths = append(paths, filepath.Join(slice, name))
		}
	}
	return paths, nil
}

func net32CgroupUsage(group *os.Root) (memoryBytes, cpuUsec uint64, err error) {
	memory, err := group.ReadFile("memory.current")
	if err != nil {
		return 0, 0, err
	}
	memoryBytes, err = strconv.ParseUint(strings.TrimSpace(string(memory)), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("read memory.current bytes: %w", err)
	}
	cpu, err := group.ReadFile("cpu.stat")
	if err != nil {
		return 0, 0, err
	}
	found := false
	for line := range strings.SplitSeq(string(cpu), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "usage_usec" {
			continue
		}
		if found || len(fields) != 2 {
			return 0, 0, errors.New("cpu.stat usage_usec is ambiguous or malformed")
		}
		cpuUsec, err = strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("read cpu.stat usage_usec: %w", err)
		}
		found = true
	}
	if !found {
		return 0, 0, errors.New("cpu.stat usage_usec unavailable")
	}
	return memoryBytes, cpuUsec, nil
}
