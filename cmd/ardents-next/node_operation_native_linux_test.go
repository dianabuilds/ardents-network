//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
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
		if _, err := owner.AcceptEpoch(t.Context(), p.Successor.Epoch.Raw, p.Successor.Epoch.Inputs, p.Successor.Epoch.Materials); err != nil {
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

type nativeIntroductionOperationPlan struct {
	Original nativeIntroductionAuthorityPlan
	Signals  string
}

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeIntroductionOperationBoundariesTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_INTRODUCTION_OPERATION_FIXTURE"); path != "" {
		nativeIntroductionOperationChild(t, path)
		return
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-commit", "post-commit-concurrent", "post-commit-late-history-loss"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeIntroductionOperationParent(t, carrier, phase) })
		}
	}
}

func nativeIntroductionOperationStack(registrationOnly bool) bool {
	var pcs [64]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	for {
		frame, more := frames.Next()
		if registrationOnly && strings.Contains(frame.Function, "/route/introduction.") && strings.HasSuffix(frame.Function, ".ServeRegistration") || !registrationOnly && strings.Contains(frame.Function, "/route/receiver.") && strings.HasSuffix(frame.Function, ".serveRole") {
			return true
		}
		if !more {
			return false
		}
	}
}

func nativeIntroductionRecordCount(t *testing.T, root string) int {
	t.Helper()
	snapshot, err := os.ReadFile(filepath.Join(root, "closed-introduction-slots"))
	if err != nil || len(snapshot) < 120 || string(snapshot[:8]) != "ARDISL01" || (len(snapshot)-120)%40 != 0 {
		t.Fatal("native History snapshot unavailable", err)
	}
	return (len(snapshot) - 120) / 40
}

func nativeIntroductionOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeIntroductionOperationPlan
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
	member, err := view.Member([32]byte{25}, view.ObservedAt())
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
		if _, armed := os.Stat(filepath.Join(plan.Signals, "arm")); armed == nil && nativeIntroductionOperationStack(strings.HasPrefix(p.Fault, "post-commit")) {
			journal, readErr := os.ReadFile(filepath.Join(p.Spend, "closed-token-spends"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			spent := len(journal) > 8+32+32+32+8
			committed := nativeIntroductionRecordCount(t, p.Introduction) > 0
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
				point = !spent && !committed && observed.ReservedBytes >= (2<<20)+(128<<10)
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
	original, err := node.ListenIntroduction(ctx, node.IntroductionConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, IntroductionRoot: p.Introduction, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Work: hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("original Introduction startup", err)
	}
	t.Cleanup(func() { nativeIssuerMarker(t, plan.Signals, "release"); cancel(); _ = original.Close() })
	nativeIssuerMarker(t, plan.Signals, "listening")
	nativeIssuerWaitMarker(t, plan.Signals, "stop", 20*time.Second)
	changedAuthority := p.Fault == "pre-spend-successor" || p.Fault == "pre-spend-process-loss"
	if p.Fault == "pre-spend-successor" {
		if _, err := owner.AcceptEpoch(t.Context(), p.Successor.Epoch.Raw, p.Successor.Epoch.Inputs, p.Successor.Epoch.Materials); err != nil {
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
	checkNativeIntroductionRetained(t, p, member, binding, filepath.Join(p.Introduction, ".ardents-introduction-lock"))
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
	} else if p.Fault == "post-commit-late-history-loss" {
		if first != "caller" || cleanup != "introduction" || original.JoinedOutcome() != "failed" {
			t.Fatal("late native History failure vanished after caller denial", first, cleanup, original.JoinedOutcome(), result)
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

func nativeIntroductionOperationParent(t *testing.T, carrier transport.CarrierProfile, phase string) {
	t.Helper()
	f, sockets, certificates := publicationRouteNetwork(t, carrier)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	holder := nativeIntroductionStock(t, f)
	binary := nodeNativeCommand(t)
	var peers []*nativeForwardingProcess
	for _, id := range [][32]byte{{21}, {22}, {23}, {24}} {
		cert, key := nativeDescriptorCredential(t, certificates[id])
		config := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "process_profile": "ardents-forwarding-process-v1", "retirement_wait_ms": 5000, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		sockets[id]()
		peers = append(peers, startNativeForwarding(t, binary, hostingConfig(t, config), id[0]))
	}
	cert, key := nativeDescriptorCredential(t, certificates[[32]byte{25}])
	p := nativeIntroductionAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Introduction: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: phase}
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
	if err := os.Chmod(p.Introduction, 0700); err != nil {
		t.Fatal(err)
	}
	plan := nativeIntroductionOperationPlan{Original: p, Signals: t.TempDir()}
	sockets[[32]byte{25}]()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	unit := fmt.Sprintf("ardents-introduction-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_INTRODUCTION_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeIntroductionOperationBoundariesTCP$", "-test.timeout=30s")
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
	prefixPlan := routePrefixPlan{Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	selectionRoot := t.TempDir()
	selected, err := selection.Open(selection.Config{EntryRoot: filepath.Join(selectionRoot, "entry"), InteriorRoot: filepath.Join(selectionRoot, "interior"), Domain: 4, Current: f.authority.current})
	if err != nil {
		t.Fatal(err)
	}
	defer selected.Close()
	prefixBudget, err := hosting.Open(routeProcessBudget(t))
	if err != nil {
		t.Fatal(err)
	}
	defer prefixBudget.Close()
	prefix, leg, err := openPublicationPrefix(t.Context(), prefixPlan, selected, prefixBudget, f.authority, holder)
	if err != nil {
		t.Fatal("genuine Registration prefix", err)
	}
	defer prefix.Close()
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	duty, err := leg.IntroductionDuty(view, nil)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(p.Spend, "closed-token-spends")
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	nativeIssuerMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		registration, err := introduction.Register(caller, prefix, introduction.RegistrationConfig{Duty: duty, Revision: 1, Deadline: prefixPlan.Deadline})
		if registration != nil {
			err = errors.Join(err, registration.Close())
		}
		completed <- err
	}()
	nativeIssuerWaitMarker(t, plan.Signals, "entered", 3*time.Second)
	atHold, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(phase, "pre-spend") != bytes.Equal(before, atHold) {
		t.Fatal("incorrect real spend boundary", phase)
	}
	if count := nativeIntroductionRecordCount(t, p.Introduction); strings.HasPrefix(phase, "post-commit") && count != 1 || !strings.HasPrefix(phase, "post-commit") && count != 0 {
		t.Fatal("incorrect real History boundary", phase, count)
	}
	budget, err := hosting.Open(p.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	assertHeld := func() {
		t.Helper()
		observed, err := budget.Observe(t.Context())
		if err != nil || observed.ReservedBytes < (2<<20)+(128<<10) {
			t.Fatal("full Registration physical reservation returned before join", observed, err)
		}
	}
	assertHeld()
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
	markerPath := filepath.Join(p.Introduction, "closed-introduction-slots")
	if phase == "post-commit-late-history-loss" {
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
	retained, err := introduction.OpenHistory(p.Introduction, introduction.Binding{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, ReceiverNodeID: [32]byte{25}, ReceiverDutyGeneration: 9})
	if err != nil {
		t.Fatal("independent History reopen", err)
	}
	if err := retained.Close(); err != nil {
		t.Fatal(err)
	}
	count := nativeIntroductionRecordCount(t, p.Introduction)
	if strings.HasPrefix(phase, "post-commit") && count != 1 || !strings.HasPrefix(phase, "post-commit") && count != 0 {
		t.Fatal("denial changed durable non-reclaim floor", phase, count)
	}
	if err := prefix.Close(); err != nil {
		t.Log("retained holder prefix cleanup after original denial", err)
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

type nativeIssuerOperationPlan struct {
	Original    nativeIssuerAuthorityPlan
	Signals     string
	ResultBytes int64
	QuotaBytes  int64
}

// Only schedules a real State return after actual result-journal growth. The
// child opens the genuine original Node; the parent owns independent genuine
// Stock and physical Prefixes. No fixture signs a successful runtime response.
func TestNativeIssuerOperationJoinAndExactRetryTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_ISSUER_OPERATION_FIXTURE"); path != "" {
		nativeIssuerOperationChild(t, path)
		return
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, mode := range []string{"retry", "pre-debit-caller", "post-debit-caller", "post-result-caller", "post-result-successor"} {
			t.Run(string(carrier)+"/"+mode, func(t *testing.T) { nativeIssuerOperationParent(t, carrier, mode) })
		}
	}
}

func TestNativeIssuerStorageUncertaintyTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, mode := range []string{"quota-uncertain", "result-uncertain"} {
			t.Run(string(carrier)+"/"+mode, func(t *testing.T) { nativeIssuerOperationParent(t, carrier, mode) })
		}
	}
}

func nativeIssuerStorageMode(mode string) bool {
	return mode == "quota-uncertain" || mode == "result-uncertain"
}

func nativeIssuerOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeIssuerOperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	p := plan.Original
	config, err := networkStateConfig(&p.Network)
	if err != nil {
		t.Fatal(err)
	}
	config.LocalRoleStateRoot = p.Participation
	stateOwner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stateOwner.Close(); err != nil {
			t.Error("original State cleanup", err)
		}
	})
	view, err := stateOwner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(p.Issuance.KeyBinding.Issuer, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var held atomic.Bool
	var fileLimit syscall.Rlimit
	if nativeIssuerStorageMode(p.Fault) {
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &fileLimit); err != nil {
			t.Fatal(err)
		}
		signal.Ignore(syscall.SIGXFSZ)
		defer signal.Reset(syscall.SIGXFSZ)
		defer func() {
			if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &fileLimit); err != nil {
				t.Error(err)
			}
		}()
	}
	current := func() (network.RuntimeView, error) {
		actual, err := stateOwner.CurrentRuntime()
		if _, armed := os.Stat(filepath.Join(plan.Signals, "arm")); armed == nil {
			info, statErr := os.Stat(filepath.Join(p.Issuance.ResultRoot, "results.journal"))
			quotaInfo, quotaErr := os.Stat(filepath.Join(p.Issuance.AdmissionRoot, "admission.journal"))
			point := statErr == nil && info.Size() > plan.ResultBytes
			if p.Fault == "pre-debit-caller" || p.Fault == "quota-uncertain" {
				point = quotaErr == nil && quotaInfo.Size() == plan.QuotaBytes && statErr == nil && info.Size() == plan.ResultBytes
			}
			if p.Fault == "post-debit-caller" || p.Fault == "result-uncertain" {
				point = quotaErr == nil && quotaInfo.Size() > plan.QuotaBytes && statErr == nil && info.Size() == plan.ResultBytes
			}
			if point && nativeIssuerIssuingStack() && held.CompareAndSwap(false, true) {
				nativeIssuerMarker(t, plan.Signals, "entered")
				nativeIssuerWaitMarker(t, plan.Signals, "release", 10*time.Second)
				if nativeIssuerStorageMode(p.Fault) {
					limit := fileLimit
					limit.Cur = uint64(plan.QuotaBytes + 1)
					if p.Fault == "result-uncertain" {
						limit.Cur = uint64(plan.ResultBytes + 1)
					}
					if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limit); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		return actual, err
	}
	original, err := node.ListenIssuer(ctx, node.IssuerConfig{Authority: role.Authority{Current: current, Duty: duty, Profile: view.Profile().ProfileBinding},
		ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Issuance: p.Issuance,
		Work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, BootstrapWork: hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("genuine original issuing child unavailable", err)
	}
	t.Cleanup(func() { nativeIssuerMarker(t, plan.Signals, "release"); cancel(); _ = original.Close() })
	nativeIssuerMarker(t, plan.Signals, "listening")
	if nativeIssuerStorageMode(p.Fault) {
		<-original.Done()
		closed := original.Close()
		if !errors.Is(closed, quota.ErrUncertain) && !errors.Is(closed, issuance.ErrUncertain) {
			t.Fatal("actual partial write did not retain uncertainty", closed)
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &fileLimit); err != nil {
			t.Fatal(err)
		}
		copy := *original
		if copy.Close() != closed || original.Close() != closed || original.JoinedOutcome() != "failed" {
			t.Fatal("restored write conditions revived original life")
		}
		first, _ := original.JoinedFailureOwners()
		if first != "issuance" {
			t.Fatal("original uncertainty owner changed", first, closed)
		}
		authority := networkAdmissionAuthority(stateOwner.CurrentRuntime, stateOwner.Close, stateOwner.CurrentRuntimeWitness)
		reopened, opened := issuer.OpenCurrent(t.Context(), p.Issuance, func() (admission.AuthorityFacts, time.Time, error) {
			return authority.issuer(p.Issuance.KeyBinding.Signer)
		})
		if opened.Outcome == "opened" {
			_ = reopened.Close(context.Background())
			t.Fatal("independent reopen repaired partial journal")
		}
		nativeIssuerMarker(t, plan.Signals, "joined")
		return
	}
	nativeIssuerWaitMarker(t, plan.Signals, "stop", 20*time.Second)
	if p.Fault == "post-result-successor" {
		if _, err := stateOwner.AcceptEpoch(t.Context(), p.Successor.Epoch.Raw, p.Successor.Epoch.Inputs, p.Successor.Epoch.Materials); err != nil {
			t.Fatal(err)
		}
		if _, err := stateOwner.AcceptClosedProfile(p.Successor.Profile); err != nil {
			t.Fatal(err)
		}
		changed, err := stateOwner.CurrentRuntime()
		if err != nil || changed.MatchDuty(duty, changed.ObservedAt()) == nil {
			t.Fatal("signed successor kept original operation authority", err)
		}
		nativeIssuerMarker(t, plan.Signals, "denied")
		<-original.Done()
	} else {
		cancel()
		original.Stop()
		nativeIssuerMarker(t, plan.Signals, "denied")
	}
	closed := original.Close()
	if p.Fault != "post-result-successor" && !errors.Is(closed, context.Canceled) || p.Fault == "post-result-successor" && closed == nil {
		nativeIssuerOperationPressureEvidence(t)
		t.Fatal("original issuing caller cause lost", closed)
	}
	if closed != context.Canceled {
		t.Log("retained original operation physical result", closed)
	}
	first, cleanup := original.JoinedFailureOwners()
	expected := "caller"
	if p.Fault == "post-result-successor" {
		expected = "network"
	}
	if first != expected || cleanup != "" && cleanup != "route" {
		t.Fatal("unexpected original issuing failure owners", first, cleanup)
	}
	checkNativeIssuerRootsReleased(t, p.Issuance)
	nativeIssuerMarker(t, plan.Signals, "joined")
}

func nativeIssuerMarker(t *testing.T, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
}
func nativeIssuerWaitMarker(t *testing.T, root, name string, limit time.Duration) {
	t.Helper()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("native operation marker unavailable", name)
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
}

// These genuine offline tokens isolate the operation fault. Actual command
// bootstrap is separately mandatory in the same native profile.
func nativeIssuerOperationStock(t *testing.T, f *networkAdmissionFixture) *stock.Owner {
	t.Helper()
	holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{4, 8, 0})
	for index, class := range []uint8{2, 1} {
		var receivers [][32]byte
		if class == 2 {
			receivers = [][32]byte{{12}, {13}, {14}, {15}}
		} else {
			receivers = [][32]byte{f.profile.IssuerNodeID, f.profile.IssuerNodeID, f.profile.IssuerNodeID}
		}
		var challenges []token.ClosedTokenContext
		for _, id := range receivers {
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			m, err := view.Member(id, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			challenges = append(challenges, token.ClosedTokenContext{NetworkID: f.profile.NetworkID, ProfileDigest: f.profile.Digest, IssuerNodeID: f.profile.IssuerNodeID, ReceiverNodeID: id, ReceiverDutyGeneration: m.DutyGeneration, Class: class, WindowStart: time.Now().UTC().Truncate(time.Hour)})
		}
		attempt, err := holder.Begin(stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(170 + index)}, ProfileDigest: f.profile.Digest}, Bootstrap: true, Deadline: f.profile.NotAfter})
		if err != nil {
			t.Fatal(err)
		}
		batch, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, batch, quota.Bootstrap, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		clear(batch)
		if issued.Outcome != "issued-offline" {
			t.Fatal("genuine preparation issuance failed", issued.Phase, issued.Outcome)
		}
		if err := attempt.Complete(issued.Response, nil); err != nil {
			t.Fatal(err)
		}
		clear(issued.Response)
	}
	return holder
}

func nativeIssuerOperationParent(t *testing.T, carrier transport.CarrierProfile, mode string) {
	t.Helper()
	f, sockets, certificates := newRoleRouteFixture(t, carrier, 1, false, true)
	var holders [2]*stock.Owner
	for i := range holders {
		holders[i] = nativeIssuerOperationStock(t, f)
	}
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	var peers []*joinCommandProcess
	binary := nodeNativeCommand(t)
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
		cert, key := nativeIssuerOperationCredential(t, certificates[id])
		plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": id, "spend_root": t.TempDir(), "hosting_root": nativeIssuerOperationBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		sockets[id]()
		p := startJoinCommandProcess(t, t.Context(), binary, "route", "receive", "--config", hostingConfig(t, plan))
		t.Cleanup(func() {
			if !p.waited {
				_ = p.command.Process.Kill()
				_ = p.wait()
			}
		})
		var ready map[string]string
		if err := p.decode.Decode(&ready); err != nil || ready["phase"] != "listening" {
			t.Fatal("actual forwarding peer unavailable", err, p.diagnostic.String())
		}
		peers = append(peers, p)
	}
	certificate, key := nativeIssuerOperationCredential(t, certificates[f.profile.IssuerNodeID])
	resultInfo, err := os.Stat(filepath.Join(f.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	plan := nativeIssuerOperationPlan{Original: nativeIssuerAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Certificate: certificate, PrivateKey: key, Issuance: f.plan}, Signals: t.TempDir(), ResultBytes: resultInfo.Size()}
	initialQuota, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
	if err != nil {
		t.Fatal(err)
	}
	initialResult, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	plan.QuotaBytes = int64(len(initialQuota))
	plan.Original.Fault = mode
	if mode == "post-result-successor" {
		next := f.spec
		next.Number++
		next.Previous = bundle.Epoch.Digest
		next.Seed[0]++
		plan.Original.Successor, err = networkfixture.BuildClosed(next)
		if err != nil {
			t.Fatal(err)
		}
	}
	sockets[f.profile.IssuerNodeID]()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	unit := fmt.Sprintf("ardents-issuer-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=60s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_ISSUER_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeIssuerOperationJoinAndExactRetryTCP$", "-test.timeout=30s")
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
			t.Log("native issuing child result", diagnostic.String())
		}
	})
	nativeIssuerWaitMarker(t, plan.Signals, "listening", 5*time.Second)
	var prefixes [2]routeHandle
	for i := range prefixes {
		p := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		prefixes[i], err = startRoutePrefix(t.Context(), p, f.authority, holders[i])
		if err != nil {
			t.Fatal("genuine operation Prefix unavailable", i, err)
		}
		defer prefixes[i].close()
	}
	nativeIssuerMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- prefixes[0].issue(caller, 2, nil) }()
	nativeIssuerWaitMarker(t, plan.Signals, "entered", 3*time.Second)
	checkNativeIssuerRootsHeld(t, f.plan)
	before, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
	if err != nil {
		t.Fatal(err)
	}
	committedAtHold, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	if (mode == "pre-debit-caller" || mode == "quota-uncertain") && (!bytes.Equal(initialQuota, before) || !bytes.Equal(initialResult, committedAtHold)) {
		t.Fatal("pre-debit hold already changed issuing journals")
	}
	if (mode == "post-debit-caller" || mode == "result-uncertain") && (bytes.Equal(initialQuota, before) || !bytes.Equal(initialResult, committedAtHold)) {
		t.Fatal("post-debit hold lacks exact commit boundary")
	}
	if (mode == "post-result-caller" || mode == "post-result-successor") && (bytes.Equal(initialQuota, before) || bytes.Equal(initialResult, committedAtHold)) {
		t.Fatal("post-result hold lacks genuine committed work")
	}
	spendBefore, err := os.ReadFile(filepath.Join(plan.Original.Spend, "closed-token-spends"))
	if err != nil {
		t.Fatal(err)
	}
	if err := prefixes[1].issue(t.Context(), 2, nil); err == nil {
		t.Fatal("concurrent private issuing operation accepted")
	}
	after, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("busy operation changed shared quota", err)
	}
	spendAfter, err := os.ReadFile(filepath.Join(plan.Original.Spend, "closed-token-spends"))
	if err != nil || !bytes.Equal(spendBefore, spendAfter) {
		t.Fatal("busy operation burned a second receiving token", err)
	}
	budget, err := hosting.Open(plan.Original.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	observed, err := budget.Observe(t.Context())
	if err != nil || observed.ReservedBytes == 0 {
		t.Fatal("held signing result returned physical Hosting", err, observed.ReservedBytes)
	}
	if mode != "retry" && !nativeIssuerStorageMode(mode) {
		nativeIssuerMarker(t, plan.Signals, "stop")
		nativeIssuerWaitMarker(t, plan.Signals, "denied", time.Second)
		checkNativeIssuerRootsHeld(t, f.plan)
		observed, err = budget.Observe(t.Context())
		if err != nil || observed.ReservedBytes == 0 {
			t.Fatal("original denial returned unjoined work", err, observed.ReservedBytes)
		}
	}
	if nativeIssuerStorageMode(mode) {
		nativeIssuerMarker(t, plan.Signals, "release")
	} else {
		cancel()
	}
	if err := <-completed; err == nil || mode == "retry" && !errors.Is(err, context.Canceled) {
		// A remote original caller's cancellation may reach this independent
		// client as physical EOF. The child separately requires that exact
		// original context cause/owner; a successful delivery is always forbidden.
		t.Fatal("denied delivery became successful or lost local cancellation", err)
	}
	if !nativeIssuerStorageMode(mode) {
		checkNativeIssuerRootsHeld(t, f.plan)
	}
	nativeIssuerMarker(t, plan.Signals, "release")
	// Wait for actual work-owned reservation return, never refund or retry in
	// an automatic loop. This observes physical cleanup, not a success flag.
	if mode != "retry" {
		nativeIssuerWaitMarker(t, plan.Signals, "joined", 3*time.Second)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		observed, err = budget.Observe(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if observed.ReservedBytes == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("original operation did not return its reservation", observed.ReservedBytes, diagnostic.String())
		}
		time.Sleep(time.Millisecond)
	}
	committed, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	if mode == "retry" {
		if err := prefixes[0].issue(t.Context(), 2, nil); err != nil {
			t.Fatal("explicit original exact retry", err)
		}
		quotaAfter, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
		if err != nil || !bytes.Equal(before, quotaAfter) {
			t.Fatal("exact retry debited again", err)
		}
		resultAfter, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
		if err != nil || !bytes.Equal(committed, resultAfter) {
			t.Fatal("exact retry changed committed result", err)
		}
		if s := holders[0].Status(); s.Pending || s.Busy || s.Remaining[1] != 2 {
			t.Fatal("genuine retry lost stock/no-refund", s)
		}
	} else if nativeIssuerStorageMode(mode) {
		if s := holders[0].Status(); !s.Pending || s.Busy || s.Remaining[1] != 2 {
			t.Fatal("uncertain output deposited/refunded stock", s)
		}
		q, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
		if err != nil {
			t.Fatal(err)
		}
		r, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
		if err != nil {
			t.Fatal(err)
		}
		if mode == "quota-uncertain" && (len(q) != len(before)+1 || !bytes.HasPrefix(q, before) || !bytes.Equal(r, committedAtHold)) {
			t.Fatal("actual quota partial-write footprint differs")
		}
		if mode == "result-uncertain" && (!bytes.Equal(q, before) || len(r) != len(committedAtHold)+1 || !bytes.HasPrefix(r, committedAtHold)) {
			t.Fatal("actual result partial-write footprint differs")
		}
	} else {
		if s := holders[0].Status(); !s.Pending || s.Busy || s.Remaining[1] != 2 {
			t.Fatal("original denied delivery deposited or refunded stock", s)
		}
		quotaAfter, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
		if err != nil || !bytes.Equal(before, quotaAfter) {
			t.Fatal("original denial changed retained quota", err)
		}
		resultAfter, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
		if err != nil || !bytes.Equal(committedAtHold, resultAfter) {
			t.Fatal("original denial changed retained result", err)
		}
	}
	for _, p := range prefixes {
		_ = p.close()
	}
	nativeIssuerMarker(t, plan.Signals, "stop")
	if err := command.Wait(); err != nil {
		joined = true
		t.Fatal("original native child did not join", err, diagnostic.String())
	}
	joined = true
	if mode == "retry" {
		request, response := nativeIssuerCommittedPair(t, initialResult, committed)
		defer clear(request)
		defer clear(response)
		beforeQuota, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
		if err != nil {
			t.Fatal(err)
		}
		reopened, opened := issuer.OpenCurrent(t.Context(), f.plan, func() (admission.AuthorityFacts, time.Time, error) {
			return f.authority.issuer(f.plan.KeyBinding.Signer)
		})
		if opened.Outcome != "opened" {
			t.Fatal("independent original issuing reopen refused", opened.Phase, opened.Outcome)
		}
		replayed := reopened.IssueCurrent(t.Context(), request, quota.Admitted)
		closed := reopened.Close(context.Background())
		if replayed.Outcome != "already-issued" || !bytes.Equal(response, replayed.Response) || reopened.Err() != nil || closed.Outcome != "closed" {
			t.Fatal("independent exact replay lost result", replayed.Phase, replayed.Outcome, closed.Outcome, reopened.Err())
		}
		clear(replayed.Response)
		afterQuota, err := os.ReadFile(filepath.Join(f.plan.AdmissionRoot, "admission.journal"))
		if err != nil || !bytes.Equal(beforeQuota, afterQuota) {
			t.Fatal("independent reopen debited again", err)
		}
		afterResult, err := os.ReadFile(filepath.Join(f.plan.ResultRoot, "results.journal"))
		if err != nil || !bytes.Equal(committed, afterResult) {
			t.Fatal("independent reopen rewrote retained result", err)
		}
	}
	for _, p := range peers {
		p.interrupt(t)
	}
	for _, p := range peers {
		err := p.wait()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 130 {
			t.Fatal("actual operation peer did not join", err, p.diagnostic.String())
		}
	}
}

