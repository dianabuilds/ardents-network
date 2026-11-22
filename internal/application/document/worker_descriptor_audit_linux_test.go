//go:build linux

package document

import (
	"os"
	"syscall"
	"testing"
)

func TestDescriptorSurvivedExecRejectsOnlyNonCloseOnExec(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()

	closeOnExec, err := descriptorSurvivedExec(int(read.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if closeOnExec {
		t.Fatal("a close-on-exec descriptor was treated as inherited")
	}

	foreign, err := syscall.Dup(int(read.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(foreign)
	survived, err := descriptorSurvivedExec(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if !survived {
		t.Fatal("a descriptor without close-on-exec was not treated as inherited")
	}
}

// The manager-passed control policy is pure path/name reasoning: only a
// resource-control file directly inside the worker's OWN unified cgroup is
// admitted. Migration, controller-enablement and lifecycle surfaces, foreign
// cgroups, prefix spoofs and nested paths remain foreign authority.
func TestManagerCgroupControlTargetPolicy(t *testing.T) {
	own := "/system.slice/ardents-text-reader@0-123-988.service"
	leaf := "/sys/fs/cgroup" + own + "/"
	for _, test := range []struct {
		name   string
		target string
		admit  bool
	}{
		{"own cpu.max", leaf + "cpu.max", true},
		{"own memory.max", leaf + "memory.max", true},
		{"own pids.max", leaf + "pids.max", true},
		{"own io.weight", leaf + "io.weight", true},
		{"root control is foreign to a service cgroup", "/sys/fs/cgroup/cpu.max", false},
		{"container root own control", "/sys/fs/cgroup/cpu.max", true},
		{"foreign unit control", "/sys/fs/cgroup/system.slice/ardents-text-reader@1-123-988.service/cpu.max", false},
		{"parent slice control", "/sys/fs/cgroup/system.slice/cpu.max", false},
		{"prefix spoof", leaf + ".evil/cpu.max", false},
		{"foreign cgroup root", "/sys/fs/cgroup-evil" + own + "/cpu.max", false},
		{"migration surface", leaf + "cgroup.procs", false},
		{"thread migration surface", leaf + "cgroup.threads", false},
		{"controller enablement", leaf + "cgroup.subtree_control", false},
		{"cgroup type", leaf + "cgroup.type", false},
		{"lifecycle kill", leaf + "cgroup.kill", false},
		{"lifecycle freeze", leaf + "cgroup.freeze", false},
		{"events observation", leaf + "cgroup.events", false},
		{"child cgroup control", leaf + "child/cpu.max", false},
		{"own directory", leaf, false},
		{"non-cgroup file", "/etc/passwd", false},
		{"traversal", "/sys/fs/cgroup/system.slice/../../etc/cpu.max", false},
		{"unknown control name", leaf + "cpu.max.evil", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ownPath := own
			if test.name == "container root own control" {
				ownPath = "/"
			}
			if managerCgroupControlTarget(test.target, ownPath) != test.admit {
				t.Fatalf("target %q admission with own cgroup %q differs", test.target, ownPath)
			}
		})
	}
}
