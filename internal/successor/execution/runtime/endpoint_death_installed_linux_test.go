//go:build linux && amd64 && text_worker_installed

package runtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	"github.com/dianabuilds/ardents-network/internal/successor/execution/worker"
)

const endpointDeathReady = "execution-endpoint-death-ready "

type endpointDeathOpening struct {
	PID            int
	Invocation     string
	Generation     [32]byte
	JobCommitments [2][32]byte
	Workers        [2]worker.Instance
}

// The independently pinned temporary Endpoint unit selects only this actor.
// It exercises real local owners and intentionally cannot report PASS: Root
// kills its original MainPID while both exact operations remain live.
func TestInstalledExecutionEndpointDeathHeldWorkers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint MainPID required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Second)
	defer cancel()
	opening := endpointDeathOpening{PID: os.Getpid(), Invocation: os.Getenv("INVOCATION_ID")}
	if decoded, err := hex.DecodeString(opening.Invocation); err != nil || len(decoded) != 16 {
		t.Fatal("original Endpoint invocation unavailable")
	}
	if _, err := rand.Read(opening.Generation[:]); err != nil {
		t.Fatal(err)
	}
	owner, err := New(execution.Config{ID: opening.Generation, Grants: []execution.Grant{
		{Principal: [32]byte{2}, Surface: execution.Connection},
		{Principal: [32]byte{3}, Surface: execution.Administration},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	for index, surface := range []execution.Surface{execution.Connection, execution.Administration} {
		invocation, operation := launchHostileExecution(t, ctx, owner, [32]byte{byte(index + 2)}, surface)
		_ = pinHostileExecutionTree(t, invocation.activation.Instance)
		if operation.Check() != nil {
			t.Fatal("held operation is unavailable")
		}
		opening.Workers[index] = invocation.activation.Instance
		nonce := invocation.job.Nonce()
		opening.JobCommitments[index] = sha256.Sum256(nonce[:])
	}
	if opening.Workers[0].UID == opening.Workers[1].UID || opening.Workers[0].Cgroup == opening.Workers[1].Cgroup {
		t.Fatal("held workers share an isolation identity")
	}
	raw, err := json.Marshal(opening)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(endpointDeathReady + string(raw))
	<-ctx.Done()
	t.Fatal("Root did not kill the original Endpoint within the held-operation bound")
}

// This root controller is outside the Endpoint unit. Copied actor facts are
// candidates only: actual manager observations and original kernel/proc/pidfd
// descriptors independently bind every process before the uncatchable exit.
func TestInstalledExecutionEndpointDeath(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: external root Endpoint-death controller required")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for path, pin := range map[string]string{
		self: os.Getenv("ARDENTS_TEXT_LIFECYCLE_SHA256"),
		"/run/systemd/system/ardents-endpoint.service":   os.Getenv("ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256"),
		"/usr/lib/ardents/text-worker-root/ardents-text": os.Getenv("ARDENTS_TEXT_HOSTILE_WORKER_SHA256"),
	} {
		body, err := os.ReadFile(path)
		digest := sha256.Sum256(body)
		if err != nil || len(pin) != 64 || hex.EncodeToString(digest[:]) != pin {
			t.Fatalf("invalid environment: independent pinned artifact %s: %v", path, err)
		}
	}
	initial := endpointDeathFacts(t)
	if initial["ActiveState"] != "inactive" || initial["MainPID"] != "0" || initial["FragmentPath"] != "/run/systemd/system/ardents-endpoint.service" || initial["DropInPaths"] != "" {
		t.Fatal("invalid environment: fresh fixed inactive qualification Endpoint required")
	}
	var previous *endpointDeathOpening
	for _, phase := range []string{"initial", "restart"} {
		t.Run(phase, func(t *testing.T) {
			endpointDeathCommand(t, "systemctl", "start", "ardents-endpoint.service")
			started := endpointDeathFacts(t)
			pid, err := strconv.Atoi(started["MainPID"])
			if err != nil || pid <= 1 || started["ActiveState"] != "active" {
				t.Fatal("started Endpoint has no live original MainPID")
			}
			// Linux amd64 ABI values, checked against maintained x/sys source;
			// this qualification test uses no unsafe, cgo or new dependency.
			pidfd, _, errno := syscall.Syscall(434, uintptr(pid), 0, 0)
			if errno != 0 {
				t.Fatalf("original Endpoint pidfd: %v", errno)
			}
			t.Cleanup(func() {
				// A failed observation still retires only this original process,
				// never a replacement selected later by a reused PID or unit name.
				_, _, errno := syscall.Syscall6(424, pidfd, uintptr(syscall.SIGKILL), 0, 0, 0, 0)
				if errno != 0 && errno != syscall.ESRCH {
					t.Errorf("original Endpoint fallback retirement: %v", errno)
				}
				if err := syscall.Close(int(pidfd)); err != nil {
					t.Error(err)
				}
			})
			original, opening := awaitEndpointDeathOpening(t)
			if original["MainPID"] != started["MainPID"] || original["InvocationID"] != started["InvocationID"] {
				t.Fatal("started original Endpoint changed during readiness")
			}
			if previous != nil {
				if opening.Invocation == previous.Invocation || opening.Generation == previous.Generation {
					t.Fatal("restart inherited an original invocation or local generation")
				}
				for index := range opening.Workers {
					if opening.Workers[index].Invocation == previous.Workers[index].Invocation || opening.JobCommitments[index] == previous.JobCommitments[index] {
						t.Fatal("restart inherited a worker invocation or Job nonce")
					}
				}
			}
			endpoint := pinEndpointDeathProcess(t, original, opening)
			var trees [2]*hostileExecutionTree
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			for index, candidate := range opening.Workers {
				actual, err := worker.ObserveInstance(ctx, candidate.Name, candidate.Role)
				if err != nil || actual != candidate {
					t.Fatalf("independent original worker observation: %v", err)
				}
				trees[index] = pinHostileExecutionTree(t, actual)
			}
			current := endpointDeathFacts(t)
			if current["MainPID"] != original["MainPID"] || current["InvocationID"] != original["InvocationID"] || current["ActiveState"] != "active" {
				t.Fatal("original Endpoint changed before pidfd SIGKILL")
			}
			_, _, errno = syscall.Syscall6(424, pidfd, uintptr(syscall.SIGKILL), 0, 0, 0, 0)
			if errno != 0 {
				t.Fatalf("original pidfd SIGKILL: %v", errno)
			}
			deadline := time.Now().Add(15 * time.Second)
			for {
				joined := endpointDeathJoined(endpoint)
				for _, tree := range trees {
					joined = joined && endpointDeathJoined(tree)
				}
				if joined {
					break
				}
				if !time.Now().Before(deadline) {
					t.Fatal("original Endpoint or hostile descendants survived SIGKILL/BindsTo")
				}
				time.Sleep(25 * time.Millisecond)
			}
			endpoint.requireJoined(t)
			for _, tree := range trees {
				tree.requireJoined(t)
			}
			terminal := endpointDeathFacts(t)
			if terminal["MainPID"] != "0" || terminal["ActiveState"] != "failed" || terminal["Result"] != "signal" || terminal["ExecMainCode"] != "2" || terminal["ExecMainStatus"] != "9" || terminal["InvocationID"] != opening.Invocation {
				t.Fatalf("original Endpoint did not retain its SIGKILL result: %v", terminal)
			}
			previous = &opening
			t.Logf("original Endpoint %s SIGKILL retained; both original hostile trees joined without Go cleanup", opening.Invocation)
		})
		if t.Failed() {
			break
		}
	}
}

func endpointDeathCommand(t *testing.T, program string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	body, err := exec.CommandContext(ctx, program, args...).CombinedOutput()
	if err != nil || len(body) > 64<<10 {
		t.Fatalf("Endpoint-death observation %s: %v (%s)", program, err, body)
	}
	return string(body)
}

func endpointDeathFacts(t *testing.T) map[string]string {
	t.Helper()
	body := endpointDeathCommand(t, "systemctl", "show", "ardents-endpoint.service", "-p", "MainPID", "-p", "InvocationID", "-p", "ControlGroup", "-p", "ActiveState", "-p", "Result", "-p", "ExecMainCode", "-p", "ExecMainStatus", "-p", "FragmentPath", "-p", "DropInPaths")
	facts := make(map[string]string)
	for _, line := range strings.Split(body, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			facts[key] = value
		}
	}
	return facts
}

