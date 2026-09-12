//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type installedHostileProcess struct {
	pid, parent             int
	started, state          string
	uid                     uint64
	ignoresTERM, restricted bool
}

func pinInstalledHostileTree(t *testing.T, instance textWorkerInstance) (*os.File, []installedHostileProcess) {
	t.Helper()
	events, err := pinTextWorkerCgroup(instance)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := events.Close(); err != nil {
			t.Error(err)
		}
	})
	body, err := readInstalledProcFile("/sys/fs/cgroup" + instance.cgroup + "/cgroup.procs")
	if err != nil {
		t.Fatal(err)
	}
	var processes []installedHostileProcess
	for _, word := range strings.Fields(string(body)) {
		pid, err := strconv.Atoi(word)
		if err != nil || pid <= 0 {
			t.Fatal("invalid installed process inventory")
		}
		process, err := readInstalledHostileProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		if process.uid != uint64(instance.uid) || !process.restricted {
			t.Fatal("descendant escaped worker UID or inherited hardening")
		}
		processes = append(processes, process)
	}
	if len(processes) != 3 {
		t.Fatalf("invalid hostile artifact positive control: got %d processes, want parent, child and grandchild", len(processes))
	}
	var parentFound bool
	var child int
	for _, process := range processes {
		if process.pid == int(instance.pid) {
			parentFound = true
			continue
		}
		if !process.ignoresTERM {
			t.Fatal("hostile descendant does not ignore SIGTERM")
		}
		if process.parent == int(instance.pid) {
			child = process.pid
		}
	}
	if !parentFound || child == 0 {
		t.Fatal("hostile parent/child lineage missing")
	}
	var grandchild bool
	for _, process := range processes {
		if process.parent == child {
			grandchild = true
		}
	}
	if !grandchild {
		t.Fatal("hostile grandchild lineage missing")
	}
	if gone, populated, err := readTextWorkerCgroup(events); err != nil || gone || !populated {
		t.Fatal("hostile cgroup positive control is not populated")
	}
	return events, processes
}

func readInstalledHostileProcess(pid int) (installedHostileProcess, error) {
	process := installedHostileProcess{pid: pid}
	root := fmt.Sprintf("/proc/%d/", pid)
	stat, err := readInstalledProcFile(root + "stat")
	if err != nil {
		return process, err
	}
	end := strings.LastIndex(string(stat), ")")
	if end < 0 {
		return process, errors.New("process stat unavailable")
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 20 {
		return process, errors.New("process identity incomplete")
	}
	process.state, process.started = fields[0], fields[19]
	process.parent, err = strconv.Atoi(fields[1])
	if err != nil {
		return process, err
	}
	status, err := readInstalledProcFile(root + "status")
	if err != nil {
		return process, err
	}
	values := make(map[string][]string)
	for _, line := range strings.Split(string(status), "\n") {
		parts := strings.Fields(line)
		if len(parts) > 1 {
			values[parts[0]] = parts[1:]
		}
	}
	if len(values["Uid:"]) != 4 || len(values["SigIgn:"]) != 1 {
		return process, errors.New("process status incomplete")
	}
	process.uid, err = strconv.ParseUint(values["Uid:"][0], 10, 32)
	if err != nil {
		return process, err
	}
	for _, value := range values["Uid:"] {
		if value != values["Uid:"][0] {
			return process, errors.New("process UID changed")
		}
	}
	ignored, err := strconv.ParseUint(values["SigIgn:"][0], 16, 64)
	if err != nil {
		return process, err
	}
	process.ignoresTERM = ignored&(1<<14) != 0
	process.restricted = strings.Join(values["NoNewPrivs:"], " ") == "1" && strings.Join(values["Seccomp:"], " ") == "2" &&
		strings.Join(values["CapEff:"], " ") == "0000000000000000"
	return process, nil
}

func readInstalledProcFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 64<<10))
	if err == nil && len(body) == 64<<10 {
		err = errors.New("process observation oversized")
	}
	return body, err
}

func waitInstalledParentExit(t *testing.T, ctx context.Context, events *os.File, processes []installedHostileProcess, pid uint32) {
	t.Helper()
	var original installedHostileProcess
	for _, process := range processes {
		if process.pid == int(pid) {
			original = process
		}
	}
	if original.pid == 0 {
		t.Fatal("original parent positive control absent")
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		current, err := readInstalledHostileProcess(original.pid)
		exited := os.IsNotExist(err) || err == nil && (current.started != original.started || current.state == "Z" || current.state == "X")
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		gone, populated, err := readTextWorkerCgroup(events)
		if err != nil {
			t.Fatal(err)
		}
		if exited && (gone || !populated) {
			return
		}
		select {
		case <-bounded.Done():
			t.Fatal("initial PID exit did not retire the hostile cgroup without Endpoint stop")
		case <-tick.C:
		}
	}
}
