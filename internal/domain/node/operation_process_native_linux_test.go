//go:build linux && node_process_native

package node_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/node"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

func nativeDescriptorOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.DescriptorOperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	p := plan.Original
	config, err := p.Network.Config()
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
			committed := processfixture.DescriptorRecordCount(t, p.Descriptor) > 0
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
				processfixture.WriteOperationMarker(t, plan.Signals, "entered")
				processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
			}
		}
		return actual, err
	}
	original, err := node.ListenDescriptor(ctx, node.DescriptorConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, DescriptorRoot: p.Descriptor, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("original Descriptor startup", err)
	}
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		cancel()
		_ = original.Close()
	})
	processfixture.WriteOperationMarker(t, plan.Signals, "listening")
	processfixture.WaitOperationMarker(t, plan.Signals, "stop", 20*time.Second)
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
	processfixture.RequireReceivingRootsHeld(t, p.Participation, p.Spend, member, binding, filepath.Join(p.Descriptor, ".ardents-reachability-store-lock"))
	select {
	case <-original.Done():
		t.Fatal("held operation became joined")
	default:
	}
	if changedAuthority {
		// This marker proves only the changed input, never an accepting result
		// or a fake synchronous Node revoke. Real work must observe and deny it.
		processfixture.WriteOperationMarker(t, plan.Signals, "changed")
		processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
		select {
		case <-original.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("changed original authority did not deny/join actual work")
		}
	} else {
		processfixture.WriteOperationMarker(t, plan.Signals, "denied")
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
	processfixture.WriteOperationMarker(t, plan.Signals, "joined")
}

// TestNativeNodeDescriptorOperationProcess is invoked by the native workload driver under systemd.
func TestNativeNodeDescriptorOperationProcess(t *testing.T) {
	path := os.Getenv("ARDENTS_DESCRIPTOR_OPERATION_FIXTURE")
	if path == "" {
		t.Fatal("native operation fixture is required")
	}
	nativeDescriptorOperationChild(t, path)
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

func nativeIntroductionOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.IntroductionOperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	p := plan.Original
	config, err := p.Network.Config()
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
			committed := processfixture.IntroductionRecordCount(t, p.Introduction) > 0
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
				processfixture.WriteOperationMarker(t, plan.Signals, "entered")
				processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
			}
		}
		return actual, err
	}
	original, err := node.ListenIntroduction(ctx, node.IntroductionConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, IntroductionRoot: p.Introduction, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Work: hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("original Introduction startup", err)
	}
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		cancel()
		_ = original.Close()
	})
	processfixture.WriteOperationMarker(t, plan.Signals, "listening")
	processfixture.WaitOperationMarker(t, plan.Signals, "stop", 20*time.Second)
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
	processfixture.RequireReceivingRootsHeld(t, p.Participation, p.Spend, member, binding, filepath.Join(p.Introduction, ".ardents-introduction-lock"))
	select {
	case <-original.Done():
		t.Fatal("held operation became joined")
	default:
	}
	if changedAuthority {
		// This marker proves only the changed input, never an accepting result
		// or a fake synchronous Node revoke. Real work must observe and deny it.
		processfixture.WriteOperationMarker(t, plan.Signals, "changed")
		processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
		select {
		case <-original.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("changed original authority did not deny/join actual work")
		}
	} else {
		processfixture.WriteOperationMarker(t, plan.Signals, "denied")
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
	processfixture.WriteOperationMarker(t, plan.Signals, "joined")
}

