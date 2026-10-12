//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The root test driver stays outside every measured Node cgroup. Each child is
// the genuine canonical command, independently hash-pinned by the native runner.
// Signed fixture inputs and genuine Stock/State/Hosting supply every success;
// no successful role, producer, grant or process observation is substituted.
func nodeNativeCommand(t *testing.T) string {
	t.Helper()
	path := os.Getenv("ARDENTS_NODE_NATIVE_COMMAND")
	want := os.Getenv("ARDENTS_NODE_NATIVE_COMMAND_SHA256")
	if os.Geteuid() != 0 || !filepath.IsAbs(path) || len(want) != 64 {
		t.Fatal("selected Node native root driver and pinned command unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != want {
		t.Fatal("selected native command digest differs")
	}
	return path
}

type nativeForwardingProcess struct {
	unit       string
	command    *exec.Cmd
	stdout     io.ReadCloser
	diagnostic nativeNodeDiagnostic
	pid        int
	joined     bool
	signaled   bool
}

type nativeNodeDiagnostic struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (diagnostic *nativeNodeDiagnostic) Write(raw []byte) (int, error) {
	diagnostic.mu.Lock()
	defer diagnostic.mu.Unlock()
	return diagnostic.buffer.Write(raw)
}
func (diagnostic *nativeNodeDiagnostic) String() string {
	diagnostic.mu.Lock()
	defer diagnostic.mu.Unlock()
	return diagnostic.buffer.String()
}

func startNativeForwarding(t *testing.T, binary, plan string, index byte) *nativeForwardingProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-forwarding-%d-%d.service", os.Getpid(), index)
	process := &nativeForwardingProcess{unit: unit}
	process.command = exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "forwarding", "--config", plan)
	process.command.Stderr = &process.diagnostic
	var err error
	process.stdout, err = process.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !process.joined {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = process.command.Wait()
			_ = process.stdout.Close()
			process.joined = true
		}
	})
	var event map[string]string
	if err := json.NewDecoder(process.stdout).Decode(&event); err != nil || event["operation"] != "node.forwarding" || event["phase"] != "listening" {
		t.Fatal("genuine original Node startup refused", event, err, process.diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &process.pid); err != nil || process.pid < 2 {
		t.Fatal("actual Node MainPID unavailable", err)
	}
	return process
}

func (process *nativeForwardingProcess) stop(t *testing.T, refusedExchange ...bool) {
	t.Helper()
	if process.joined {
		return
	}
	if !process.signaled {
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGINT", process.unit).Run(); err != nil {
			t.Fatal("original Node unavailable before explicit stop", err, process.diagnostic.String())
		}
		process.signaled = true
	}
	err := process.command.Wait()
	process.joined = true
	_ = process.stdout.Close()
	var exit *exec.ExitError
	var event map[string]string
	decodeErr := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String())).Decode(&event)
	if !errors.As(err, &exit) || decodeErr != nil || event["phase"] != "joined" {
		t.Fatal("original Node did not report known joined retirement", err, decodeErr, process.diagnostic.String())
	}
	if exit.ExitCode() != 130 || event["outcome"] != "canceled" {
		// The deliberate pre-spend refusal can leave an actual Route physical
		// cleanup failure. It must remain a failed joined result, never EXIT or
		// clean cancellation. All positive cancellation rows still require 130.
		if len(refusedExchange) == 0 || !refusedExchange[0] || exit.ExitCode() != 1 || event["outcome"] != "failed" || event["first_owner"] != "caller" || event["cleanup_owner"] != "route" {
			t.Fatal("original Node did not retain the expected cancellation or refusal cleanup", err, process.diagnostic.String())
		}
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", process.pid)); !os.IsNotExist(err) {
		t.Fatal("original Node process remains after manager join", err)
	}
}

func TestNativeNodeForwardingBothCarriers(t *testing.T) {
	for _, retirement := range []string{"cancel", "protection-loss", "clock-loss", "undercoverage"} {
		t.Run(retirement, func(t *testing.T) { testNativeNodeForwarding(t, retirement) })
	}
}

