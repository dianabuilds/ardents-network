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
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeDescriptorOperationBoundariesTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-commit", "post-commit-late-store-loss", "post-commit-concurrent"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeDescriptorOperationParent(t, carrier, phase) })
		}
	}
}

func nativeDescriptorOperationParent(t *testing.T, carrier transport.CarrierProfile, phase string) {
	t.Helper()
	f, sockets, certificates := descriptorRouteFixture(t, carrier)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := processfixture.StartObservedClock(t)
	defer closeClock()
	holder := descriptorRouteStock(t, f)
	proof, target := routeSignedDescriptor(t, f, 3, [32]byte{18})
	binary := processfixture.PinnedCommand(t)
	var peers []*processfixture.NativeProcess
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
		cert, key := processfixture.WriteNativeCredential(t, certificates[id])
		config := map[string]any{"network": processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), "node_id": id, "process_profile": "ardents-forwarding-process-v1", "retirement_wait_ms": 5000, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": processfixture.ProcessBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		sockets[id]()
		peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, hostingConfig(t, config), id[0]))
	}
	cert, key := processfixture.WriteNativeCredential(t, certificates[[32]byte{17}])
	p := processfixture.DescriptorAuthorityPlan{Network: processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Descriptor: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: phase}
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
	plan := processfixture.DescriptorOperationPlan{Original: p, Signals: t.TempDir()}
	sockets[[32]byte{17}]()
	testBinary := processfixture.PinnedOwnerTest(t)
	unit := fmt.Sprintf("ardents-descriptor-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_DESCRIPTOR_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeNodeDescriptorOperationProcess$", "-test.timeout=30s")
	var diagnostic processfixture.DiagnosticBuffer
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		processfixture.WriteOperationMarker(t, plan.Signals, "stop")
		if !joined {
			_ = command.Wait()
		}
		if t.Failed() {
			t.Log(diagnostic.String())
		}
	})
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	prefixPlan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
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
	processfixture.WriteOperationMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- prefix.publishDescriptor(caller, proof) }()
	processfixture.WaitOperationMarker(t, plan.Signals, "entered", 3*time.Second)
	atHold, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(phase, "pre-spend") != bytes.Equal(before, atHold) {
		t.Fatal("incorrect real spend boundary", phase)
	}
	if count := processfixture.DescriptorRecordCount(t, p.Descriptor); strings.HasPrefix(phase, "post-commit") && count != 1 || !strings.HasPrefix(phase, "post-commit") && count != 0 {
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
	processfixture.WriteOperationMarker(t, plan.Signals, "stop")
	marker := "denied"
	if changedAuthority {
		marker = "changed"
	}
	processfixture.WaitOperationMarker(t, plan.Signals, marker, time.Second)
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
	processfixture.WriteOperationMarker(t, plan.Signals, "release")
	processfixture.WaitOperationMarker(t, plan.Signals, "joined", 3*time.Second)
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
		signals = append(signals, peer.Unit)
	}
	if output, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	for _, peer := range peers {
		peer.Signaled = true
		peer.Stop(t, true)
	}
}

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeIntroductionOperationBoundariesTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-commit", "post-commit-concurrent", "post-commit-late-history-loss"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeIntroductionOperationParent(t, carrier, phase) })
		}
	}
}

