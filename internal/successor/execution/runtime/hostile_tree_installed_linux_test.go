//go:build linux && text_worker_installed

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

// The separately root-pinned hostile Text artifact forks before READY. Its
// child and grandchild retain the attachment and ignore TERM. This exercises
// genuine new launch/Job/operation/cleanup owners, without Service authority.
func TestInstalledExecutionHostileTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint MainPID required")
	}
	for _, surface := range []execution.Surface{execution.Connection, execution.Administration} {
		t.Run(string(surface), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			principal, siblingPrincipal := [32]byte{2}, [32]byte{3}
			siblingSurface := execution.Administration
			if surface == execution.Administration {
				siblingSurface = execution.Connection
			}
			owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{
				{Principal: principal, Surface: surface},
				{Principal: siblingPrincipal, Surface: siblingSurface},
			}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := owner.Close(); err != nil {
					t.Errorf("original owner cleanup: %v", err)
				}
			})
			sibling, siblingOperation := launchHostileExecution(t, ctx, owner, siblingPrincipal, siblingSurface)
			siblingTree := pinHostileExecutionTree(t, sibling.activation.Instance)
			victim, operation := launchHostileExecution(t, ctx, owner, principal, surface)
			victimTree := pinHostileExecutionTree(t, victim.activation.Instance)
			if victim.activation.Instance.UID == sibling.activation.Instance.UID ||
				victim.activation.Instance.Cgroup == sibling.activation.Instance.Cgroup {
				t.Fatal("live hostile siblings share an isolation identity")
			}
			operation.Close()
			if err := victim.Close(); err != nil {
				t.Fatal(err)
			}
			victimTree.requireJoined(t)
			if !victim.CompletedCurrent() || operation.Check() == nil {
				t.Fatal("joined victim result or revoked operation is incorrect")
			}
			if siblingOperation.Check() != nil || sibling.CompletedCurrent() {
				t.Fatal("victim retirement revoked or completed the live sibling")
			}
			observed, err := worker.ObserveInstance(ctx, sibling.activation.Instance.Name, sibling.activation.Instance.Role)
			if err != nil || observed != sibling.activation.Instance {
				t.Fatalf("victim cleanup changed original sibling invocation: %v", err)
			}
			if removed, populated, err := siblingTree.observe(); err != nil || removed || !populated {
				t.Fatalf("original sibling tree did not survive: %v", err)
			}
			siblingOperation.Close()
			if err := sibling.Close(); err != nil {
				t.Fatal(err)
			}
			siblingTree.requireJoined(t)

			// Peer loss retires the exact new Job before its operation joins.
			// The original descriptors, rather than a replacement pathname or PID,
			// remain the independent physical completion observers.
			replacement, replacementOperation := launchHostileExecution(t, ctx, owner, principal, surface)
			replacementTree := pinHostileExecutionTree(t, replacement.activation.Instance)
			if replacement.job == victim.job || replacement.activation.Instance.Invocation == victim.activation.Instance.Invocation {
				t.Fatal("fresh launch inherited retired Job or physical invocation")
			}
			if err := replacement.activation.Attachment.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-replacementOperation.Context().Done():
			case <-ctx.Done():
				t.Fatal("original attachment loss retained operation permission")
			}
			if replacementOperation.Check() == nil {
				t.Fatal("lost attachment still permits effects")
			}
			select {
			case <-replacement.done:
				t.Fatal("hostile invocation completed before its operation joined")
			default:
			}
			replacementOperation.Close()
			if err := replacement.Close(); !errors.Is(err, errUnexpectedWorkerEnd) {
				t.Fatalf("original peer-loss result: %v", err)
			}
			replacementTree.requireJoined(t)
			if replacement.CompletedCurrent() {
				t.Fatal("failed worker use became a successful joined result")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if victim.CompletedCurrent() || sibling.CompletedCurrent() {
				t.Fatal("closed local generation retained completed-current permission")
			}
			t.Log("original hostile trees joined; sibling invocation and operation survived victim retirement; attachment loss denied effects before operation join")
		})
	}
}

func launchHostileExecution(t *testing.T, ctx context.Context, owner *Owner, principal [32]byte, surface execution.Surface) (*Invocation, *Operation) {
	t.Helper()
	invocation, err := owner.Launch(ctx, principal, surface)
	if err != nil {
		t.Fatal(err)
	}
	var operation *Operation
	t.Cleanup(func() {
		if operation != nil {
			operation.Close()
		}
		if err := invocation.Close(); err != nil && !errors.Is(err, errUnexpectedWorkerEnd) {
			t.Errorf("original hostile invocation cleanup: %v", err)
		}
	})
	operation, err = invocation.BeginOperation(ctx)
	if err != nil || operation.Check() != nil {
		t.Fatalf("qualified hostile operation unavailable: %v", err)
	}
	return invocation, operation
}

type hostileExecutionTree struct {
	events    *os.File
	processes []*os.File
}