func (process *nativeForwardingProcess) failedJoin(t *testing.T, expectedOwner string) {
	t.Helper()
	err := process.command.Wait()
	process.joined = true
	_ = process.stdout.Close()
	var exit *exec.ExitError
	var event map[string]string
	decodeErr := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String())).Decode(&event)
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || decodeErr != nil || event["phase"] != "joined" || event["outcome"] != "failed" || event["first_owner"] != expectedOwner {
		t.Fatal("original Node did not retain failed physical completion", err, process.diagnostic.String())
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", process.pid)); !os.IsNotExist(err) {
		t.Fatal("failed original Node remains after manager join", err)
	}
}

func testNativeNodeForwarding(t *testing.T, retirement string) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			seed := func() networkAuthorityPlan {
				var key [32]byte
				copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
				plan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "network"), NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
				config, err := networkStateConfig(&plan)
				if err != nil {
					t.Fatal(err)
				}
				owner, err := state.Open(config)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := owner.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
					t.Fatal(err)
				}
				if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
					t.Fatal(err)
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
				return plan
			}
			var processes []*nativeForwardingProcess
			var budgets []string
			var participation []string
			var spendRoots []string
			var initialSpends [][]byte
			for index := byte(12); index < 16; index++ {
				id := [32]byte{index}
				certificate := certificates[id]
				certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
				if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
					t.Fatal(err)
				}
				key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
				if err != nil {
					t.Fatal(err)
				}
				encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
				if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
					t.Fatal(err)
				}
				clear(key)
				clear(encoded)
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				root := filepath.Join(t.TempDir(), "participation")
				participation = append(participation, root)
				spend := t.TempDir()
				spendRoots = append(spendRoots, spend)
				work := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				if retirement == "undercoverage" {
					work = hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}
				}
				plan := map[string]any{"network": seed(), "node_id": id, "process_profile": processresource.Profile, "participation_root": root, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				sockets[id]()
				processes = append(processes, startNativeForwarding(t, binary, hostingConfig(t, plan), index))
				before, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
				if err != nil || len(before) != 8+32+32+32+8 {
					t.Fatal("fresh real receiving ledger is not an empty canonical header", err)
				}
				initialSpends = append(initialSpends, before)
			}
			holder := routeStock(t, f)
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 3, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := startRoutePrefix(context.Background(), plan, f.authority, holder)
			if retirement == "undercoverage" {
				if err == nil {
					_ = prefix.close()
					t.Fatal("undersized actual Hosting admitted the complete Forward allowance")
				}
			} else if err != nil {
				t.Fatal("actual Forwarding prefix failed", err)
			} else if err := prefix.close(); err != nil {
				t.Fatal("actual Forwarding physical join failed", err)
			}
			switch retirement {
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", processes[0].unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				processes[0].failedJoin(t, "process")
			case "clock-loss":
				stopClock()
				if err := os.Remove(clock); err != nil {
					t.Fatal(err)
				}
				for _, process := range processes {
					process.failedJoin(t, "network")
				}
			}
			// Retire every original caller before waiting for any peer's join.
			// Sequential stop-and-wait would manufacture peer loss in still-live
			// invocations and mix that failure with the cancellation control.
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, process := range processes {
				if !process.joined {
					signals = append(signals, process.unit)
				}
			}
			if len(signals) > 3 {
				if err := exec.Command("systemctl", signals...).Run(); err != nil {
					t.Fatal("original caller batch cancellation unavailable", err)
				}
				for _, process := range processes {
					process.signaled = true
				}
			}
			for index, process := range processes {
				// A forced protection loss also closes its real peer channels;
				// adjacent invocations retain any resulting Route cleanup failure.
				process.stop(t, retirement == "undercoverage" || retirement == "protection-loss")
				if retirement == "undercoverage" {
					after, err := os.ReadFile(filepath.Join(spendRoots[index], "closed-token-spends"))
					if err != nil || !bytes.Equal(initialSpends[index], after) {
						t.Fatal("pre-spend physical coverage refusal changed real durable spend", err)
					}
				}
				view, err := f.current()
				if err != nil {
					t.Fatal(err)
				}
				member, err := view.Member([32]byte{byte(12 + index)}, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation[index], Clock: time.Now})
				if err != nil {
					t.Fatal("participation unavailable after known join", err)
				}
				conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
				if closeErr := journal.Close(); err != nil || closeErr != nil || conflict {
					t.Fatal("joined original producer still conflicts", err, closeErr, conflict)
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observation, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observation.ReservedBytes != 0 {
					t.Fatal("joined original Node retained or invented capacity", err, closeErr, observation.ReservedBytes)
				}
			}
		})
	}
}