func nativeIssuerOperationCredential(t *testing.T, c tls.Certificate) (string, string) {
	t.Helper()
	certificate, key := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := x509.MarshalPKCS8PrivateKey(c.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw})
	if err := os.WriteFile(key, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	clear(raw)
	clear(encoded)
	return certificate, key
}

func nativeIssuerOperationBudget(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "budget")
	start := time.Now().UTC().Truncate(time.Hour)
	// Two complete simultaneous Forward allowances and their independent
	// termination reserves must fit the declared provider period before spend.
	if err := hosting.Initialize(root, hosting.Policy{Provider: "native two-holder operation", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: 256, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	return root
}

// Independent persisted-byte oracle: one newly appended request/response pair,
// exact length and checksum-chain framing, with no production decoder reuse.
// This extracts evidence only; the reopened owners still validate everything.
func nativeIssuerCommittedPair(t *testing.T, before, after []byte) ([]byte, []byte) {
	t.Helper()
	if len(before) < 32 || !bytes.HasPrefix(after, before) {
		t.Fatal("result history prefix changed")
	}
	var tail [32]byte
	copy(tail[:], before[len(before)-32:])
	rest := after[len(before):]
	read := func() []byte {
		if len(rest) < 36 {
			t.Fatal("result pair frame absent")
		}
		n := int(binary.BigEndian.Uint32(rest[:4]))
		if n < 1 || n > 16384 || len(rest) < 4+n+32 {
			t.Fatal("result pair frame bound")
		}
		body := rest[4 : 4+n]
		h := sha256.New()
		_, _ = h.Write(tail[:])
		_, _ = h.Write(body)
		if !bytes.Equal(h.Sum(nil), rest[4+n:4+n+32]) {
			t.Fatal("result pair chain checksum")
		}
		copy(tail[:], rest[4+n:4+n+32])
		rest = rest[4+n+32:]
		return append([]byte(nil), body...)
	}
	request, response := read(), read()
	if len(request) < 9 || request[0] != byte(quota.Admitted) || len(rest) != 0 || len(response) != 16347 {
		t.Fatal("result pair kind/canonical size/count")
	}
	raw := append([]byte(nil), request[9:]...)
	clear(request)
	if _, err := admission.DecodeClosedTokenBatch(raw); err != nil {
		t.Fatal("persisted canonical request invalid", err)
	}
	return raw, response
}

// Identify the actual issuing caller rather than an unrelated background
// observation. This schedules a test fault; it grants no authority or success.
func nativeIssuerIssuingStack() bool {
	var pcs [32]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, "/admission/issuer.Owner.IssueCurrent") {
			return true
		}
		if !more {
			return false
		}
	}
}

