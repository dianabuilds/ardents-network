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
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type nativeIntroductionOperationPlan struct {
	Original nativeIntroductionAuthorityPlan
	Signals  string
}

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeIntroductionOperationBoundariesBothCarriers(t *testing.T) {
	if path := os.Getenv("ARDENTS_INTRODUCTION_OPERATION_FIXTURE"); path != "" {
		nativeIntroductionOperationChild(t, path)
		return
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
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
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_INTRODUCTION_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeIntroductionOperationBoundariesBothCarriers$", "-test.timeout=30s")
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
