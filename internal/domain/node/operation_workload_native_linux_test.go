//go:build linux && node_process_native

package node_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/session"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

// Fault scheduling pauses an actual State return on the real operation stack.
// Files only coordinate the independent test driver; they authorize no effect.
func TestNativeDescriptorOperationBoundariesTCP(t *testing.T) {
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-commit", "post-commit-late-store-loss", "post-commit-concurrent"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeDescriptorOperationParent(t, carrier, phase) })
		}
	}
}

func nativeDescriptorOperationParent(t *testing.T, carrier string, phase string) {
	t.Helper()
	fixture := openOperationNetwork(t, carrier, "descriptor")
	holder := fixture.stock(t, admission.AllocationUser, [3]uint32{8, 4, 0}, 41,
		operationBatch{class: admission.ForwardClass, ids: []byte{12, 13, 14, 15}},
		operationBatch{class: admission.ControlClass, ids: []byte{17, 17, 17, 17, 17, 17, 17, 17}})
	proof, target := processfixture.SignedDescriptor(t, fixture.profile, 3, [32]byte{18}, fixture.profile.Digest)
	peers := fixture.startForwarding(t, 256, 12, 13, 14, 15)
	cert, key := processfixture.WriteNativeCredential(t, fixture.certificates[[32]byte{17}])
	p := processfixture.DescriptorAuthorityPlan{Network: fixture.seed(t), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Descriptor: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: phase}
	if phase == "pre-spend-successor" {
		next := fixture.spec
		next.Number++
		next.Previous = fixture.bundle.Epoch.Digest
		next.Seed[0]++
		var err error
		p.Successor, err = networkfixture.BuildClosed(next)
		if err != nil {
			t.Fatal(err)
		}
	}
	plan := processfixture.DescriptorOperationPlan{Original: p, Signals: t.TempDir()}
	fixture.releasePort(t, 17)
	unit := fmt.Sprintf("ardents-descriptor-operation-%d.service", os.Getpid())
	process := processfixture.StartOperationProcess(t, unit, "TestNativeNodeDescriptorOperationProcess", "ARDENTS_DESCRIPTOR_OPERATION_FIXTURE", writeAuthorityProcessPlan(t, plan), plan.Signals, 120)
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	prefixPlan := funding.Plan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	prefix, err := session.OpenPrefix(t.Context(), prefixPlan, fixture.routeAuthority, holder)
	if err != nil {
		t.Fatal("genuine operation Prefix", err)
	}
	defer prefix.Close()
	journalPath := filepath.Join(p.Spend, "closed-token-spends")
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	processfixture.WriteOperationMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- prefix.PublishDescriptor(caller, proof) }()
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
		got, err := prefix.LookupDescriptor(t.Context(), target, history)
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
	if err := process.Wait(); err != nil {
		t.Fatal("original operation child", err, process.Diagnostic.String())
	}
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
	fresh, err := reachability.OpenStore(reachability.StoreConfig{Root: p.Descriptor, Network: fixture.profile.NetworkID})
	if err != nil {
		t.Fatal("independent Store reopen", err)
	}
	got, outcome, lookupErr := fresh.Lookup(target, fixture.profile.Digest, time.Now())
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
	if err := prefix.Close(); err != nil {
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
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-commit", "post-commit-concurrent", "post-commit-late-history-loss"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeIntroductionOperationParent(t, carrier, phase) })
		}
	}
}