func nativeIssuerOperationPressureEvidence(t *testing.T) {
	t.Helper()
	groups, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Log("native cgroup evidence unavailable", err)
		return
	}
	for _, line := range strings.Split(string(groups), "\n") {
		if strings.HasPrefix(line, "0::/") {
			root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(line, "0::"))
			for _, name := range []string{"cpu.pressure", "memory.pressure", "io.pressure", "memory.events", "memory.current", "pids.events", "cpu.stat"} {
				raw, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Log("actual native pressure unavailable", name, err)
				} else {
					t.Log("actual native pressure", name, string(raw))
				}
			}
		}
	}
}

type nativeJoinOperationPlan struct {
	Original nativeJoinAuthorityPlan
	Signals  string
}

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeJoinOperationBoundariesTCP(t *testing.T) {
	if path := os.Getenv("ARDENTS_JOIN_OPERATION_FIXTURE"); path != "" {
		nativeJoinOperationChild(t, path)
		return
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-spend-concurrent"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeJoinOperationParent(t, carrier, phase) })
		}
	}
}

func nativeJoinOperationStack(registrationOnly bool) bool {
	var pcs [64]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs[:])])
	for {
		frame, more := frames.Next()
		if registrationOnly && strings.Contains(frame.Function, "/route/join.") && strings.HasSuffix(frame.Function, ".ServeRegistration") || !registrationOnly && strings.Contains(frame.Function, "/route/receiver.") && strings.HasSuffix(frame.Function, ".serveRole") {
			return true
		}
		if !more {
			return false
		}
	}
}

func nativeJoinOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan nativeJoinOperationPlan
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
	member, err := view.Member([32]byte{20}, view.ObservedAt())
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
		if _, armed := os.Stat(filepath.Join(plan.Signals, "arm")); armed == nil && nativeJoinOperationStack(false) {
			journal, readErr := os.ReadFile(filepath.Join(p.Spend, "closed-token-spends"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			spent := len(journal) > 8+32+32+32+8
			point := spent
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
				point = !spent && observed.ReservedBytes >= (32<<20)+(128<<10)
			}
			if point && held.CompareAndSwap(false, true) {
				nativeIssuerMarker(t, plan.Signals, "entered")
				nativeIssuerWaitMarker(t, plan.Signals, "release", 10*time.Second)
			}
		}
		return actual, err
	}
	original, err := node.ListenJoin(ctx, node.JoinConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Work: hosting.Traffic{Tx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("original Join startup", err)
	}
	t.Cleanup(func() { nativeIssuerMarker(t, plan.Signals, "release"); cancel(); _ = original.Close() })
	nativeIssuerMarker(t, plan.Signals, "listening")
	nativeIssuerWaitMarker(t, plan.Signals, "stop", 20*time.Second)
	changedAuthority := p.Fault == "pre-spend-successor" || p.Fault == "pre-spend-process-loss"
	if p.Fault == "pre-spend-successor" {
		if _, err := owner.AcceptEpoch(t.Context(), p.Successor.Epoch.Raw, p.Successor.Epoch.Inputs, p.Successor.Epoch.Materials); err != nil {
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
	checkNativeJoinRetained(t, p, member, binding)
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
	if p.Fault == "post-spend-concurrent" {
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
	} else if first != "caller" || cleanup != "" && cleanup != "route" {
		t.Fatal("original failure ownership changed", first, cleanup, result)
	}
	copy := *original
	if copy.Close() != result || copy.Done() != original.Done() || copy.JoinedOutcome() != original.JoinedOutcome() {
		t.Fatal("copied Close changed original completion")
	}
	nativeIssuerMarker(t, plan.Signals, "joined")
}

func nativeJoinOperationParent(t *testing.T, carrier transport.CarrierProfile, phase string) {
	t.Helper()
	f, sockets, certificates := newJoinRouteFixture(t, carrier)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	holder := joinRouteStock(t, f, admission.AllocationUser)
	startJoinRouteReceiverRange(t, f, sockets, certificates, 19)
	cert, key := nativeDescriptorCredential(t, certificates[[32]byte{20}])
	p := nativeJoinAuthorityPlan{Network: seedNativeNodeState(t, f, bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: routeProcessBudget(t), Certificate: cert, PrivateKey: key, Fault: phase}
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
	plan := nativeJoinOperationPlan{Original: p, Signals: t.TempDir()}
	sockets[[32]byte{20}]()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	unit := fmt.Sprintf("ardents-join-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_JOIN_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", binary, "-test.v=true", "-test.run=^TestNativeJoinOperationBoundariesTCP$", "-test.timeout=30s")
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
	root := t.TempDir()
	prefixPlan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: routeProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	routeContext, err := newRouteJoinContext(t.Context(), prefixPlan, f.authority, holder)
	if err != nil {
		t.Fatal(err)
	}
	defer routeContext.close()
	user, err := routeContext.open(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer user.close()
	var intent routeJoinIntent
	for choice := uint8(0); choice < 2; choice++ {
		recipient, e := user.recipient(choice)
		if e != nil {
			t.Fatal(e)
		}
		if recipient.Node == [32]byte{20} {
			intent = routeJoinIntent{Choice: choice, Node: recipient.Node, Generation: recipient.Generation, Deadline: minRouteDeadline(prefixPlan.Deadline, recipient.NotAfter), SetupDeadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
			break
		}
	}
	if intent.Node != [32]byte{20} {
		t.Fatal("original receiving duty not selected")
	}
	if _, err := rand.Read(intent.Secret[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(intent.Context[:]); err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(p.Spend, "closed-token-spends")
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	nativeIssuerMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		var stream net.Conn
		stream, err := user.join(caller, intent)
		if stream != nil {
			err = errors.Join(err, stream.Close())
		}
		completed <- err
	}()
	nativeIssuerWaitMarker(t, plan.Signals, "entered", 3*time.Second)
	atHold, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(phase, "pre-spend") != bytes.Equal(before, atHold) {
		t.Fatal("incorrect actual spend boundary", phase)
	}
	budget, err := hosting.Open(p.Hosting)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	assertHeld := func() {
		t.Helper()
		observed, e := budget.Observe(t.Context())
		if e != nil || observed.ReservedBytes < (32<<20)+(128<<10) {
			t.Fatal("original physical reservation returned before join", observed, e)
		}
	}
	assertHeld()
	changedAuthority := phase == "pre-spend-successor" || phase == "pre-spend-process-loss"
	if phase == "pre-spend-process-loss" {
		if e := exec.Command("systemctl", "set-property", "--runtime", unit, "CPUQuota=50%").Run(); e != nil {
			t.Fatal(e)
		}
	}
	nativeIssuerMarker(t, plan.Signals, "stop")
	marker := "denied"
	if changedAuthority {
		marker = "changed"
	}
	nativeIssuerWaitMarker(t, plan.Signals, marker, time.Second)
	assertHeld()
	cancel()
	if e := <-completed; e == nil {
		t.Fatal("retired original returned accepting stream")
	}
	assertHeld()
	if _, e := os.Stat(filepath.Join(plan.Signals, "joined")); e == nil {
		t.Fatal("held original reported join")
	}
	nativeIssuerMarker(t, plan.Signals, "release")
	nativeIssuerWaitMarker(t, plan.Signals, "joined", 3*time.Second)
	if e := command.Wait(); e != nil {
		t.Fatal("original operation child", e, diagnostic.String())
	}
	joined = true
	after, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(atHold, after) {
		t.Fatal("denial added or refunded spend", err)
	}
	observed, err := budget.Observe(t.Context())
	if err != nil || observed.ReservedBytes != 0 {
		t.Fatal("joined original leaked Hosting", observed, err)
	}
	if e := user.close(); e != nil {
		t.Log("original holder refusal cleanup", e)
	}
}
