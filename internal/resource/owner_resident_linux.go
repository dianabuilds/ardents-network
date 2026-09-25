//go:build linux

package resource

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var errOwnerResidentProcessGone = errors.New("owner process disappeared during resident sample")

const ownerResidentSampleAttempts = 8

// cgroup.procs includes thread-group leaders, so threads do not multiply RSS.
// Nested cgroups are included; process IDs are deduplicated across observations.
func ownerResidentBytes(groups []string) (uint64, error) {
	return ownerResidentBytesWithReader(groups, boundedFile)
}

func ownerResidentBytesWithReader(groups []string, readFile func(string, int) (string, error)) (uint64, error) {
	for attempt := 0; attempt < ownerResidentSampleAttempts; attempt++ {
		total, err := ownerResidentBytesOnce(groups, readFile)
		if err == nil {
			return total, nil
		}
		if attempt+1 < ownerResidentSampleAttempts && errors.Is(err, errOwnerResidentProcessGone) {
			continue
		}
		return 0, err
	}
	return 0, errors.New("owner resident sample retry exhausted")
}

func ownerResidentBytesOnce(groups []string, readFile func(string, int) (string, error)) (uint64, error) {
	pids := make(map[string]bool)
	directories := 0
	for _, group := range groups {
		err := filepath.WalkDir(group, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				return nil
			}
			directories++
			if directories > 4096 {
				return errors.New("owner cgroup inventory exceeds bound")
			}
			body, err := readFile(filepath.Join(path, "cgroup.procs"), 64<<10)
			if err != nil {
				return err
			}
			for _, pid := range strings.Fields(body) {
				value, err := strconv.ParseUint(pid, 10, 32)
				if err != nil || value == 0 || strconv.FormatUint(value, 10) != pid {
					return errors.New("owner process identity invalid")
				}
				pids[pid] = true
				if len(pids) > 4096 {
					return errors.New("owner process inventory exceeds bound")
				}
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	if len(pids) == 0 {
		return 0, errors.New("owner process inventory empty")
	}
	var total uint64
	for pid := range pids {
		body, err := readFile(filepath.Join("/proc", pid, "statm"), 4096)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
				return 0, fmt.Errorf("%w: %w", errOwnerResidentProcessGone, err)
			}
			return 0, err
		}
		resident, err := residentBytes(body, uint64(os.Getpagesize()))
		if err != nil || resident > math.MaxUint64-total {
			return 0, errors.New("owner resident memory counter invalid")
		}
		total += resident
	}
	return total, nil
}

func residentBytes(statm string, pageSize uint64) (uint64, error) {
	fields := strings.Fields(statm)
	if len(fields) != 7 || pageSize == 0 {
		return 0, errors.New("process memory sample invalid")
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil || pages > math.MaxUint64/pageSize {
		return 0, errors.New("process resident pages invalid")
	}
	return pages * pageSize, nil
}