func nativeIntroductionOperationParent(t *testing.T, carrier string, phase string) {
	t.Helper()
	fixture := openOperationNetwork(t, carrier, "introduction")
	holder := processfixture.IntroductionStock(t, fixture.spec, fixture.plan, fixture.state.CurrentRuntime)
	peers := fixture.startForwarding(t, 100, 21, 22, 23, 24)
	cert, key := processfixture.WriteNativeCredential(t, fixture.certificates[[32]byte{25}])
	p := processfixture.IntroductionAuthorityPlan{Network: fixture.seed(t), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Introduction: t.TempDir(), Certificate: cert, PrivateKey: key, Fault: phase}
	if phase == "pre-spend-successor" {
		next := fixture.spec
		next.Number++
		next.Previous = fixture.bundle.Epoch.Digest
		next.Seed[0]++
		var err error
		p.Successor, err = networkfixture.BuildClosed(next)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(p.Introduction, 0700); err != nil {
		t.Fatal(err)
	}
	plan := processfixture.IntroductionOperationPlan{Original: p, Signals: t.TempDir()}
	fixture.releasePort(t, 25)
	unit := fmt.Sprintf("ardents-introduction-operation-%d.service", os.Getpid())
	process := processfixture.StartOperationProcess(t, unit, "TestNativeNodeIntroductionOperationProcess", "ARDENTS_INTRODUCTION_OPERATION_FIXTURE", writeAuthorityProcessPlan(t, plan), plan.Signals, 120)
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	root := t.TempDir()
	prefixPlan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 4,
		Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	prefix, err := session.OpenPrefix(t.Context(), prefixPlan, fixture.routeAuthority, holder)
	if err != nil {
		t.Fatal("genuine Registration prefix", err)
	}
	defer prefix.Close()
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
		registration, err := prefix.Register(caller, 1)
		if registration.Owner() != nil {
			err = errors.Join(err, registration.Owner().Close())
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
	if err := process.Wait(); err != nil {
		t.Fatal("original operation child", err, process.Diagnostic.String())
	}
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
	retained, err := introduction.OpenHistory(p.Introduction, introduction.Binding{NetworkID: fixture.profile.NetworkID, ProfileDigest: fixture.profile.Digest, ReceiverNodeID: [32]byte{25}, ReceiverDutyGeneration: 9})
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
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		for _, mode := range []string{"retry", "pre-debit-caller", "post-debit-caller", "post-result-caller", "post-result-successor"} {
			t.Run(string(carrier)+"/"+mode, func(t *testing.T) { nativeIssuerOperationParent(t, carrier, mode) })
		}
	}
}

func TestNativeIssuerStorageUncertaintyTCP(t *testing.T) {
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		for _, mode := range []string{"quota-uncertain", "result-uncertain"} {
			t.Run(string(carrier)+"/"+mode, func(t *testing.T) { nativeIssuerOperationParent(t, carrier, mode) })
		}
	}
}