func pinHostileExecutionTree(t *testing.T, instance worker.Instance) *hostileExecutionTree {
	t.Helper()
	group, err := os.OpenRoot("/sys/fs/cgroup" + instance.Cgroup)
	if err != nil {
		t.Fatal(err)
	}
	defer group.Close()
	events, err := group.Open("cgroup.events")
	if err != nil {
		t.Fatal(err)
	}
	tree := &hostileExecutionTree{events: events}
	t.Cleanup(func() {
		for _, process := range tree.processes {
			if err := process.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := tree.events.Close(); err != nil {
			t.Error(err)
		}
	})
	var filesystem syscall.Statfs_t
	if syscall.Fstatfs(int(events.Fd()), &filesystem) != nil || filesystem.Type != 0x63677270 {
		t.Fatal("original event observer is not a kernel cgroup-v2 descriptor")
	}
	procs, err := group.Open("cgroup.procs")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(io.LimitReader(procs, 4096))
	if err := errors.Join(readErr, procs.Close()); err != nil || len(body) == 4096 {
		t.Fatalf("bounded original process inventory unavailable: %v", err)
	}
	parentByPID := make(map[int]int)
	for _, value := range strings.Fields(string(body)) {
		pid, err := strconv.Atoi(value)
		if err != nil || pid <= 0 {
			t.Fatal("kernel process inventory has an invalid PID")
		}
		process, err := os.Open(fmt.Sprintf("/proc/%d", pid))
		if err != nil {
			t.Fatal(err)
		}
		tree.processes = append(tree.processes, process)
		stat, err := hostileExecutionProcRead(process, "stat")
		if err != nil {
			t.Fatal(err)
		}
		end := strings.LastIndex(string(stat), ")")
		if end < 0 {
			t.Fatal("original process stat unavailable")
		}
		fields := strings.Fields(string(stat[end+1:]))
		if len(fields) < 20 || fields[0] == "Z" || fields[0] == "X" {
			t.Fatal("hostile tree contains no live original process")
		}
		parent, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		parentByPID[pid] = parent
		status, err := hostileExecutionProcRead(process, "status")
		if err != nil {
			t.Fatal(err)
		}
		values := make(map[string][]string)
		for _, line := range strings.Split(string(status), "\n") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				values[parts[0]] = parts[1:]
			}
		}
		uid := strconv.FormatUint(uint64(instance.UID), 10)
		if len(values["Uid:"]) != 4 || strings.Join(values["Uid:"], " ") != uid+" "+uid+" "+uid+" "+uid ||
			strings.Join(values["NoNewPrivs:"], " ") != "1" || strings.Join(values["Seccomp:"], " ") != "2" ||
			strings.Join(values["CapEff:"], " ") != "0000000000000000" || len(values["SigIgn:"]) != 1 {
			t.Fatal("hostile descendant escaped the observed worker identity or hardening")
		}
		ignored, err := strconv.ParseUint(values["SigIgn:"][0], 16, 64)
		if err != nil || pid != int(instance.PID) && ignored&(1<<14) == 0 {
			t.Fatal("hostile descendant positive control does not ignore TERM")
		}
	}
	if len(parentByPID) != 3 {
		t.Fatal("invalid hostile artifact: exact parent, child and grandchild required")
	}
	if _, present := parentByPID[int(instance.PID)]; !present {
		t.Fatal("original manager MainPID is absent from hostile tree")
	}
	var child int
	for pid, parent := range parentByPID {
		if parent == int(instance.PID) {
			child = pid
		}
	}
	var grandchild bool
	for _, parent := range parentByPID {
		grandchild = grandchild || child != 0 && parent == child
	}
	if !grandchild {
		t.Fatal("original hostile parent/child/grandchild lineage unavailable")
	}
	if removed, populated, err := tree.observe(); err != nil || removed || !populated {
		t.Fatal("original hostile cgroup positive control is not populated")
	}
	return tree
}

func hostileExecutionProcRead(process *os.File, name string) ([]byte, error) {
	fd, err := syscall.Openat(int(process.Fd()), name, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "original hostile process observation")
	body, readErr := io.ReadAll(io.LimitReader(file, 64<<10))
	err = errors.Join(readErr, file.Close())
	if len(body) == 64<<10 {
		err = errors.Join(err, errors.New("original process observation oversized"))
	}
	return body, err
}

func (tree *hostileExecutionTree) observe() (removed, populated bool, result error) {
	if _, err := tree.events.Seek(0, io.SeekStart); err != nil {
		if errors.Is(err, syscall.ENODEV) {
			return true, false, nil
		}
		return false, false, err
	}
	body, err := io.ReadAll(io.LimitReader(tree.events, 4096))
	if errors.Is(err, syscall.ENODEV) {
		return true, false, nil
	}
	if err != nil || len(body) == 4096 {
		return false, false, errors.Join(err, errors.New("original cgroup observation unavailable"))
	}
	for _, line := range strings.Split(string(body), "\n") {
		if line == "populated 0" || line == "populated 1" {
			return false, line == "populated 1", nil
		}
	}
	return false, false, errors.New("original kernel populated observation absent")
}

func (tree *hostileExecutionTree) requireJoined(t *testing.T) {
	t.Helper()
	if removed, populated, err := tree.observe(); err != nil || !removed && populated {
		t.Fatalf("original hostile tree not joined: %v", err)
	}
	for _, process := range tree.processes {
		stat, err := hostileExecutionProcRead(process, "stat")
		if errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		end := strings.LastIndex(string(stat), ")")
		fields := strings.Fields(string(stat[end+1:]))
		if end < 0 || len(fields) == 0 || fields[0] != "Z" && fields[0] != "X" {
			t.Fatal("original hostile process survived physical cleanup")
		}
	}
}
