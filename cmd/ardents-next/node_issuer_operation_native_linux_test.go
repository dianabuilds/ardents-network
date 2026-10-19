//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
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

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/node"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

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
		if _, err := stateOwner.Accept(t.Context(), p.Successor.Epoch.Raw, p.Successor.Epoch.Inputs, p.Successor.Epoch.Materials); err != nil {
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