// These genuine offline tokens isolate the operation fault. Actual command
// bootstrap is separately mandatory in the same native profile.
func nativeIssuerOperationParent(t *testing.T, carrier string, mode string) {
	t.Helper()
	fixture := openOperationNetwork(t, carrier, "issuer")
	var holders [2]*stock.Owner
	for i := range holders {
		holders[i] = fixture.stock(t, admission.AllocationUser, [3]uint32{4, 8, 0}, 170,
			operationBatch{class: admission.ForwardClass, ids: []byte{12, 13, 14, 15}},
			operationBatch{class: admission.ControlClass, ids: []byte{11, 11, 11}})
	}
	peers := fixture.startForwarding(t, 256, 12, 13, 14, 15)
	certificate, key := processfixture.WriteNativeCredential(t, fixture.certificates[fixture.profile.IssuerNodeID])
	resultInfo, err := os.Stat(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	plan := processfixture.IssuerOperationPlan{Original: processfixture.IssuerAuthorityPlan{Network: fixture.seed(t), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Certificate: certificate, PrivateKey: key, Issuance: fixture.plan}, Signals: t.TempDir(), ResultBytes: resultInfo.Size()}
	initialQuota, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
	if err != nil {
		t.Fatal(err)
	}
	initialResult, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	plan.QuotaBytes = int64(len(initialQuota))
	plan.Original.Fault = mode
	if mode == "post-result-successor" {
		next := fixture.spec
		next.Number++
		next.Previous = fixture.bundle.Epoch.Digest
		next.Seed[0]++
		plan.Original.Successor, err = networkfixture.BuildClosed(next)
		if err != nil {
			t.Fatal(err)
		}
	}
	fixture.releasePort(t, 11)
	unit := fmt.Sprintf("ardents-issuer-operation-%d.service", os.Getpid())
	process := processfixture.StartOperationProcess(t, unit, "TestNativeNodeIssuerOperationProcess", "ARDENTS_ISSUER_OPERATION_FIXTURE", writeAuthorityProcessPlan(t, plan), plan.Signals, 60)
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	var prefixes [2]session.Prefix
	for i := range prefixes {
		p := funding.Plan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
		prefixes[i], err = session.OpenPrefix(t.Context(), p, fixture.routeAuthority, holders[i])
		if err != nil {
			t.Fatal("genuine operation Prefix unavailable", i, err)
		}
		defer prefixes[i].Close()
	}
	processfixture.WriteOperationMarker(t, plan.Signals, "arm")
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	completed := make(chan error, 1)
	go func() { completed <- prefixes[0].Issue(caller, 2, nil) }()
	processfixture.WaitOperationMarker(t, plan.Signals, "entered", 3*time.Second)
	processfixture.RequireIssuerRootsHeld(t, fixture.plan)
	before, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
	if err != nil {
		t.Fatal(err)
	}
	committedAtHold, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
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
	if err := prefixes[1].Issue(t.Context(), 2, nil); err == nil {
		t.Fatal("concurrent private issuing operation accepted")
	}
	after, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
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
		processfixture.RequireIssuerRootsHeld(t, fixture.plan)
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
		processfixture.RequireIssuerRootsHeld(t, fixture.plan)
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
			t.Fatal("original operation did not return its reservation", observed.ReservedBytes, process.Diagnostic.String())
		}
		time.Sleep(time.Millisecond)
	}
	committed, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	if mode == "retry" {
		if err := prefixes[0].Issue(t.Context(), 2, nil); err != nil {
			t.Fatal("explicit original exact retry", err)
		}
		quotaAfter, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
		if err != nil || !bytes.Equal(before, quotaAfter) {
			t.Fatal("exact retry debited again", err)
		}
		resultAfter, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
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
		q, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
		if err != nil {
			t.Fatal(err)
		}
		r, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
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
		quotaAfter, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
		if err != nil || !bytes.Equal(before, quotaAfter) {
			t.Fatal("original denial changed retained quota", err)
		}
		resultAfter, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
		if err != nil || !bytes.Equal(committedAtHold, resultAfter) {
			t.Fatal("original denial changed retained result", err)
		}
	}
	for _, p := range prefixes {
		_ = p.Close()
	}
	processfixture.WriteOperationMarker(t, plan.Signals, "stop")
	if err := process.Wait(); err != nil {
		t.Fatal("original native child did not join", err, process.Diagnostic.String())
	}
	if mode == "retry" {
		request, response := nativeIssuerCommittedPair(t, initialResult, committed)
		defer clear(request)
		defer clear(response)
		beforeQuota, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
		if err != nil {
			t.Fatal(err)
		}
		reopened, opened := issuer.OpenCurrent(t.Context(), fixture.plan, func() (admission.AuthorityFacts, time.Time, error) {
			return fixture.issuerObservation()
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
		afterQuota, err := os.ReadFile(filepath.Join(fixture.plan.AdmissionRoot, "admission.journal"))
		if err != nil || !bytes.Equal(beforeQuota, afterQuota) {
			t.Fatal("independent reopen debited again", err)
		}
		afterResult, err := os.ReadFile(filepath.Join(fixture.plan.ResultRoot, "results.journal"))
		if err != nil || !bytes.Equal(committed, afterResult) {
			t.Fatal("independent reopen rewrote retained result", err)
		}
	}
	for _, peer := range peers {
		peer.Stop(t, true)
	}
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
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		for _, phase := range []string{"pre-spend", "pre-spend-stop", "pre-spend-successor", "pre-spend-process-loss", "post-spend", "post-spend-concurrent"} {
			t.Run(string(carrier)+"/"+phase, func(t *testing.T) { nativeJoinOperationParent(t, carrier, phase) })
		}
	}
}

