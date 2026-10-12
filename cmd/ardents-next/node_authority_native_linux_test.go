//go:build linux && node_process_native

package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// This is a native composition test, not a replacement command or installed
// qualification. The ordinary command scenarios separately prove that consumer.
// The root driver prepares signed inputs outside the measured cgroup; the child
// opens genuine original State/Node/participation/Admission/Hosting owners.
// The only injected mechanism is scheduling the return of an actual completed
// State observation. No successful authority, role or native reading is supplied.
type nativeOriginalAuthorityPlan struct {
	Network                        networkAuthorityPlan
	Participation, Spend, Hosting  string
	Certificate, PrivateKey, Fault string
	Successor                      networkfixture.Closed
	Conflict                       []byte
}

func TestNativeNodeOriginalAuthorityAndJoinBothCarriers(t *testing.T) {
	if path := os.Getenv("ARDENTS_NODE_ORIGINAL_FIXTURE"); path != "" {
		testNativeOriginalAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "stale-measurement", "successor", "profile-conflict", "member-expiry", "startup-cancel"} {
				t.Run(fault, func(t *testing.T) {
					f, sockets, certificates := newRouteFixture(t, carrier)
					if fault == "member-expiry" {
						for index := range f.spec.Nodes {
							if f.spec.Nodes[index].NodeID == [32]byte{12} {
								f.spec.Nodes[index].ValidUntil = time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)
							}
						}
					}
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					plan := nativeOriginalAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Certificate: filepath.Join(t.TempDir(), "certificate"), PrivateKey: filepath.Join(t.TempDir(), "key"), Fault: fault}
					certificate := certificates[[32]byte{12}]
					if err := os.WriteFile(plan.Certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
						t.Fatal(err)
					}
					key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
					if err != nil {
						t.Fatal(err)
					}
					encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
					if err := os.WriteFile(plan.PrivateKey, encoded, 0600); err != nil {
						t.Fatal(err)
					}
					clear(key)
					clear(encoded)
					next := f.spec
					next.Number++
					next.Previous = bundle.Epoch.Digest
					next.Seed[0]++
					plan.Successor, err = networkfixture.BuildClosed(next)
					if err != nil {
						t.Fatal(err)
					}
					conflict := f.spec
					conflict.IssuanceAuthority[0] ^= 1
					changed, err := networkfixture.BuildClosed(conflict)
					if err != nil || changed.Epoch.Digest != bundle.Epoch.Digest {
						t.Fatal("conflict changed the original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := hostingConfig(t, plan)
					sockets[[32]byte{12}]()
					unit := fmt.Sprintf("ardents-forwarding-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
						"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
						"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_NODE_ORIGINAL_FIXTURE="+path,
						"--", binary, "-test.v=true", "-test.run=^TestNativeNodeOriginalAuthorityAndJoinBothCarriers$", "-test.timeout=30s")
					output, err := command.CombinedOutput()
					if err != nil {
						t.Fatalf("genuine original native composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func testNativeOriginalAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeOriginalAuthorityPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	config, err := networkStateConfig(&plan.Network)
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = plan.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error("original State close", err)
		}
	})
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{12}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held atomic.Bool
	var canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && nativeOriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			// Cancel after an actual listener effect and a completed real State
			// read, before the original consumer's successful final handoff.
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	original, err := node.ListenForwarding(ctx, node.ForwardingConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey,
		Work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled acquisition returned an accepting original")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late startup cancellation lost its original cause", err)
		}
		checkNativeFailedStartupCleanup(t, plan, member, binding)
		return
	}
	if err != nil {
		t.Fatal("original native Node refused real startup", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real original State observation never entered the held return")
	}
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous Node denial waited for foreign authority I/O")
		}
	case "caller":
		cancel()
	case "stale-measurement":
		timer := time.NewTimer(1200 * time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
		timer.Stop()
	case "successor":
		if _, err := owner.Accept(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("genuine successor preserved original duty", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted as original")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("genuine State conflict supplied authority")
		}
	case "member-expiry":
		timer := time.NewTimer(time.Until(member.NotAfter()) + 150*time.Millisecond)
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
		timer.Stop()
	default:
		t.Fatal("unselected original authority fault", plan.Fault)
	}
	// A delayed original watcher is still a physical borrower. A bounded wait
	// cannot finish the Node, remove its producer or release its spend root.
	select {
	case <-original.Done():
		t.Fatal("original completion was published before held work joined")
	case <-time.After(30 * time.Millisecond):
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || !collision {
		t.Fatal("unjoined original producer disappeared", collision, err, closeErr)
	}
	if ledger, err := spending.Open(plan.Spend, binding); err == nil {
		_ = ledger.Close()
		t.Fatal("unjoined Node returned the original spend root")
	}
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original role did not join after releasing actual observation return")
	}
	result := original.Close()
	first, _ := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" {
		expected = "caller"
		if !errors.Is(result, context.Canceled) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original cancellation lost its cause or retained a counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" {
			expected = "process"
		}
		if plan.Fault == "member-expiry" {
			expected = "participation"
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original authority/resource retirement reported success", result)
		}
	}
	if first != expected {
		t.Fatal("original first-failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("late copied handle changed the original result")
	}
	journal, err = duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err = journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("original physical join did not release spend root", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("original join changed capacity", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("original State root failed genuine reopen", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("profile conflict vanished on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("original duty rebound to recovered successor")
	}
}

func nativeOriginalListenerOccupied(t *testing.T, member network.Member) bool {
	t.Helper()
	var closer interface{ Close() error }
	var err error
	if member.CarrierProfile == string(transport.ClosedCarrierTCP) {
		closer, err = net.Listen("tcp", member.Endpoint)
	} else if member.CarrierProfile == string(transport.ClosedCarrierQUIC) {
		closer, err = net.ListenPacket("udp", member.Endpoint)
	} else {
		t.Fatal("unselected native listener probe")
	}
	if err == nil {
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
		return false
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatal("original native endpoint observation unavailable", err)
	}
	return true
}

func checkNativeFailedStartupCleanup(t *testing.T, plan nativeOriginalAuthorityPlan, member network.Member, binding spending.Binding) {
	t.Helper()
	if nativeOriginalListenerOccupied(t, member) {
		t.Fatal("rejected original acquisition retained its actual listener")
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || conflict {
		t.Fatal("rejected original startup left a producer", conflict, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("late startup returned before its purpose root joined", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := budget.Observe(t.Context())
	if closeErr := budget.Close(); err != nil || closeErr != nil || snapshot.ReservedBytes != 0 {
		t.Fatal("late rejected acquisition retained capacity", snapshot, err, closeErr)
	}
}
