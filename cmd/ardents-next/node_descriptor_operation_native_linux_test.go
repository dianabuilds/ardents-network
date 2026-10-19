//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type nativeDescriptorOperationPlan struct {
	Original nativeDescriptorAuthorityPlan
	Signals  string
}

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeDescriptorOperationBoundariesTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_DESCRIPTOR_OPERATION_FIXTURE"); path != "" {
		nativeDescriptorOperationChild(t, path)
		return
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-commit", "post-commit-late-store-loss", "post-commit-concurrent"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeDescriptorOperationParent(t, carrier, phase) })
		}
	}
}

func nativeDescriptorOperationStack(storeOnly bool) bool {
	var pcs [64]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	for {
		frame, more := frames.Next()
		if storeOnly && strings.Contains(frame.Function, "/reachability.") && strings.HasSuffix(frame.Function, ".Publish") || !storeOnly && strings.Contains(frame.Function, "/route/receiver.") && strings.HasSuffix(frame.Function, ".serveRole") {
			return true
		}
		if !more {
			return false
		}
	}
}

func nativeDescriptorRecordCount(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) == 64 {
			count++
		}
	}
	return count
}

func nativeDescriptorOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeDescriptorOperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	p := plan.Original
	config, err := networkStateConfig(&p.Network)
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = p.Participation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member([32]byte{17}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var held atomic.Bool
	current := func() (network.RuntimeView, error) {
		actual, err := owner.CurrentRuntime()
		if _, armed := os.Stat(filepath.Join(plan.Signals, "arm")); armed == nil && nativeDescriptorOperationStack(strings.HasPrefix(p.Fault, "post-commit")) {
			journal, readErr := os.ReadFile(filepath.Join(p.Spend, "closed-token-spends"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			spent := len(journal) > 8+32+32+32+8
			committed := nativeDescriptorRecordCount(t, p.Descriptor) > 0
			point := spent && !committed
			if strings.HasPrefix(p.Fault, "pre-spend") {
				budget, openErr := hosting.Open(p.Hosting)
				if openErr != nil {
					t.Fatal(openErr)
				}
				observed, observeErr := budget.Observe(t.Context())
				closeErr := budget.Close()
				if observeErr != nil || closeErr != nil {
					t.Fatal(observeErr, closeErr)
				}
				point = !spent && !committed && observed.ReservedBytes >= 256<<10
			}
			if strings.HasPrefix(p.Fault, "post-commit") {
				point = spent && committed
			}
			if point && held.CompareAndSwap(false, true) {
				nativeIssuerMarker(t, plan.Signals, "entered")
				nativeIssuerWaitMarker(t, plan.Signals, "release", 10*time.Second)
			}
		}
		return actual, err
	}
	original, err := node.ListenDescriptor(ctx, node.DescriptorConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, DescriptorRoot: p.Descriptor, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("original Descriptor startup", err)
	}
	t.Cleanup(func() { nativeIssuerMarker(t, plan.Signals, "release"); cancel(); _ = original.Close() })
	nativeIssuerMarker(t, plan.Signals, "listening")
	nativeIssuerWaitMarker(t, plan.Signals, "stop", 20*time.Second)
	changedAuthority := p.Fault == "pre-spend-successor" || p.Fault == "pre-spend-process-loss"
	if p.Fault == "pre-spend-successor" {
		if _, err := owner.Accept(t.Context(), p.Successor.Epoch.Raw, p.Successor.Epoch.Inputs, p.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.AcceptClosedProfile(p.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		actual, err := owner.CurrentRuntime()
		if err != nil || actual.MatchDuty(retained, actual.ObservedAt()) == nil {
			t.Fatal("genuine successor did not retire original binding", err)
		}
	} else if !changedAuthority {
		if p.Fault != "pre-spend-stop" {
			cancel()
		}
		original.Stop()
	}
	binding := spending.Binding{NetworkID: view.Profile().Network, ProfileDigest: view.Profile().Digest, ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration}
	checkNativeDescriptorRetained(t, p, member, binding, filepath.Join(p.Descriptor, ".ardents-reachability-store-lock"))
	select {
	case <-original.Done():
		t.Fatal("held operation became joined")
	default:
	}
	if changedAuthority {
		// This marker proves only the changed input, never an accepting result
		// or a fake synchronous Node revoke. Real work must observe and deny it.
		nativeIssuerMarker(t, plan.Signals, "changed")
		nativeIssuerWaitMarker(t, plan.Signals, "release", 10*time.Second)
		select {
		case <-original.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("changed original authority did not deny/join actual work")
		}
	} else {
		nativeIssuerMarker(t, plan.Signals, "denied")
	}
	var result error
	if p.Fault == "post-commit-concurrent" {
		copy := *original
		joined := make(chan error, 2)
		go func() { joined <- original.Close() }()
		go func() { joined <- copy.Close() }()
		result = <-joined
		if second := <-joined; second != result {
			t.Fatal("concurrent copied Close changed original result", result, second)
		}
	} else {
		result = original.Close()
	}
	if p.Fault == "pre-spend-stop" {
		if result != nil || original.JoinedOutcome() != "canceled" {
			t.Fatal("ordinary Stop counterfeited physical failure", result)
		}
	} else if !changedAuthority && !errors.Is(result, context.Canceled) {
		t.Fatal("original caller failure lost", result)
	}
	first, cleanup := original.JoinedFailureOwners()
	if changedAuthority {
		expected := "network"
		if p.Fault == "pre-spend-process-loss" {
			expected = "process"
		}
		if first != expected || result == nil || original.JoinedOutcome() != "failed" {
			t.Fatal("changed input did not retain original owner failure", first, cleanup, result)
		}
	} else if p.Fault == "pre-spend-stop" {
		if first != "" || cleanup != "" {
			t.Fatal("ordinary Stop acquired foreign failure", first, cleanup)
		}
	} else if p.Fault == "post-commit-late-store-loss" {
		if first != "caller" || cleanup != "reachability" || original.JoinedOutcome() != "failed" {
			t.Fatal("late native Store failure vanished after caller denial", first, cleanup, original.JoinedOutcome(), result)
		}
	} else if first != "caller" || cleanup != "" && cleanup != "route" {
		t.Fatal("original failure ownership changed", first, cleanup, result)
	}
	copy := *original
	if copy.Close() != result || copy.Done() != original.Done() || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("copied Close changed original completion")
	}
	nativeIssuerMarker(t, plan.Signals, "joined")
}

func nativeDescriptorOperationParent(t *testing.T, carrier transport.CarrierProfile, phase string) {
	t.Helper()
	f, sockets, certificates := descriptorRouteFixture(t, carrier)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	holder := descriptorRouteStock(t, f)
	proof, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
	binary := nodeNativeCommand(t)
	var peers []*nativeForwardingProcess
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
		cert, key := nativeDescriptorCredential(t, certificates[id])
		config := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": "ardents-forwarding-process-v1", "retirement_wait_ms": 5000, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		sockets[id]()
		peers = append(peers, startNativeForwarding(t, binary, hostingConfig(t, config), id[0]))
	}
	cert, key := nativeDescriptorCredential(t, certificates[[32]byte{17}])
	p := nativeDescriptorAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Descriptor: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: phase}
	if phase == "pre-spend-successor" {
		next := f.spec
		next.Number++
		next.Previous = bundle.Epoch.Digest
		next.Seed[0]++
		p.Successor, err = networkfixture.BuildClosed(next)
		if err != nil {
			t.Fatal(err)
		}
	}
	plan := nativeDescriptorOperationPlan{Original: p, Signals: t.TempDir()}
	sockets[[32]byte{17}]()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	unit := fmt.Sprintf("ardents-descriptor-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_DESCRIPTOR_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeDescriptorOperationBoundariesTCP$", "-test.timeout=30s")
	var diagnostic nativeNodeDiagnostic
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	t.Cleanup(func() {
		nativeIssuerMarker(t, plan.Signals, "release")
		nativeIssuerMarker(t, plan.Signals, "stop")
		if !joined {
			_ = command.Wait()
		}
		if t.Failed() {
			t.Log(diagnostic.String())
		}
	})
	nativeIssuerWaitMarker(t, plan.Signals, "listening", 5*time.Second)
	prefixPlan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	prefix, err := startRoutePrefix(t.Context(), prefixPlan, f.authority, holder)
	if err != nil {
		t.Fatal("genuine operation Prefix", err)
	}
	defer prefix.close()
	journalPath := filepath.Join(p.Spend, "closed-token-spends")
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	nativeIssuerMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- prefix.publishDescriptor(caller, proof) }()
	nativeIssuerWaitMarker(t, plan.Signals, "entered", 3*time.Second)
	atHold, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(phase, "pre-spend") != bytes.Equal(before, atHold) {
		t.Fatal("incorrect real spend boundary", phase)
	}
	if count := nativeDescriptorRecordCount(t, p.Descriptor); strings.HasPrefix(phase, "post-commit") && count != 1 || !strings.HasPrefix(phase, "post-commit") && count != 0 {
		t.Fatal("incorrect real Store boundary", phase, count)
	}
	budget, err := hosting.Open(p.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	assertHeld := func() {
		t.Helper()
		observed, err := budget.Observe(t.Context())
		if err != nil || observed.ReservedBytes < 256<<10 {
			t.Fatal("full Control physical reservation returned before join", observed, err)
		}
	}
	assertHeld()
	if phase == "post-commit-concurrent" {
		// One genuine publication is still inside its post-commit observer.
		// A second real exchange on that original Source must read the floor while
		// that writer borrow remains live, before concurrent original Close.
		history := &reachability.History{}
		defer history.Close()
		got, err := prefix.lookupDescriptor(t.Context(), target, history)
		if err != nil || !bytes.Equal(got, proof) {
			t.Fatal("concurrent lookup lost actual committed floor", err)
		}
		atHold, err = os.ReadFile(journalPath)
		if err != nil {
			t.Fatal(err)
		}
		assertHeld()
	}
	changedAuthority := phase == "pre-spend-successor" || phase == "pre-spend-process-loss"
	if phase == "pre-spend-process-loss" {
		if err := exec.Command("systemctl", "set-property", "--runtime", unit, "CPUQuota=50%").Run(); err != nil {
			t.Fatal(err)
		}
	}
	nativeIssuerMarker(t, plan.Signals, "stop")
	marker := "denied"
	if changedAuthority {
		marker = "changed"
	}
	nativeIssuerWaitMarker(t, plan.Signals, marker, time.Second)
	assertHeld()
	var markerOriginal []byte
	markerPath := filepath.Join(p.Descriptor, ".ardents-reachability-store-v3")
	if phase == "post-commit-late-store-loss" {
		markerOriginal, err = os.ReadFile(markerPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.WriteFile(markerPath, markerOriginal, 0600) })
		// Original caller denial already happened, while the actual publication
		// borrow is still held. Its post-write native check must retain this loss.
		if err := os.WriteFile(markerPath, []byte("foreign marker\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	if err := <-completed; err == nil {
		t.Fatal("denied publication returned ACK")
	}
	assertHeld()
	if _, err := os.Stat(filepath.Join(plan.Signals, "joined")); err == nil {
		t.Fatal("unreleased original operation reported joined")
	}
	nativeIssuerMarker(t, plan.Signals, "release")
	nativeIssuerWaitMarker(t, plan.Signals, "joined", 3*time.Second)
	if err := command.Wait(); err != nil {
		t.Fatal("original operation child", err, diagnostic.String())
	}
	joined = true
	if markerOriginal != nil {
		if err := os.WriteFile(markerPath, markerOriginal, 0600); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(atHold, after) {
		t.Fatal("denial/refusal refunded or added spend", err)
	}
	observed, err := budget.Observe(t.Context())
	if err != nil || observed.ReservedBytes != 0 {
		t.Fatal("joined operation failed to return Hosting", observed, err)
	}
	fresh, err := reachability.OpenStore(reachability.StoreConfig{Root: p.Descriptor, Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal("independent Store reopen", err)
	}
	got, outcome, lookupErr := fresh.Lookup(target, f.profile.Digest, time.Now())
	if err := fresh.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(phase, "post-commit") {
		if lookupErr != nil || outcome != reachability.Accepted || !bytes.Equal(got, proof) {
			t.Fatal("post-commit denial lost exact floor", outcome, lookupErr)
		}
	} else if lookupErr != nil || outcome != reachability.Stale || len(got) != 0 {
		t.Fatal("pre-mutation denial wrote proof", outcome, lookupErr)
	}
	if err := prefix.close(); err != nil {
		t.Log("retained Source cleanup after original denial", err)
	}
	signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
	for _, peer := range peers {
		signals = append(signals, peer.unit)
	}
	if output, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	for _, peer := range peers {
		peer.signaled = true
		peer.stop(t, true)
	}
}
