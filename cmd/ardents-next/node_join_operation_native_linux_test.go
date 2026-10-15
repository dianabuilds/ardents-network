//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"net"
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
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type nativeJoinOperationPlan struct {
	Original nativeJoinAuthorityPlan
	Signals  string
}

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeJoinOperationBoundariesBothCarriers(t *testing.T) {
	if path := os.Getenv("ARDENTS_JOIN_OPERATION_FIXTURE"); path != "" {
		nativeJoinOperationChild(t, path)
		return
	}
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
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
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_JOIN_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", binary, "-test.v=true", "-test.run=^TestNativeJoinOperationBoundariesBothCarriers$", "-test.timeout=30s")
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
