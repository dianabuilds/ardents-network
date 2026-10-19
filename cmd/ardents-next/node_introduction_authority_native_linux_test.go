//go:build linux && node_process_native

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type nativeIntroductionAuthorityPlan struct {
	Network                                     networkAuthorityPlan
	Participation, Spend, Hosting, Introduction string
	Certificate, PrivateKey, Fault              string
	Successor                                   networkfixture.Closed
	Conflict                                    []byte
}

// A cancellation-state fixture delays only notification. It supplies no
// successful authority; original Err must deny before derived Done propagates.
type nativeIntroductionCaller struct {
	context.Context
	denied atomic.Bool
}

func (caller *nativeIntroductionCaller) Err() error {
	if caller.denied.Load() {
		return context.Canceled
	}
	return caller.Context.Err()
}

// Native composition holds only a completed genuine State return, never a fake
// accepting observation. The actual command has a separate positive workload.
func TestNativeIntroductionOriginalAuthorityAndJoinTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_INTRODUCTION_ORIGINAL_FIXTURE"); path != "" {
		nativeIntroductionAuthorityChild(t, path)
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			for index, fault := range []string{"stop", "caller", "direct-caller", "stale-measurement", "memory-pressure", "process-fd-loss", "successor", "profile-conflict", "member-expiry", "startup-cancel", "history-root-loss", "history-loss", "history-lock-loss", "history-lease-fd-loss"} {
				t.Run(fault, func(t *testing.T) {
					f, sockets, certificates := publicationRouteNetworkConfigured(t, carrier, func(f *networkAdmissionFixture) {
						if fault == "member-expiry" {
							for i := range f.spec.Nodes {
								if f.spec.Nodes[i].NodeID == [32]byte{25} {
									f.spec.Nodes[i].ValidUntil = time.Now().Add(5 * time.Second).UTC().Truncate(time.Second)
								}
							}
						}
					})
					bundle, err := networkfixture.BuildClosed(f.spec)
					if err != nil {
						t.Fatal(err)
					}
					clock, closeClock := joinProcessClock(t)
					defer closeClock()
					cert, key := nativeDescriptorCredential(t, certificates[[32]byte{25}])
					plan := nativeIntroductionAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Introduction: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: fault}
					if err := os.Chmod(plan.Introduction, 0700); err != nil {
						t.Fatal(err)
					}
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
						t.Fatal("signed conflict changed original Epoch", err)
					}
					plan.Conflict = changed.Profile
					path := hostingConfig(t, plan)
					sockets[[32]byte{25}]()
					unit := fmt.Sprintf("ardents-introduction-owner-%d-%d.service", os.Getpid(), index)
					t.Cleanup(func() { _ = exec.Command("systemctl", "stop", unit).Run() })
					command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
						"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
						"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_INTRODUCTION_ORIGINAL_FIXTURE="+path,
						"--", binary, "-test.v=true", "-test.run=^TestNativeIntroductionOriginalAuthorityAndJoinTCP$", "-test.timeout=30s")
					if output, err := command.CombinedOutput(); err != nil {
						t.Fatalf("genuine original Introduction composition failed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func nativeIntroductionAuthorityChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeIntroductionAuthorityPlan
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
	member, err := view.Member([32]byte{25}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	var armed, held, canceledAtListener atomic.Bool
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(resume) }) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	directCaller := &nativeIntroductionCaller{Context: ctx}
	var originalCaller context.Context = ctx
	if plan.Fault == "direct-caller" {
		originalCaller = directCaller
	}
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if plan.Fault == "startup-cancel" && !canceledAtListener.Load() && nativeOriginalListenerOccupied(t, member) && canceledAtListener.CompareAndSwap(false, true) {
			cancel()
		}
		if armed.Load() && held.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
		return actual, err
	}
	original, err := node.ListenIntroduction(originalCaller, node.IntroductionConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: plan.Participation, SpendRoot: plan.Spend, HostingRoot: plan.Hosting, IntroductionRoot: plan.Introduction, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey, Work: hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	if plan.Fault == "startup-cancel" {
		if original != nil {
			_ = original.Close()
			t.Fatal("late canceled startup handed over accepting Introduction")
		}
		if !canceledAtListener.Load() || !errors.Is(err, context.Canceled) {
			t.Fatal("late canceled startup lost original cause", err)
		}
		checkNativeFailedStartupCleanup(t, nativeOriginalAuthorityPlan{Network: plan.Network, Participation: plan.Participation, Spend: plan.Spend, Hosting: plan.Hosting, Certificate: plan.Certificate, PrivateKey: plan.PrivateKey}, member, binding)
		history, err := introduction.OpenHistory(plan.Introduction, introduction.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration})
		if err != nil {
			t.Fatal("rejected partial startup retained History lease", err)
		}
		if err := history.Close(); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatal("genuine Introduction startup refused", err)
	}
	t.Cleanup(func() { release(); original.Stop(); _ = original.Close() })
	armed.Store(true)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("original actual State observation did not enter held return")
	}
	originalLock := filepath.Join(plan.Introduction, ".ardents-introduction-lock")
	var restorePath, restoreFrom string
	var pressureMemory []byte
	switch plan.Fault {
	case "stop":
		stopped := make(chan struct{})
		go func() { original.Stop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("synchronous denial held Node state lock across foreign observation")
		}
	case "caller":
		cancel()
	case "direct-caller":
		directCaller.denied.Store(true)
	case "stale-measurement":
		time.Sleep(1200 * time.Millisecond)
	case "memory-pressure":
		pressureMemory = make([]byte, 513<<20)
		for index := 0; index < len(pressureMemory); index += os.Getpagesize() {
			pressureMemory[index] = 1
		}
	case "process-fd-loss":
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		for _, entry := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && target == filepath.Join("/proc", strconv.Itoa(os.Getpid())) {
				if fd >= 0 {
					t.Fatal("original process descriptor ambiguous")
				}
				fd, err = strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if fd < 0 {
			t.Fatal("actual original process descriptor absent")
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal(err)
		}
	case "successor":
		if _, err := owner.Accept(t.Context(), plan.Successor.Epoch.Raw, plan.Successor.Epoch.Inputs, plan.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(plan.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := owner.CurrentRuntime()
		if err != nil || changed.MatchDuty(retained, changed.ObservedAt()) == nil {
			t.Fatal("signed successor preserved original authority", err)
		}
	case "profile-conflict":
		if _, err := owner.AcceptClosedProfile(plan.Conflict); err == nil {
			t.Fatal("signed second profile accepted")
		}
		if _, err := owner.CurrentRuntime(); err == nil {
			t.Fatal("signed conflict supplied current authority")
		}
	case "member-expiry":
		time.Sleep(time.Until(member.NotAfter()) + 150*time.Millisecond)
	case "history-lease-fd-loss":
		checkNativeIntroductionRetained(t, plan, member, binding, originalLock)
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		fd := -1
		for _, entry := range entries {
			path, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err == nil && path == originalLock {
				if fd >= 0 {
					t.Fatal("original History lease descriptor ambiguous")
				}
				fd, err = strconv.Atoi(entry.Name())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if fd < 0 {
			t.Fatal("original History lease descriptor unavailable")
		}
		if err := syscall.Close(fd); err != nil {
			t.Fatal("actual original descriptor loss failed", err)
		}
		// The fixture physically stole that descriptor. Do not pretend the
		// kernel still holds its flock or classify this as an owner release.
		originalLock = ""
	case "history-root-loss", "history-loss", "history-lock-loss":
		object := plan.Introduction
		if plan.Fault == "history-loss" {
			object = filepath.Join(object, "closed-introduction-slots")
		}
		if plan.Fault == "history-lock-loss" {
			object = filepath.Join(object, ".ardents-introduction-lock")
		}
		moved := object + ".original"
		if err := os.Rename(object, moved); err != nil {
			t.Fatal("actual native custody loss unavailable", err)
		}
		restorePath, restoreFrom = object, moved
		if plan.Fault == "history-root-loss" {
			originalLock = filepath.Join(moved, ".ardents-introduction-lock")
		}
		if plan.Fault == "history-lock-loss" {
			originalLock = moved
		}
	default:
		t.Fatal("unselected Introduction authority control", plan.Fault)
	}
	select {
	case <-original.Done():
		t.Fatal("original Done reported before held user physically joined")
	case <-time.After(30 * time.Millisecond):
	}
	checkNativeIntroductionRetained(t, plan, member, binding, originalLock)
	release()
	select {
	case <-original.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("original Introduction did not join released actual observer")
	}
	result := original.Close()
	runtime.KeepAlive(pressureMemory)
	pressureMemory = nil
	runtime.GC()
	if restorePath != "" {
		if err := os.Rename(restoreFrom, restorePath); err != nil {
			t.Fatal(err)
		}
	}
	first, cleanup := original.JoinedFailureOwners()
	expected := "network"
	if plan.Fault == "stop" || plan.Fault == "caller" || plan.Fault == "direct-caller" {
		expected = "caller"
		if plan.Fault == "stop" {
			expected = ""
		}
		if ((plan.Fault == "caller" || plan.Fault == "direct-caller") && !errors.Is(result, context.Canceled)) || (plan.Fault == "stop" && result != nil) || original.JoinedOutcome() != "canceled" {
			t.Fatal("original retirement lost cause or counterfeit cleanup failure", result, original.JoinedOutcome())
		}
	} else {
		if plan.Fault == "stale-measurement" || plan.Fault == "memory-pressure" || plan.Fault == "process-fd-loss" {
			expected = "process"
		}
		if plan.Fault == "member-expiry" {
			expected = "participation"
		}
		if plan.Fault == "history-root-loss" || plan.Fault == "history-loss" || plan.Fault == "history-lock-loss" || plan.Fault == "history-lease-fd-loss" {
			expected = "introduction"
		}
		if plan.Fault == "history-lease-fd-loss" && (cleanup != "introduction" || !errors.Is(result, syscall.EBADF)) {
			t.Fatal("actual failed physical lease release lost cause/owner", cleanup, result)
		}
		if result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("original owner loss reported successful withdrawal", result)
		}
	}
	if plan.Fault == "process-fd-loss" && (cleanup != "process" || !errors.Is(result, syscall.EBADF)) {
		t.Fatal("actual process physical release failure lost", cleanup, result)
	}
	if first != expected {
		t.Fatal("original failure owner changed", first, expected, result)
	}
	copy := *original
	if copy.Done() != original.Done() || copy.Close() != result || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("copied handle replaced original result")
	}
	history, err := introduction.OpenHistory(plan.Introduction, introduction.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration})
	if err != nil {
		t.Fatal("History not released after actual role join", err)
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || collision {
		t.Fatal("joined producer remains live", collision, err, closeErr)
	}
	ledger, err := spending.Open(plan.Spend, binding)
	if err != nil {
		t.Fatal("joined Introduction retained receiving root", err)
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
		t.Fatal("original join did not return Hosting", snapshot, err, closeErr)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal("State native reopen failed", err)
	}
	defer reopened.Close()
	recovered, err := reopened.CurrentRuntime()
	if plan.Fault == "profile-conflict" {
		if err == nil {
			t.Fatal("conflict disappeared on reopen")
		}
	} else if err != nil {
		t.Fatal(err)
	} else if plan.Fault == "successor" && recovered.MatchDuty(retained, recovered.ObservedAt()) == nil {
		t.Fatal("reopen rebound old duty")
	}
}

func checkNativeIntroductionRetained(t *testing.T, plan nativeIntroductionAuthorityPlan, member network.Member, binding spending.Binding, originalLock string) {
	t.Helper()
	journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: plan.Participation, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	collision, err := journal.Conflict(member.NodeID, member.FamilyID)
	if closeErr := journal.Close(); err != nil || closeErr != nil || !collision {
		t.Fatal("unjoined original producer disappeared, including beyond work expiry", collision, err, closeErr)
	}
	if ledger, err := spending.Open(plan.Spend, binding); err == nil {
		_ = ledger.Close()
		t.Fatal("original receiving root returned before physical join")
	}
	if originalLock == "" {
		return
	}
	lock, err := os.OpenFile(originalLock, os.O_RDWR, 0)
	if err != nil {
		t.Fatal("actual original History lock unavailable", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("original History lease returned before physical join", err)
	}
}
