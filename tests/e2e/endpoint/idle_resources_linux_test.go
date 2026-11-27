//go:build linux && endpoint_launch_native

package endpoint

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNET32CgroupUsageDistinguishesMissingFromZero(t *testing.T) {
	dir := t.TempDir()
	writeNET32File(t, filepath.Join(dir, "memory.current"), "4096\n")
	group, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer group.Close()

	for _, tc := range []struct {
		name, cpuStat, wantError string
		wantCPU                  uint64
	}{
		{name: "missing", cpuStat: "user_usec 0\n", wantError: "usage_usec unavailable"},
		{name: "zero", cpuStat: "usage_usec 0\nuser_usec 0\n"},
		{name: "microseconds", cpuStat: "usage_usec 16\n", wantCPU: 16},
		{name: "malformed", cpuStat: "usage_usec\n", wantError: "malformed"},
		{name: "duplicate", cpuStat: "usage_usec 0\nusage_usec 16\n", wantError: "ambiguous"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeNET32File(t, filepath.Join(dir, "cpu.stat"), tc.cpuStat)
			memory, cpu, err := net32CgroupUsage(group)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("invalid counter returned %d bytes, %d usec, %v", memory, cpu, err)
				}
				return
			}
			if err != nil || memory != 4096 || cpu != tc.wantCPU {
				t.Fatalf("usage = %d bytes, %d usec, %v", memory, cpu, err)
			}
		})
	}
}

func TestNET32ResourcesRejectChangedCohort(t *testing.T) {
	for name, change := range map[string]func(testing.TB, string, string, string){
		"added worker": func(t testing.TB, slice, _, _ string) {
			writeNET32Group(t, filepath.Join(slice, "ardents-text-publisher@new.service"), "2048", "30")
		},
		"removed worker": func(t testing.TB, _, worker, _ string) {
			if err := os.RemoveAll(worker); err != nil {
				t.Fatal(err)
			}
		},
		"reused worker path": func(t testing.TB, slice, worker, _ string) {
			if err := os.Rename(worker, filepath.Join(slice, "old-worker")); err != nil {
				t.Fatal(err)
			}
			writeNET32Group(t, worker, "4096", "50")
		},
		"Endpoint moved": func(t testing.TB, _, _, proc string) {
			writeNET32File(t, filepath.Join(proc, "cgroup"), "0::/system.slice/other.service\n")
		},
		"regressed worker hidden by rising total": func(t testing.TB, slice, worker, _ string) {
			writeNET32File(t, filepath.Join(worker, "cpu.stat"), "usage_usec 19\n")
			writeNET32File(t, filepath.Join(slice, "ardents-endpoint.service", "cpu.stat"), "usage_usec 100\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			proc, mount := t.TempDir(), t.TempDir()
			slice := filepath.Join(mount, "system.slice")
			worker := filepath.Join(slice, "ardents-text-reader@original.service")
			writeNET32File(t, filepath.Join(proc, "cgroup"), "0::/system.slice/ardents-endpoint.service\n")
			writeNET32Group(t, filepath.Join(slice, "ardents-endpoint.service"), "1024", "10")
			writeNET32Group(t, worker, "4096", "20")
			writeNET32Group(t, filepath.Join(slice, "other.service"), "8192", "500")
			resources, err := openNet32ClientResources(proc, mount)
			if err != nil {
				t.Fatal(err)
			}
			defer resources.close()
			if memory, cpu, err := resources.read(); err != nil || memory != 5120 || cpu != 30 {
				t.Fatalf("original cohort = %d bytes, %d usec, %v", memory, cpu, err)
			}
			change(t, slice, worker, proc)
			if memory, cpu, err := resources.read(); err == nil {
				t.Fatalf("changed cohort returned %d bytes, %d usec without an error", memory, cpu)
			}
		})
	}
}

func writeNET32Group(t testing.TB, path, memory, cpu string) {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	writeNET32File(t, filepath.Join(path, "memory.current"), memory+"\n")
	writeNET32File(t, filepath.Join(path, "cpu.stat"), "usage_usec "+cpu+"\n")
}