func nativeIntroductionOperationParent(t *testing.T, carrier transport.CarrierProfile, phase string) {
	t.Helper()
	f, sockets, certificates := publicationRouteNetwork(t, carrier)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := processfixture.StartObservedClock(t)
	defer closeClock()
	holder := nativeIntroductionStock(t, f)
	binary := processfixture.PinnedCommand(t)
	var peers []*processfixture.NativeProcess
	for _, id := range [][32]byte{{21}, {22}, {23}, {24}} {
		cert, key := processfixture.WriteNativeCredential(t, certificates[id])
		config := map[string]any{"network": processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), "node_id": id, "process_profile": "ardents-forwarding-process-v1", "retirement_wait_ms": 5000, "participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(), "hosting_root": processfixture.ProcessBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		sockets[id]()
		peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, hostingConfig(t, config), id[0]))
	}
	cert, key := processfixture.WriteNativeCredential(t, certificates[[32]byte{25}])
	p := processfixture.IntroductionAuthorityPlan{Network: processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Introduction: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: phase}
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
	plan := processfixture.IntroductionOperationPlan{Original: p, Signals: t.TempDir()}
	sockets[[32]byte{25}]()
	testBinary := processfixture.PinnedOwnerTest(t)
	unit := fmt.Sprintf("ardents-introduction-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_INTRODUCTION_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeNodeIntroductionOperationProcess$", "-test.timeout=30s")
	var diagnostic processfixture.DiagnosticBuffer
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		processfixture.WriteOperationMarker(t, plan.Signals, "stop")
		if !joined {
			_ = command.Wait()
		}
		if t.Failed() {
			t.Log(diagnostic.String())
		}
	})
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	prefixPlan := routePrefixPlan{Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	selectionRoot := t.TempDir()
	selected, err := selection.Open(selection.Config{EntryRoot: filepath.Join(selectionRoot, "entry"), InteriorRoot: filepath.Join(selectionRoot, "interior"), Domain: 4, Current: f.authority.current})
	if err != nil {
		t.Fatal(err)
	}
	defer selected.Close()
	prefixBudget, err := hosting.Open(processfixture.ProcessBudget(t))
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
	processfixture.WriteOperationMarker(t, plan.Signals, "arm")
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
	processfixture.WaitOperationMarker(t, plan.Signals, "entered", 3*time.Second)
	atHold, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(phase, "pre-spend") != bytes.Equal(before, atHold) {
		t.Fatal("incorrect real spend boundary", phase)
	}
	if count := processfixture.IntroductionRecordCount(t, p.Introduction); strings.HasPrefix(phase, "post-commit") && count != 1 || !strings.HasPrefix(phase, "post-commit") && count != 0 {
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
	processfixture.WriteOperationMarker(t, plan.Signals, "stop")
	marker := "denied"
	if changedAuthority {
		marker = "changed"
	}
	processfixture.WaitOperationMarker(t, plan.Signals, marker, time.Second)
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
	processfixture.WriteOperationMarker(t, plan.Signals, "release")
	processfixture.WaitOperationMarker(t, plan.Signals, "joined", 3*time.Second)
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
	count := processfixture.IntroductionRecordCount(t, p.Introduction)
	if strings.HasPrefix(phase, "post-commit") && count != 1 || !strings.HasPrefix(phase, "post-commit") && count != 0 {
		t.Fatal("denial changed durable non-reclaim floor", phase, count)
	}
	if err := prefix.Close(); err != nil {
		t.Log("retained holder prefix cleanup after original denial", err)
	}
	signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
	for _, peer := range peers {
		signals = append(signals, peer.Unit)
	}
	if output, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	for _, peer := range peers {
		peer.Signaled = true
		peer.Stop(t, true)
	}
}

// Only schedules a real State return after actual result-journal growth. The
// child opens the genuine original Node; the parent owns independent genuine
// Stock and physical Prefixes. No fixture signs a successful runtime response.
func TestNativeIssuerOperationJoinAndExactRetryTCP(t *testing.T) {
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
	clock, closeClock := processfixture.StartObservedClock(t)
	defer closeClock()
	var peers []*joinCommandProcess
	binary := processfixture.PinnedCommand(t)
	for _, id := range [][32]byte{{12}, {13}, {14}, {15}} {
		cert, key := nativeIssuerOperationCredential(t, certificates[id])
		plan := map[string]any{"network": processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), "node_id": id, "spend_root": t.TempDir(), "hosting_root": nativeIssuerOperationBudget(t), "certificate": cert, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
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
	plan := processfixture.IssuerOperationPlan{Original: processfixture.IssuerAuthorityPlan{Network: processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Certificate: certificate, PrivateKey: key, Issuance: f.plan}, Signals: t.TempDir(), ResultBytes: resultInfo.Size()}
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
	testBinary := processfixture.PinnedOwnerTest(t)
	unit := fmt.Sprintf("ardents-issuer-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=60s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_ISSUER_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", testBinary, "-test.v=true", "-test.run=^TestNativeNodeIssuerOperationProcess$", "-test.timeout=30s")
	var diagnostic processfixture.DiagnosticBuffer
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		processfixture.WriteOperationMarker(t, plan.Signals, "stop")
		if !joined {
			_ = command.Wait()
		}
		if t.Failed() {
			t.Log("native issuing child result", diagnostic.String())
		}
	})
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	var prefixes [2]routeHandle
	for i := range prefixes {
		p := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		prefixes[i], err = startRoutePrefix(t.Context(), p, f.authority, holders[i])
		if err != nil {
			t.Fatal("genuine operation Prefix unavailable", i, err)
		}
		defer prefixes[i].close()
	}
	processfixture.WriteOperationMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- prefixes[0].issue(caller, 2, nil) }()
	processfixture.WaitOperationMarker(t, plan.Signals, "entered", 3*time.Second)
	processfixture.RequireIssuerRootsHeld(t, f.plan)
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
	if mode != "retry" && !processfixture.IssuerStorageFault(mode) {
		processfixture.WriteOperationMarker(t, plan.Signals, "stop")
		processfixture.WaitOperationMarker(t, plan.Signals, "denied", time.Second)
		processfixture.RequireIssuerRootsHeld(t, f.plan)
		observed, err = budget.Observe(t.Context())
		if err != nil || observed.ReservedBytes == 0 {
			t.Fatal("original denial returned unjoined work", err, observed.ReservedBytes)
		}
	}
	if processfixture.IssuerStorageFault(mode) {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
	} else {
		cancel()
	}
	if err := <-completed; err == nil || mode == "retry" && !errors.Is(err, context.Canceled) {
		// A remote original caller's cancellation may reach this independent
		// client as physical EOF. The child separately requires that exact
		// original context cause/owner; a successful delivery is always forbidden.
		t.Fatal("denied delivery became successful or lost local cancellation", err)
	}
	if !processfixture.IssuerStorageFault(mode) {
		processfixture.RequireIssuerRootsHeld(t, f.plan)
	}
	processfixture.WriteOperationMarker(t, plan.Signals, "release")
	// Wait for actual work-owned reservation return, never refund or retry in
	// an automatic loop. This observes physical cleanup, not a success flag.
	if mode != "retry" {
		processfixture.WaitOperationMarker(t, plan.Signals, "joined", 3*time.Second)
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
	} else if processfixture.IssuerStorageFault(mode) {
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
	processfixture.WriteOperationMarker(t, plan.Signals, "stop")
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

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeJoinOperationBoundariesTCP(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-spend-concurrent"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeJoinOperationParent(t, carrier, phase) })
		}
	}
}