func nativeJoinOperationParent(t *testing.T, carrier string, phase string) {
	t.Helper()
	fixture := openOperationNetwork(t, carrier, "join")
	holder := fixture.stock(t, admission.AllocationUser, [3]uint32{0, 10, 0}, 31,
		operationBatch{class: admission.ForwardClass, ids: []byte{12, 13, 14, 15, 16, 17, 18, 19, 20, 21}})
	peers := fixture.startForwarding(t, 256, 12, 13, 14, 15, 16, 17, 18, 19)
	cert, key := processfixture.WriteNativeCredential(t, fixture.certificates[[32]byte{20}])
	p := processfixture.JoinAuthorityPlan{Network: fixture.seed(t), Participation: filepath.Join(t.TempDir(), "participation"), Spend: t.TempDir(), Hosting: processfixture.ProcessBudget(t), Certificate: cert, PrivateKey: key, Fault: phase}
	if phase == "pre-spend-successor" {
		next := fixture.spec
		next.Number++
		next.Previous = fixture.bundle.Epoch.Digest
		next.Seed[0]++
		var err error
		p.Successor, err = networkfixture.BuildClosed(next)
		if err != nil {
			t.Fatal(err)
		}
	}
	plan := processfixture.JoinOperationPlan{Original: p, Signals: t.TempDir()}
	fixture.releasePort(t, 20)
	unit := fmt.Sprintf("ardents-join-operation-%d.service", os.Getpid())
	process := processfixture.StartOperationProcess(t, unit, "TestNativeNodeJoinOperationProcess", "ARDENTS_JOIN_OPERATION_FIXTURE", writeAuthorityProcessPlan(t, plan), plan.Signals, 120)
	processfixture.WaitOperationMarker(t, plan.Signals, "listening", 5*time.Second)
	root := t.TempDir()
	prefixPlan := funding.Plan{EntryRoot: filepath.Join(root, "entry"), InteriorRoot: filepath.Join(root, "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
	routeContext, err := session.OpenJoinContext(t.Context(), prefixPlan, fixture.routeAuthority, holder)
	if err != nil {
		t.Fatal(err)
	}
	defer routeContext.Close()
	user, err := routeContext.Open(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	var intent session.JoinIntent
	for choice := uint8(0); choice < 2; choice++ {
		recipient, e := user.Recipient(choice)
		if e != nil {
			t.Fatal(e)
		}
		if recipient.Node == [32]byte{20} {
			intent = session.JoinIntent{Choice: choice, Node: recipient.Node, Generation: recipient.Generation, Deadline: earlierOperationDeadline(prefixPlan.Deadline, recipient.NotAfter), SetupDeadline: time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)}
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
		stream, err := user.Join(caller, intent)
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
	if e := process.Wait(); e != nil {
		t.Fatal("original operation child", e, process.Diagnostic.String())
	}
	after, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(atHold, after) {
		t.Fatal("denial added or refunded spend", err)
	}
	observed, err := budget.Observe(t.Context())
	if err != nil || observed.ReservedBytes != 0 {
		t.Fatal("joined original leaked Hosting", observed, err)
	}
	if e := user.Close(); e != nil {
		t.Log("original holder refusal cleanup", e)
	}
	for _, peer := range peers {
		peer.Stop(t, true)
	}
}

type operationNetwork struct {
	spec           networkfixture.ClosedSpec
	bundle         networkfixture.Closed
	plan           issuer.Plan
	state          *state.State
	profile        admission.AuthorityFacts
	routeAuthority funding.Authority
	listeners      map[[32]byte]net.Listener
	certificates   map[[32]byte]tls.Certificate
	clock          string
}

// openOperationNetwork constructs the independent signed input and live holder
// State. The measured Node and every Forwarding process open their own State.
func openOperationNetwork(t *testing.T, carrier, measuredRole string) *operationNetwork {
	t.Helper()
	processfixture.WaitAdmissionHour(t)
	spec, plan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	spec.Nodes = spec.Nodes[:1]
	fixture := &operationNetwork{plan: plan, listeners: make(map[[32]byte]net.Listener), certificates: make(map[[32]byte]tls.Certificate)}
	type duty struct{ id, domain, subrole byte }
	roster := []duty{{12, 1, 1}, {13, 1, 1}, {14, 1, 2}, {15, 1, 2}}
	switch measuredRole {
	case "descriptor":
		roster = append(roster, duty{17, 2, 5}, duty{18, 4, 3})
	case "introduction":
		roster = append(roster, duty{21, 4, 1}, duty{22, 4, 1}, duty{23, 4, 2}, duty{24, 4, 2}, duty{25, 4, 3})
	case "join":
		roster = append(roster, duty{16, 3, 1}, duty{17, 3, 1}, duty{18, 3, 2}, duty{19, 3, 2}, duty{20, 2, 4}, duty{21, 2, 4})
	case "issuer":
	default:
		t.Fatalf("unexpected measured role %q", measuredRole)
	}
	for _, duty := range roster {
		processfixture.AddReceivingFixtureNode(t, &spec, fixture.listeners, fixture.certificates, carrier, duty.id, duty.domain, duty.subrole)
	}
	if measuredRole == "issuer" {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		})
		original := &spec.Nodes[0]
		original.Endpoint = listener.Addr().String()
		fixture.listeners[original.NodeID] = listener
		fixture.certificates[original.NodeID] = processfixture.RoleCertificate(t, original.PrivateKey)
	}
	fixture.spec = spec
	var err error
	fixture.bundle, err = networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	networkinputs.PrepareIssuer(t, &fixture.plan, inventory, spec, fixture.bundle.Nodes[0], sha256.Sum256(fixture.bundle.Profile))
	clock, closeClock := networkinputs.StartObservedClock(t)
	fixture.clock = clock
	t.Cleanup(closeClock)
	declaration := fixture.seed(t)
	config, err := declaration.Config()
	if err != nil {
		t.Fatal(err)
	}
	fixture.state, err = state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.state.Close(); err != nil {
			t.Error(err)
		}
	})
	read := fixture.state.CurrentRuntime
	fixture.routeAuthority = funding.Observe(read, fixture.state.CurrentRuntimeWitness, funding.PresentationGuard(read))
	fixture.profile, _, err = fixture.issuerObservation()
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *operationNetwork) seed(t *testing.T) state.AuthorityDeclaration {
	t.Helper()
	return networkinputs.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), f.bundle, f.clock)
}