func writeNET32File(t testing.TB, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestNativeNET32ResourceScopeAndUnits requires a disposable systemd host with
// no installed Ardents units. Its frozen processes make the independent raw
// counter comparison exact without depending on scheduler timing.
func TestNativeNET32ResourceScopeAndUnits(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("NET-32 resource sampling requires a root driver")
	}
	const slice = "/sys/fs/cgroup/system.slice"
	units := []string{
		"ardents-endpoint.service",
		"ardents-text-reader@net32-scope.service",
		"ardents-text-publisher@net32-scope.service",
		"ardents-net32-unrelated.service",
	}
	for _, unit := range units {
		out, err := exec.CommandContext(t.Context(), "systemctl", "show", "-p", "LoadState", "--value", unit).Output()
		if err != nil || strings.TrimSpace(string(out)) != "not-found" {
			t.Fatalf("NET-32 test unit %s already exists or manager unavailable: %q, %v", unit, out, err)
		}
	}
	entries, err := os.ReadDir(slice)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "ardents-text-reader@") || strings.HasPrefix(entry.Name(), "ardents-text-publisher@") {
			t.Fatal("NET-32 sampling fixture requires an empty worker cohort")
		}
	}
	for _, unit := range units {
		runNET32Command(t, t.Context(), "systemd-run", "--quiet", "--collect", "--unit="+unit, "--slice=system.slice",
			"-p", "Type=exec", "-p", "MemoryAccounting=yes", "-p", "CPUAccounting=yes", "/usr/bin/sleep", "120")
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, action := range []string{"thaw", "stop"} {
				if out, err := exec.CommandContext(ctx, "systemctl", action, unit).CombinedOutput(); err != nil {
					t.Errorf("%s %s: %v: %s", action, unit, err, out)
				}
			}
		})
		runNET32Command(t, t.Context(), "systemctl", "freeze", unit)
	}
	pid := runNET32Command(t, t.Context(), "systemctl", "show", "-p", "MainPID", "--value", units[0])
	resources, err := openNet32ClientResources(filepath.Join("/proc", pid), "/sys/fs/cgroup")
	if err != nil {
		t.Fatal(err)
	}
	defer resources.close()

	var expectedMemory, expectedCPU uint64
	for index, unit := range units {
		group := filepath.Join(slice, unit)
		memoryRaw := runNET32Command(t, t.Context(), "cat", filepath.Join(group, "memory.current"))
		cpuRaw := runNET32Command(t, t.Context(), "awk", "$1 == \"usage_usec\" { print $2 }", filepath.Join(group, "cpu.stat"))
		memory, memErr := strconv.ParseUint(memoryRaw, 10, 64)
		cpu, cpuErr := strconv.ParseUint(cpuRaw, 10, 64)
		if err := errors.Join(memErr, cpuErr); err != nil || memory == 0 || cpu == 0 {
			t.Fatalf("raw %s counters = %d bytes, %d usec, %v", unit, memory, cpu, err)
		}
		if index < 3 {
			expectedMemory += memory
			expectedCPU += cpu
		}
		t.Logf("kernel %s: memory.current=%d bytes cpu.stat usage_usec=%d microseconds", unit, memory, cpu)
	}
	memory, cpu, err := resources.read()
	if err != nil || len(resources.groups) != 3 || memory != expectedMemory || cpu != expectedCPU {
		t.Fatalf("sampled %d cgroups: %d bytes and %d usec, want 3 cgroups: %d bytes and %d usec, err=%v",
			len(resources.groups), memory, cpu, expectedMemory, expectedCPU, err)
	}
	t.Logf("sampler scope: Endpoint and two workers, %d bytes and %d microseconds", memory, cpu)
}

func runNET32Command(t testing.TB, ctx context.Context, command string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", command, args, err, out)
	}
	return strings.TrimSpace(string(out))
}
