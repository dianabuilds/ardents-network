//go:build ignore

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type residualBaseline struct {
	FDTargets  map[string]int
	Goroutines int
	PIDs       map[int]string
}

type residualObservation struct {
	ClientReaped         bool `json:"client_reaped"`
	ServerReaped         bool `json:"server_reaped"`
	StateRemoved         bool `json:"state_removed"`
	FDTargetsRestored    bool `json:"fd_targets_restored"`
	GoroutinesRestored   bool `json:"goroutines_restored"`
	PIDNamespaceRestored bool `json:"pid_namespace_restored"`
}

func captureResidualBaseline() (residualBaseline, error) {
	targets, err := fdTargets(os.Getpid())
	if err != nil {
		return residualBaseline{}, err
	}
	pids, err := pidSnapshot()
	return residualBaseline{FDTargets: targets, Goroutines: runtime.NumGoroutine(), PIDs: pids}, err
}

func verifyResidual(baseline residualBaseline, clientPID, serverPID int, stateRoot string) (residualObservation, error) {
	result := residualObservation{
		ClientReaped: processAbsent(clientPID),
		ServerReaped: processAbsent(serverPID),
		StateRemoved: pathAbsent(stateRoot),
	}
	current, err := fdTargets(os.Getpid())
	if err != nil {
		return result, err
	}
	result.FDTargetsRestored = sameCounts(baseline.FDTargets, current)
	result.GoroutinesRestored = runtime.NumGoroutine() <= baseline.Goroutines
	pids, err := pidSnapshot()
	if err != nil {
		return result, err
	}
	result.PIDNamespaceRestored = samePIDs(baseline.PIDs, pids)
	if !result.ClientReaped || !result.ServerReaped || !result.StateRemoved ||
		!result.FDTargetsRestored || !result.GoroutinesRestored || !result.PIDNamespaceRestored {
		return result, errors.New("owned cleanup residue detected")
	}
	return result, nil
}

func validatePIDNamespace(baseline residualBaseline, clientPID, serverPID int) error {
	expected := make(map[int]string, len(baseline.PIDs)+2)
	for pid, identity := range baseline.PIDs {
		expected[pid] = identity
	}
	current, err := pidSnapshot()
	if err != nil {
		return err
	}
	for _, pid := range []int{clientPID, serverPID} {
		identity, ok := current[pid]
		if !ok {
			return fmt.Errorf("candidate pid %d absent from namespace", pid)
		}
		expected[pid] = identity
	}
	if !samePIDs(expected, current) {
		return errors.New("unexpected process in candidate PID namespace")
	}
	return nil
}

func pidSnapshot() (map[int]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	result := make(map[int]string)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		closing := strings.LastIndex(string(stat), ") ")
		if closing < 0 {
			return nil, errors.New("malformed /proc stat")
		}
		fields := strings.Fields(string(stat)[closing+2:])
		if len(fields) <= 19 {
			return nil, errors.New("short /proc stat")
		}
		result[pid] = fields[19]
	}
	return result, nil
}

func fdTargets(pid int) (map[string]int, error) {
	directory := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(directory, entry.Name()))
		if err != nil {
			continue
		}
		result[target]++
	}
	return result, nil
}

func processAbsent(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return errors.Is(err, os.ErrNotExist)
}

func pathAbsent(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func sameCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func samePIDs(left, right map[int]string) bool {
	if len(left) != len(right) {
		return false
	}
	for pid, identity := range left {
		if right[pid] != identity {
			return false
		}
	}
	return true
}

func validateObserver(observation dnsObservation) error {
	if observation.Capabilities != "0000000000002000" {
		return fmt.Errorf("DNS observer capabilities %s", observation.Capabilities)
	}
	if observation.Packets != 0 {
		return fmt.Errorf("observed %d DNS packets", observation.Packets)
	}
	if observation.ControlPackets < 1 {
		return errors.New("DNS observer missed its positive control")
	}
	if observation.AmbiguousPackets != 0 {
		return fmt.Errorf("observed %d packets with hidden transport headers", observation.AmbiguousPackets)
	}
	return nil
}