func (f *operationNetwork) issuerObservation() (admission.AuthorityFacts, time.Time, error) {
	view, err := f.state.CurrentRuntime()
	if err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	facts, now, err := funding.NetworkFacts(view)
	if err != nil {
		return admission.AuthorityFacts{}, time.Time{}, err
	}
	member, err := view.Member(facts.IssuerNodeID, now)
	if err != nil || member.PublicKey != f.plan.KeyBinding.Signer || member.DutyGeneration != facts.IssuerDutyGeneration || member.RoleDomain != 2 || member.Subrole != 6 {
		return admission.AuthorityFacts{}, time.Time{}, errors.Join(errors.New("original fixture Issuer unavailable"), err)
	}
	return facts, now, nil
}

func (f *operationNetwork) releasePort(t *testing.T, id byte) {
	t.Helper()
	if err := f.listeners[[32]byte{id}].Close(); err != nil {
		t.Fatal(err)
	}
}

func (f *operationNetwork) startForwarding(t *testing.T, capacity uint64, ids ...byte) []*processfixture.NativeProcess {
	t.Helper()
	binary := processfixture.PinnedCommand(t)
	var peers []*processfixture.NativeProcess
	for _, id := range ids {
		nodeID := [32]byte{id}
		cert, key := processfixture.WriteNativeCredential(t, f.certificates[nodeID])
		plan := map[string]any{
			"network": f.seed(t), "node_id": nodeID, "process_profile": processresource.Profile, "retirement_wait_ms": 5000,
			"participation_root": filepath.Join(t.TempDir(), "participation"), "spend_root": t.TempDir(),
			"hosting_root": processfixture.ProcessBudgetWithCapacity(t, capacity), "certificate": cert, "private_key": key,
			"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
		}
		f.releasePort(t, id)
		peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, writeAuthorityProcessPlan(t, plan), id))
	}
	return peers
}

type operationBatch struct {
	class admission.Class
	ids   []byte
}

func (f *operationNetwork) stock(t *testing.T, role admission.AllocationRole, maxima [3]uint32, first byte, batches ...operationBatch) *stock.Owner {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, role, f.issuerObservation)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	request, digest, err := holder.Request(maxima)
	if err != nil {
		t.Fatal(err)
	}
	profile, now, err := f.issuerObservation()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Import(digest, processfixture.FixturePermission(t, request, profile, f.spec.Authority, now)); err != nil {
		t.Fatal(err)
	}
	view, err := f.state.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	for index, batch := range batches {
		var challenges []token.ClosedTokenContext
		for _, id := range batch.ids {
			member, err := view.Member([32]byte{id}, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			challenges = append(challenges, token.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
				ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.DutyGeneration, Class: uint8(batch.class), WindowStart: now.UTC().Truncate(time.Hour)})
		}
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{first + byte(index)}, ProfileDigest: profile.Digest}, Bootstrap: true, Deadline: profile.NotAfter}
		attempt, err := holder.Begin(intent)
		if err != nil {
			t.Fatal(err)
		}
		raw, _, err := attempt.Request()
		if err != nil {
			t.Fatal(err)
		}
		issued := issuer.IssueCurrent(t.Context(), f.plan, raw, quota.Bootstrap, f.issuerObservation)
		clear(raw)
		if issued.Outcome != "issued-offline" {
			t.Fatal("genuine preparation issuance failed", issued.Phase, issued.Outcome)
		}
		err = attempt.Complete(issued.Response, nil)
		clear(issued.Response)
		if err != nil {
			t.Fatal(err)
		}
	}
	return holder
}

func earlierOperationDeadline(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