func nativeJoinOperationParent(t *testing.T, carrier transport.CarrierProfile, phase string) {
	t.Helper()
	f, sockets, certificates := newJoinRouteFixture(t, carrier)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := processfixture.StartObservedClock(t)
	defer closeClock()
	holder := joinRouteStock(t, f, admission.AllocationUser)
	startJoinRouteReceiverRange(t, f, sockets, certificates, 19)
	cert, key := processfixture.WriteNativeCredential(t, certificates[[32]byte{20}])
	p := processfixture.JoinAuthorityPlan{Network: processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Certificate: cert, PrivateKey: key, Fault: phase}
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
	plan := processfixture.JoinOperationPlan{Original: p, Signals: t.TempDir()}
	sockets[[32]byte{20}]()
	binary := processfixture.PinnedOwnerTest(t)
	unit := fmt.Sprintf("ardents-join-operation-%d.service", os.Getpid())
	command := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit, "--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s", "--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--setenv=ARDENTS_JOIN_OPERATION_FIXTURE="+hostingConfig(t, plan), "--", binary, "-test.v=true", "-test.run=^TestNativeNodeJoinOperationProcess$", "-test.timeout=30s")
	var diagnostic processfixture.DiagnosticBuffer
	command.Stdout, command.Stderr = &diagnostic, &diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	joined := false
	t.Cleanup(func() {
		processfixture.WriteOperationMarker(t, plan.Signals, "release")
		processfixture.WriteOperationMarker(t, plan.Signals, "stop")
		if !joined {
			_ = command.Wait()
		}
		if t.Failed() {
			t.Log(diagnostic.String())
		}
	})
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	root := t.TempDir()
	prefixPlan := routePrefixPlan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
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
	processfixture.WriteOperationMarker(t, plan.Signals, "arm")
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
	processfixture.WaitOperationMarker(t, plan.Signals, "entered", 3*time.Second)
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
	processfixture.WriteOperationMarker(t, plan.Signals, "stop")
	marker := "denied"
	if changedAuthority {
		marker = "changed"
	}
	processfixture.WaitOperationMarker(t, plan.Signals, marker, time.Second)
	assertHeld()
	cancel()
	if e := <-completed; e == nil {
		t.Fatal("retired original returned accepting stream")
	}
	assertHeld()
	if _, e := os.Stat(filepath.Join(plan.Signals, "joined")); e == nil {
		t.Fatal("held original reported join")
	}
	processfixture.WriteOperationMarker(t, plan.Signals, "release")
	processfixture.WaitOperationMarker(t, plan.Signals, "joined", 3*time.Second)
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