func awaitEndpointDeathOpening(t *testing.T) (map[string]string, endpointDeathOpening) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		facts := endpointDeathFacts(t)
		journal := endpointDeathCommand(t, "journalctl", "--no-pager", "-o", "cat", "-n", "50", "_SYSTEMD_INVOCATION_ID="+facts["InvocationID"])
		if facts["ActiveState"] != "active" {
			t.Fatalf("held Endpoint is not live: %v; original journal: %s", facts, journal)
		}
		var opening endpointDeathOpening
		found := false
		for _, line := range strings.Split(journal, "\n") {
			if _, raw, ok := strings.Cut(line, endpointDeathReady); ok {
				if found || json.Unmarshal([]byte(raw), &opening) != nil {
					t.Fatal("held Endpoint has duplicate or invalid readiness input")
				}
				found = true
			}
		}
		if found {
			if strconv.Itoa(opening.PID) != facts["MainPID"] || opening.Invocation != facts["InvocationID"] || opening.Generation == [32]byte{} || opening.JobCommitments[0] == [32]byte{} || opening.JobCommitments[1] == [32]byte{} || opening.Workers[0].Role != "reader" || opening.Workers[1].Role != "publisher" {
				t.Fatal("held facts do not match original Endpoint and separate operations")
			}
			return facts, opening
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("qualified held workers not observed: %s", journal)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func pinEndpointDeathProcess(t *testing.T, facts map[string]string, opening endpointDeathOpening) *hostileExecutionTree {
	t.Helper()
	if facts["ControlGroup"] != "/system.slice/ardents-endpoint.service" {
		t.Fatal("original Endpoint cgroup is not canonical")
	}
	events, err := os.Open("/sys/fs/cgroup" + facts["ControlGroup"] + "/cgroup.events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = events.Close() })
	var filesystem syscall.Statfs_t
	if syscall.Fstatfs(int(events.Fd()), &filesystem) != nil || filesystem.Type != 0x63677270 {
		t.Fatal("original Endpoint event observer is not cgroup v2")
	}
	process, err := os.Open(fmt.Sprintf("/proc/%d", opening.PID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Close() })
	status, err := hostileExecutionProcRead(process, "status")
	if err != nil || strings.Contains(string(status), "\nUid:\t0\t") {
		t.Fatalf("original Endpoint proc identity unavailable: %v", err)
	}
	environment, err := hostileExecutionProcRead(process, "environ")
	if err != nil || !strings.Contains("\x00"+string(environment), "\x00INVOCATION_ID="+opening.Invocation+"\x00") {
		t.Fatalf("original Endpoint proc invocation unavailable: %v", err)
	}
	tree := &hostileExecutionTree{events: events, processes: []*os.File{process}}
	if removed, populated, err := tree.observe(); err != nil || removed || !populated {
		t.Fatal("original Endpoint scope positive control is not populated")
	}
	return tree
}

func endpointDeathJoined(tree *hostileExecutionTree) bool {
	removed, populated, err := tree.observe()
	if err != nil || !removed && populated {
		return false
	}
	for _, process := range tree.processes {
		_, err := hostileExecutionProcRead(process, "stat")
		if !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}
	return true
}