// TestNativeNodeIntroductionOperationProcess is invoked by the native workload driver under systemd.
func TestNativeNodeIntroductionOperationProcess(t *testing.T) {
	path := os.Getenv("ARDENTS_INTRODUCTION_OPERATION_FIXTURE")
	if path == "" {
		t.Fatal("native operation fixture is required")
	}
	nativeIntroductionOperationChild(t, path)
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

func nativeIssuerOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.IssuerOperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	p := plan.Original
	config, err := p.Network.Config()
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
	if processfixture.IssuerStorageFault(p.Fault) {
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
				processfixture.WriteOperationMarker(t, plan.Signals, "entered")
				processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
				if processfixture.IssuerStorageFault(p.Fault) {
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
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		cancel()
		_ = original.Close()
	})
	processfixture.WriteOperationMarker(t, plan.Signals, "listening")
	if processfixture.IssuerStorageFault(p.Fault) {
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
		reopened, opened := issuer.OpenCurrent(t.Context(), p.Issuance, func() (admission.AuthorityFacts, time.Time, error) {
			view, err := stateOwner.CurrentRuntime()
			if err != nil {
				return admission.AuthorityFacts{}, time.Time{}, err
			}
			facts, now, err := funding.NetworkFacts(view)
			if err != nil {
				return admission.AuthorityFacts{}, time.Time{}, err
			}
			member, err := view.Member(facts.IssuerNodeID, now)
			signer := p.Issuance.KeyBinding.Signer
			if err != nil || signer == [32]byte{} || member.PublicKey != signer || member.DutyGeneration != facts.IssuerDutyGeneration || member.RoleDomain != 2 || member.Subrole != 6 {
				return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("network issuer duty is unavailable"), err)
			}
			return facts, now, nil
		})
		if opened.Outcome == "opened" {
			_ = reopened.Close(context.Background())
			t.Fatal("independent reopen repaired partial journal")
		}
		processfixture.WriteOperationMarker(t, plan.Signals, "joined")
		return
	}
	processfixture.WaitOperationMarker(t, plan.Signals, "stop", 20*time.Second)
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
		processfixture.WriteOperationMarker(t, plan.Signals, "denied")
		<-original.Done()
	} else {
		cancel()
		original.Stop()
		processfixture.WriteOperationMarker(t, plan.Signals, "denied")
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
	processfixture.RequireIssuerRootsReleased(t, p.Issuance)
	processfixture.WriteOperationMarker(t, plan.Signals, "joined")
}

// TestNativeNodeIssuerOperationProcess is invoked by the native workload driver under systemd.
func TestNativeNodeIssuerOperationProcess(t *testing.T) {
	path := os.Getenv("ARDENTS_ISSUER_OPERATION_FIXTURE")
	if path == "" {
		t.Fatal("native operation fixture is required")
	}
	nativeIssuerOperationChild(t, path)
}

func nativeJoinOperationChild(t *testing.T, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan processfixture.JoinOperationPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	p := plan.Original
	config, err := p.Network.Config()
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
				processfixture.WriteOperationMarker(t, plan.Signals, "entered")
				processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
			}
		}
		return actual, err
	}
	original, err := node.ListenJoin(ctx, node.JoinConfig{Authority: role.Authority{Current: current, Duty: retained, Profile: view.Profile().ProfileBinding}, ParticipationRoot: p.Participation, SpendRoot: p.Spend, HostingRoot: p.Hosting, Certificate: p.Certificate, PrivateKey: p.PrivateKey, Work: hosting.Traffic{Tx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
	if err != nil {
		t.Fatal("original Join startup", err)
	}
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		cancel()
		_ = original.Close()
	})
	processfixture.WriteOperationMarker(t, plan.Signals, "listening")
	processfixture.WaitOperationMarker(t, plan.Signals, "stop", 20*time.Second)
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
	processfixture.RequireReceivingRootsHeld(t, p.Participation, p.Spend, member, binding, "")
	select {
	case <-original.Done():
		t.Fatal("held operation became joined")
	default:
	}
	if changedAuthority {
		// This marker proves only the changed input, never an accepting result
		// or a fake synchronous Node revoke. Real work must observe and deny it.
		processfixture.WriteOperationMarker(t, plan.Signals, "changed")
		processfixture.WaitOperationMarker(t, plan.Signals, "release", 10*time.Second)
		select {
		case <-original.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("changed original authority did not deny/join actual work")
		}
	} else {
		processfixture.WriteOperationMarker(t, plan.Signals, "denied")
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
	processfixture.WriteOperationMarker(t, plan.Signals, "joined")
}

// TestNativeNodeJoinOperationProcess is invoked by the native workload driver under systemd.
func TestNativeNodeJoinOperationProcess(t *testing.T) {
	path := os.Getenv("ARDENTS_JOIN_OPERATION_FIXTURE")
	if path == "" {
		t.Fatal("native operation fixture is required")
	}
	nativeJoinOperationChild(t, path)
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
