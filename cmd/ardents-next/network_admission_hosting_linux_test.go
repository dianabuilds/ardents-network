//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

// Signed Network and a genuinely allocated holder permission exercise the
// issuer's durable conflict boundary. Both requests have valid holder proofs;
// this is not malformed-input coverage or a Carrier/Stock consumer substitute.
func TestNetworkIssuerValidChangedDigestRefusesBeforeSigning(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	p, now, err := f.authority.observe()
	if err != nil {
		t.Fatal(err)
	}
	prepared, holderKey, err := admission.PreparePermissionRequest(p.IssuanceAuthorityKey, p.NetworkID, p.IssuerNodeID, p.IssuerDutyGeneration, admission.AllocationUser, now.Truncate(time.Hour), [3]uint32{0, 2, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holderKey)
	public, err := admission.EncodePermissionRequest(prepared)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(public)
	allocationRequest, err := allocation.Prepare(public, p.NetworkID, now)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := allocationRequest.Decide(nil, p.IssuanceAuthorityKey, now)
	if err != nil {
		t.Fatal(err)
	}
	allocationPath := filepath.Join(t.TempDir(), "allocation")
	if err := os.WriteFile(allocationPath, decision.Journal(), 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(allocationPath)
	if err != nil || !bytes.Equal(committed, decision.Journal()) {
		t.Fatal("allocation durable readback", err)
	}
	defer clear(committed)
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(f.spec.Authority, admission.PermissionTranscript(permission)))
	challenge := token.ClosedTokenContext{NetworkID: p.NetworkID, ProfileDigest: p.Digest, IssuerNodeID: p.IssuerNodeID, ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration, Class: 2, WindowStart: permission.NotBefore}
	var pending [2]*token.PendingClosedTokenBatch
	var raw [2][]byte
	for i := range pending {
		pending[i], err = token.PrepareClosedTokenBatch(token.ClosedTokenBatchConfig{Profile: p, Contexts: []token.ClosedTokenContext{challenge}, Permission: permission, HolderKey: holderKey, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		defer pending[i].Discard()
		raw[i] = pending[i].Request()
		defer clear(raw[i])
	}
	first, err := admission.DecodeClosedTokenBatch(raw[0])
	if err != nil {
		t.Fatal(err)
	}
	changed, err := admission.DecodeClosedTokenBatch(raw[1])
	if err != nil {
		t.Fatal(err)
	}
	changed.RequestID = first.RequestID
	copy(changed.Signature[:], ed25519.Sign(holderKey, admission.TokenBatchTranscript(changed)))
	changedRaw, err := admission.EncodeClosedTokenBatch(changed)
	if err != nil {
		t.Fatal("changed request must retain a valid holder proof", err)
	}
	defer clear(changedRaw)
	if bytes.Equal(changedRaw, raw[0]) {
		t.Fatal("digest conflict oracle has identical inputs")
	}
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		return f.authority.issuer(f.plan.KeyBinding.Signer)
	}
	issued := issuer.IssueCurrent(t.Context(), f.plan, raw[0], quota.Bootstrap, observe)
	defer clear(issued.Response)
	if issued.Outcome != "issued-offline" {
		t.Fatal("original genuine issuance", issued.Phase, issued.Outcome)
	}
	tokens, err := pending[0].FinalizeEncoded(issued.Response)
	for _, raw := range tokens {
		clear(raw)
	}
	if err != nil || len(tokens) != 1 {
		t.Fatal("original signature did not verify", err)
	}
	readJournal := func(root, name string) []byte {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	debitBefore, resultBefore := readJournal(f.plan.AdmissionRoot, "admission.journal"), readJournal(f.plan.ResultRoot, "results.journal")
	defer clear(debitBefore)
	defer clear(resultBefore)
	refused := issuer.IssueCurrent(t.Context(), f.plan, changedRaw, quota.Bootstrap, observe)
	defer clear(refused.Response)
	if refused.Phase != "debit" || refused.Outcome != "request-conflict" || refused.Response != nil {
		t.Fatal("valid changed digest reached signing or lost conflict category", refused.Phase, refused.Outcome)
	}
	retry := issuer.IssueCurrent(t.Context(), f.plan, raw[0], quota.Bootstrap, observe)
	defer clear(retry.Response)
	if retry.Outcome != "already-issued" || !bytes.Equal(retry.Response, issued.Response) {
		t.Fatal("conflict replaced original retained result", retry.Outcome)
	}
	debitAfter, resultAfter := readJournal(f.plan.AdmissionRoot, "admission.journal"), readJournal(f.plan.ResultRoot, "results.journal")
	defer clear(debitAfter)
	defer clear(resultAfter)
	if !bytes.Equal(debitBefore, debitAfter) || !bytes.Equal(resultBefore, resultAfter) {
		t.Fatal("valid conflict/retry changed durable histories")
	}
}

func networkTestBudget(t *testing.T) *hosting.Budget {
	t.Helper()
	root := filepath.Join(t.TempDir(), "budget")
	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(root, hosting.Policy{Provider: "local integration", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	return budget
}

func networkTestReservation(t *testing.T, budget *hosting.Budget, deadline time.Time) (func() error, error) {
	t.Helper()
	held, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, WorkUntil: deadline, HoldUntil: deadline.Add(time.Second)})
	if err != nil {
		return nil, err
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return held.Release(ctx)
	}, nil
}

func TestNetworkAdmissionRefillRetainsDeadlineAndHostingUntilReplacement(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(f.receiver.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal("public observation cannot bind future Node work", err)
	}
	_, tokens := f.tokens(t, 3)
	budget := networkTestBudget(t)
	owner, err := receiving.Open(t.TempDir(), f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	deadline := time.Now().Add(20 * time.Second)
	reserve := func() (func() error, error) { return networkTestReservation(t, budget, deadline) }
	first, err := owner.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, reserve)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	before, err := budget.Observe(t.Context())
	if err != nil || before.ReservedBytes == 0 {
		t.Fatal("initial reservation absent", err)
	}
	next, err := owner.Refill(t.Context(), first, 1, tokens[1], reserve)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Release()
	if !next.Allowance().Deadline().Equal(first.Allowance().Deadline()) || next.Allowance().Bytes() != first.Allowance().Bytes() {
		t.Fatal("refill changed the original lifetime or byte policy")
	}
	combined, err := budget.Observe(t.Context())
	if err != nil || combined.ReservedBytes != 2*before.ReservedBytes {
		t.Fatal("refill released predecessor capacity", combined, err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	f.successor(t)
	newView, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	if err := newView.MatchDuty(duty, newView.ObservedAt()); err == nil {
		t.Fatal("retained duty rebound to successor")
	}
	called := false
	if _, err := owner.Refill(t.Context(), next, 1, tokens[2], func() (func() error, error) { called = true; return reserve() }); err == nil || called {
		t.Fatal("successor refill reached reservation effects", err)
	}
	retained, err := budget.Observe(t.Context())
	if err != nil || retained.ReservedBytes != before.ReservedBytes {
		t.Fatal("refused refill released current work", retained, err)
	}
	if err := next.Release(); err != nil {
		t.Fatal(err)
	}
	final, err := budget.Observe(t.Context())
	if err != nil || final.ReservedBytes != 0 {
		t.Fatal("replacement leaked capacity", final, err)
	}
}

func TestNetworkClockAndProfileConflictRefuseBeforeReceivingEffects(t *testing.T) {
	for _, fault := range []string{"clock", "profile-conflict"} {
		t.Run(fault, func(t *testing.T) {
			f := newNetworkAdmissionFixture(t)
			_, tokens := f.tokens(t, 1)
			root := t.TempDir()
			owner, err := receiving.Open(root, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			if fault == "clock" {
				f.clockUnavailable.Store(true)
			} else if err := f.profileConflict(t); err == nil {
				t.Fatal("second signed profile was accepted")
			}
			called := false
			if _, err := owner.Accept(t.Context(), admission.ForwardClass, tokens[0], time.Now().Add(time.Second), func() (func() error, error) { called = true; return nil, nil }); err == nil || called {
				t.Fatal("unavailable Network reached physical effects", err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			f.clockUnavailable.Store(false)
			f.reopen()
			if fault == "profile-conflict" {
				if _, _, err := f.authority.observe(); err == nil {
					t.Fatal("profile conflict disappeared on recovery")
				}
				return
			}
			// Lost clock confidence refused before the burn. Recovery of genuine
			// time evidence permits this same previously unspent token.
			owner, err = receiving.Open(root, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			budget := networkTestBudget(t)
			deadline := time.Now().Add(time.Second)
			grant, err := owner.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) })
			if err != nil {
				t.Fatal("clock refusal consumed token", err)
			}
			if err := grant.Release(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNetworkAdmissionHostingSignedStateAndJoinedWork(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	// Recovery re-authenticates persisted bytes; no injected accepted values.
	f.reopen()
	if facts, _, err := f.authority.observe(); err != nil || facts != f.profile {
		t.Fatal("Network recovery changed authority", err)
	}
	if _, _, err := f.authority.issuer([32]byte{99}); err == nil {
		t.Fatal("foreign issuer signing key accepted")
	}
	_, tokens := f.tokens(t, 1)
	budget := networkTestBudget(t)
	deadline := time.Now().Add(4 * time.Second)
	root := t.TempDir()
	receiver, err := receiving.Open(root, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	grant, err := receiver.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) })
	if err != nil {
		t.Fatal(err)
	}
	defer grant.Release()
	// The real socket reader can stop but still await its owner's join. Closing
	// either authority owner must leave transferred Hosting capacity retained.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, stopped, join := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- networkTransferLocal(ctx, 64<<10, grant.Allowance().Deadline(), networkWorkAuthority(f.profile, time.Now(), deadline, f.authority.observe), func(reader io.Reader, n int64) error {
			close(started)
			<-ctx.Done()
			err := networkDiscardPayload(reader, n)
			close(stopped)
			<-join
			return err
		})
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatal("socket child did not start", err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-stopped
	view, observeErr := budget.Observe(t.Context())
	close(join)
	workErr := <-done
	if observeErr != nil || view.ReservedBytes == 0 {
		t.Fatal("capacity released before socket join", view, observeErr)
	}
	if !errors.Is(workErr, context.Canceled) {
		t.Fatal("socket cancellation lost", workErr)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("capacity retained after joined release", view, err)
	}
	f.reopen()
	reopened, err := receiving.Open(root, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Accept(t.Context(), admission.ForwardClass, tokens[0], time.Now().Add(time.Second), func() (func() error, error) { return networkTestReservation(t, budget, time.Now().Add(time.Second)) }); err == nil {
		t.Fatal("durable token burn revived after owners reopened")
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refused replay leaked capacity", view, err)
	}
}

func TestNetworkSuccessorDuringSpendRetainsBurnAndReleasesHosting(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	_, tokens := f.tokens(t, 1)
	budget := networkTestBudget(t)
	root := t.TempDir()
	changed := false
	observer := func() (receiving.Observation, error) {
		info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
		// This is an observable durable-I/O boundary, not an observer-call count.
		// Publish real independently signed successor bytes after the burn.
		if err == nil && info.Size() > 112 && !changed {
			changed = true
			f.successor(t)
		}
		return f.authority.receiver(f.receiver, f.profile.NotAfter)
	}
	receiver, err := receiving.Open(root, f.receiver, observer)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	if _, err := receiver.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) }); err == nil {
		t.Fatal("old duty authorized after Network successor")
	}
	if !changed {
		t.Fatal("scenario never reached the durable spend boundary")
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refused successor transition leaked capacity", view, err)
	}
	// Reopening the actual old-binding spend owner proves retained burn without
	// manufacturing old current authority to reopen Receiving.
	ledger, err := spending.Open(root, spending.Binding{NetworkID: f.receiver.NetworkID, ProfileDigest: f.receiver.ProfileDigest, ReceiverNodeID: f.receiver.NodeID, ReceiverDutyGeneration: f.receiver.DutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	if err := ledger.Spend(tokens[0], time.Now().UTC().Truncate(time.Hour), time.Now()); err == nil {
		t.Fatal("post-spend refusal refunded token on reopen")
	}
	f.reopen()
	if _, err := f.authority.receiver(f.receiver, f.profile.NotAfter); err == nil {
		t.Fatal("recovery revived superseded receiver binding")
	}
}

func TestNetworkLossBeforeSpendHasNoTokenDebit(t *testing.T) {
	f := newNetworkAdmissionFixture(t)
	_, tokens := f.tokens(t, 1)
	budget := networkTestBudget(t)
	root := t.TempDir()
	receiver, err := receiving.Open(root, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	if _, err := receiver.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) {
		release, err := networkTestReservation(t, budget, deadline)
		if err == nil {
			err = f.close()
		}
		return release, err
	}); err == nil {
		t.Fatal("closed Network authorized a spend")
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("pre-spend refusal leaked capacity", view, err)
	}
	f.reopen()
	reopened, err := receiving.Open(root, f.receiver, func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) })
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	deadline = time.Now().Add(time.Second)
	grant, err := reopened.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) { return networkTestReservation(t, budget, deadline) })
	if err != nil {
		t.Fatal("pre-spend refusal burned token", err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
}
