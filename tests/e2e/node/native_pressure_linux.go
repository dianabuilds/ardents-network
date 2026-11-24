//go:build linux && node_process_native

package node

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Create genuine scheduling pressure after successful work, without changing
// the selected cgroup/Go/FD envelope or placing another process in the role leaf.
func ExerciseNativeCPUPressure(t testing.TB, original *NativeProcess) {
	t.Helper()
	if _, err := exec.LookPath("taskset"); err != nil {
		t.Fatal("selected native CPU-affinity prerequisite unavailable", err)
	}
	proc := filepath.Join("/proc", strconv.Itoa(original.PID))
	status, err := os.ReadFile(filepath.Join(proc, "status"))
	if err != nil {
		t.Fatal(err)
	}
	cpu := ""
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "Cpus_allowed_list:") {
			cpu = strings.Split(strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:")), ",")[0], "-")[0]
		}
	}
	if _, err := strconv.ParseUint(cpu, 10, 32); err != nil {
		t.Fatal("original allowed CPU unavailable", err)
	}
	groupRaw, err := os.ReadFile(filepath.Join(proc, "cgroup"))
	if err != nil {
		t.Fatal(err)
	}
	group := strings.TrimSpace(string(groupRaw))
	if !strings.HasPrefix(group, "0::/") || strings.Contains(group, "\n") {
		t.Fatal("original sole cgroup unavailable")
	}
	root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(group, "0::/"))
	maximum, err := os.ReadFile(filepath.Join(root, "cpu.max"))
	if err != nil || string(maximum) != "100000 100000\n" {
		t.Fatal("original CPU envelope changed before pressure", string(maximum), err)
	}
	pressure := func() float64 {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "cpu.pressure"))
		if err != nil {
			t.Fatal("original pressure unavailable", err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 5 && fields[0] == "some" && strings.HasPrefix(fields[1], "avg10=") {
				value, err := strconv.ParseFloat(strings.TrimPrefix(fields[1], "avg10="), 64)
				if err != nil || value < 0 || value > 100 {
					t.Fatal("actual pressure invalid", err)
				}
				return value
			}
		}
		t.Fatal("original pressure grammar unavailable")
		return 0
	}
	if pressure() != 0 {
		t.Fatal("pressure already present before causal control")
	}
	if output, err := exec.Command("taskset", "-apc", cpu, strconv.Itoa(original.PID)).CombinedOutput(); err != nil {
		t.Fatal("original allowed CPU affinity unavailable", err, string(output))
	}
	// Twelve bounded helper groups contend on ONE original allowed CPU. The
	// measured development control uses at most one busy CPU, 384 MiB declared
	// auxiliary memory and 24 helper tasks, outside the sole original leaf.
	// Positive PSI alone is not the selected 1% overload criterion.
	for index := range 12 {
		unit := fmt.Sprintf("ardents-descriptor-cpu-helper-%d-%d.service", os.Getpid(), index)
		// Independent manager limits survive a forced exit of the test driver.
		// These are test-helper limits, never the measured Node's envelope.
		helper := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
			"--property=RuntimeMaxSec=8s", "--property=CPUQuota=50%", "--property=MemoryMax=32M", "--property=TasksMax=2", "--property=LimitNOFILE=64",
			"--property=DynamicUser=yes", "--property=NoNewPrivileges=yes", "--property=PrivateTmp=yes",
			"--", "taskset", "-c", cpu, "sh", "-c", "while :; do :; done")
		if err := helper.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run(); _ = helper.Wait() })
		helperPID := 0
		readyUntil := time.Now().Add(time.Second)
		for helperPID == 0 {
			raw, err := exec.Command("systemctl", "show", unit, "-p", "MainPID", "--value").Output()
			if err == nil {
				helperPID, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			}
			if helperPID > 1 {
				break
			}
			if time.Now().After(readyUntil) {
				t.Fatal("bounded original CPU helper unavailable")
			}
			helperPID = 0
			time.Sleep(5 * time.Millisecond)
		}
		helperGroup, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(helperPID), "cgroup"))
		if err != nil || bytes.Equal(helperGroup, groupRaw) {
			t.Fatal("CPU helper entered original role leaf", err)
		}
	}
	deadline := time.Now().Add(6 * time.Second)
	for pressure() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("causal original CPU pressure did not become measured")
		}
		time.Sleep(5 * time.Millisecond)
	}
	retained, err := os.ReadFile(filepath.Join(root, "cpu.max"))
	if err != nil || !bytes.Equal(retained, maximum) {
		t.Fatal("pressure control changed required CPU envelope", err)
	}
}
