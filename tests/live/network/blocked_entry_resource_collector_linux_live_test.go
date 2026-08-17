//go:build linux && live

package network_test

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func runBlockedResourceCollector(t *testing.T) {
	t.Helper()
	root := blockedSync()
	path := filepath.Join(root, "resource.jsonl")
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	encoder := json.NewEncoder(output)
	started := time.Now()
	ready := filepath.Join(root, "resource-ready")
	writeLiveFile(t, ready, []byte("ready\n"))
	defer os.Remove(ready)
	next, ordinal, cleanupCaptured := started, uint16(0), false
	captured := map[string]bool{}
	emit := func(boundary string) {
		value := collectBlockedResourceSample(t, started, ordinal, boundary)
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
		if err := output.Sync(); err != nil {
			t.Fatal(err)
		}
		ordinal++
	}
	for {
		for _, boundary := range []string{"baseline", "after-churn"} {
			request := filepath.Join(root, "resource-"+boundary)
			if !captured[boundary] && fileExists(request) {
				emit(boundary)
				captured[boundary] = true
				writeLiveFile(t, request+"-captured", []byte("captured\n"))
			}
		}
		if !cleanupCaptured && fileExists(filepath.Join(root, "resource-cleanup")) {
			emit("post-cleanup")
			cleanupCaptured = true
			writeLiveFile(t, filepath.Join(root, "resource-cleanup-captured"), []byte("captured\n"))
		}
		if fileExists(filepath.Join(root, "resource-stop")) {
			return
		}
		if !time.Now().Before(next) {
			emit("")
			next = next.Add(time.Second)
			continue
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func collectBlockedResourceSample(t *testing.T, started time.Time, ordinal uint16,
	boundary string,
) blockedProcessSample {
	t.Helper()
	rss, fds, sockets, processes, threads, capabilities, processErr := sampleBlockedProcesses()
	swap, emergency, cgroupErr := sampleBlockedCgroup()
	stateBytes, stateEntries, stateErr := blockedTreeUsage("/proc/1/root/run/state", "")
	members, contacts, attempts, regimes, durableErr := blockedDurableCounts()
	evidenceBytes, evidenceRecords, evidenceErr := blockedTreeUsage(blockedSync(), "resource.jsonl")
	if err := errors.Join(processErr, cgroupErr, stateErr, durableErr, evidenceErr); err != nil {
		t.Fatal(err)
	}
	return blockedProcessSample{Schema: "ardents-h3-process-resource-v1", Ordinal: ordinal,
		OffsetMillis: uint64(time.Since(started) / time.Millisecond), RSSBytes: rss,
		FDs: fds, Sockets: sockets, Processes: processes, Threads: threads, SwapBytes: swap,
		EmergencyEvents: emergency, StateBytes: stateBytes, StateEntries: stateEntries,
		EvidenceRecords: evidenceRecords, EvidenceBytes: evidenceBytes, Capabilities: capabilities,
		DurableMembers: members, DurableContacts: contacts, DurableAttempts: attempts,
		DurableRegimes: regimes, Boundary: boundary}
}

func blockedDurableCounts() (uint16, uint16, uint16, uint16, error) {
	root := "/proc/1/root/run/state/bridge"
	pointer, err := os.ReadFile(filepath.Join(root, "current"))
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, 0, 0, nil
	}
	name := strings.TrimSuffix(string(pointer), "\n")
	if err != nil || len(pointer) != 65 || len(name) != 64 {
		return 0, 0, 0, 0, errors.Join(err, errors.New("Bridge state pointer is invalid"))
	}
	raw, err := os.ReadFile(filepath.Join(root, "state-"+name))
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if err != nil || len(raw) > 256<<10 || digest != name {
		return 0, 0, 0, 0, errors.Join(err, errors.New("Bridge state generation is invalid"))
	}
	var state struct {
		Records  []json.RawMessage `json:"records"`
		Contacts []json.RawMessage `json:"contacts"`
		Attempt  json.RawMessage   `json:"attempt"`
		Regime   json.RawMessage   `json:"regime"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return 0, 0, 0, 0, err
	}
	attempts, regimes := uint16(0), uint16(0)
	if len(state.Attempt) != 0 && string(state.Attempt) != "null" {
		attempts = 1
	}
	if len(state.Regime) != 0 && string(state.Regime) != "null" {
		regimes = 1
	}
	return uint16(len(state.Records)), uint16(len(state.Contacts)), attempts, regimes, nil
}

func sampleBlockedCgroup() (uint64, uint64, error) {
	root := "/proc/1/root/sys/fs/cgroup"
	raw, err := os.ReadFile(filepath.Join(root, "memory.swap.current"))
	if err != nil {
		return 0, 0, err
	}
	swap, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	events, err := os.ReadFile(filepath.Join(root, "memory.events.local"))
	if err != nil {
		return 0, 0, err
	}
	var emergency uint64
	for _, line := range strings.Split(string(events), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && (fields[0] == "max" || fields[0] == "oom" || fields[0] == "oom_kill") {
			value, parseErr := strconv.ParseUint(fields[1], 10, 64)
			if parseErr != nil {
				return 0, 0, parseErr
			}
			emergency += value
		}
	}
	return swap, emergency, nil
}

func sampleBlockedProcesses() (uint64, uint16, uint16, uint16, uint16, uint16, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	pageSize, own := uint64(os.Getpagesize()), os.Getpid()
	var rss uint64
	var fds, sockets, processes, threads, capabilities uint16
	for _, entry := range entries {
		pid, parseErr := strconv.Atoi(entry.Name())
		if parseErr != nil || pid == own || !entry.IsDir() {
			continue
		}
		statm, readErr := os.ReadFile(filepath.Join("/proc", entry.Name(), "statm"))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return 0, 0, 0, 0, 0, 0, readErr
		}
		fields := strings.Fields(string(statm))
		if len(fields) < 2 {
			return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid statm for pid %d", pid)
		}
		pages, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			return 0, 0, 0, 0, 0, 0, parseErr
		}
		processFDs, processSockets, countErr := countBlockedDescriptors(filepath.Join("/proc", entry.Name(), "fd"))
		if errors.Is(countErr, os.ErrNotExist) {
			continue
		}
		if countErr != nil {
			return 0, 0, 0, 0, 0, 0, countErr
		}
		processThreads, processCapabilities, threadErr := blockedProcessStatus(
			filepath.Join("/proc", entry.Name(), "status"))
		if threadErr != nil {
			return 0, 0, 0, 0, 0, 0, threadErr
		}
		rss += pages * pageSize
		fds += processFDs
		sockets += processSockets
		processes++
		threads += processThreads
		capabilities += processCapabilities
	}
	return rss, fds, sockets, processes, threads, capabilities, nil
}

func blockedProcessStatus(file string) (uint16, uint16, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, err
	}
	var threads, capabilities uint16
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "Threads:" {
			value, parseErr := strconv.ParseUint(fields[1], 10, 16)
			if parseErr != nil {
				return 0, 0, parseErr
			}
			threads = uint16(value)
		}
		if len(fields) == 2 && fields[0] == "CapEff:" && strings.Trim(fields[1], "0") != "" {
			capabilities = 1
		}
	}
	if threads == 0 {
		return 0, 0, errors.New("process status omits Threads")
	}
	return threads, capabilities, nil
}

func blockedTreeUsage(root, excluded string) (uint64, uint16, error) {
	var bytes uint64
	var entries uint16
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil || entry.Type()&os.ModeSymlink != 0 {
			return errors.Join(walkErr, errors.New("resource tree is unavailable or aliased"))
		}
		base := filepath.Base(path)
		if entry.IsDir() || base == excluded || blockedHarnessControl(base) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		bytes += uint64(info.Size())
		entries++
		return nil
	})
	return bytes, entries, err
}

func blockedHarnessControl(base string) bool {
	if strings.HasPrefix(base, "resource-") || strings.HasSuffix(base, "-stop") ||
		strings.HasSuffix(base, "-ready") || strings.HasSuffix(base, ".start") {
		return true
	}
	return false
}

func countBlockedDescriptors(root string) (uint16, uint16, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, err
	}
	var sockets uint16
	for _, entry := range entries {
		target, readErr := os.Readlink(filepath.Join(root, entry.Name()))
		if readErr == nil && strings.HasPrefix(target, "socket:[") {
			sockets++
		}
	}
	return uint16(len(entries)), sockets, nil
}
