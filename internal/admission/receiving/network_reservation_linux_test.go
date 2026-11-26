package receiving_test

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	epochfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNetworkLossBeforeSpendHasNoTokenDebit(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t)
	receiverBinding, tokens, _ := issueFixtureTokens(t, fixture, admission.ForwardClass, 1)
	tokenBytes := tokens[0]
	budget := openReceivingBudget(t)
	observe := func() (receiving.Observation, error) { return observeReceiver(fixture, receiverBinding) }
	root := t.TempDir()
	receiver, err := receiving.Open(root, receiverBinding, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	closedDuringReservation := false
	deadline := time.Now().Add(5 * time.Second)
	if _, err := receiver.Accept(t.Context(), admission.ForwardClass, tokenBytes, deadline, func() (func() error, error) {
		release, err := reserveForwardWork(t, budget, deadline)
		if err == nil {
			err = fixture.State.Close()
			closedDuringReservation = err == nil
		}
		return release, err
	}); err == nil || !closedDuringReservation {
		t.Fatalf("pre-spend refusal = %v, Network closed during reservation = %v", err, closedDuringReservation)
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("pre-spend refusal leaked capacity", view, err)
	}
	recovered, err := state.OpenRetained(fixture.Config)
	if err != nil {
		t.Fatal(err)
	}
	fixture.State = recovered
	reopened, err := receiving.Open(root, receiverBinding, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	deadline = time.Now().Add(5 * time.Second)
	grant, err := reopened.Accept(t.Context(), admission.ForwardClass, tokenBytes, deadline, func() (func() error, error) { return reserveForwardWork(t, budget, deadline) })
	if err != nil {
		t.Fatal("pre-spend refusal burned token", err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkSuccessorDuringSpendRetainsBurnAndReleasesHosting(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t)
	binding, tokens, tokenWindow := issueFixtureTokens(t, fixture, admission.ForwardClass, 1)
	tokenBytes := tokens[0]
	budget := openReceivingBudget(t)
	root := t.TempDir()
	journal := filepath.Join(root, "closed-token-spends")
	var originalSize int64
	var transitioned bool
	var burnObservedAt time.Time

	successor := prepareReceivingSuccessor(t, fixture, binding)
	observe := func() (receiving.Observation, error) {
		if originalSize > 0 && !transitioned {
			info, err := os.Stat(journal)
			if err != nil {
				return receiving.Observation{}, err
			}
			if info.Size() > originalSize {
				burnObservedAt = time.Now().UTC()
				if err := acceptReceivingSuccessor(t.Context(), fixture.State, successor); err != nil {
					return receiving.Observation{}, err
				}
				transitioned = true
			}
		}
		return observeReceiver(fixture, binding)
	}
	receiver, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	info, err := os.Stat(journal)
	if err != nil || info.Size() <= 0 {
		t.Fatalf("initial spend history unavailable: %v", err)
	}
	originalSize = info.Size()
	deadline := time.Now().Add(5 * time.Second)
	if _, err := receiver.Accept(t.Context(), admission.ForwardClass, tokenBytes, deadline, func() (func() error, error) {
		return reserveForwardWork(t, budget, deadline)
	}); err == nil || !transitioned {
		t.Fatalf("post-spend refusal = %v, successor accepted after append = %v", err, transitioned)
	}
	if err := receiver.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refused successor transition leaked capacity", view, err)
	}

	ledger, err := spending.Open(root, spending.Binding{
		NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest,
		ReceiverNodeID: binding.NodeID, ReceiverDutyGeneration: binding.DutyGeneration,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	if err := ledger.Spend(tokenBytes, tokenWindow, burnObservedAt); err == nil || err.Error() != "closed token is already spent" {
		t.Fatalf("reopened spend history did not retain burn: %v", err)
	}
	if err := fixture.State.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := state.OpenRetained(fixture.Config)
	if err != nil {
		t.Fatal(err)
	}
	fixture.State = recovered
	current, err := recovered.CurrentRuntime()
	if err != nil || current.Profile().EpochDigest != successor.Epoch.Digest {
		t.Fatalf("retained State did not recover signed successor: %v", err)
	}
	if _, err := observeReceiver(fixture, binding); err == nil {
		t.Fatal("recovery revived superseded receiver binding")
	}
}

func TestNetworkAdmissionRefillRetainsDeadlineAndHostingUntilReplacement(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t)
	binding, tokens, _ := issueFixtureTokens(t, fixture, admission.ForwardClass, 3)
	successor := prepareReceivingSuccessor(t, fixture, binding)
	view, err := fixture.State.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(binding.NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal("public observation cannot bind future Node work", err)
	}
	budget := openReceivingBudget(t)
	owner, err := receiving.Open(t.TempDir(), binding, func() (receiving.Observation, error) { return observeReceiver(fixture, binding) })
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	deadline := time.Now().Add(20 * time.Second)
	reserve := func() (func() error, error) { return reserveForwardWork(t, budget, deadline) }
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
	if err := acceptReceivingSuccessor(t.Context(), fixture.State, successor); err != nil {
		t.Fatal(err)
	}
	newView, err := fixture.State.CurrentRuntime()
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
			fixture := networkfixture.OpenSignedAdmission(t)
			binding, tokens, _ := issueFixtureTokens(t, fixture, admission.ForwardClass, 1)
			root := t.TempDir()
			observe := func() (receiving.Observation, error) { return observeReceiver(fixture, binding) }
			owner, err := receiving.Open(root, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			if _, err := observe(); err != nil {
				t.Fatal("original receiver authority unavailable", err)
			}
			clock := fixture.Config.ClockObservationFile
			switch fault {
			case "clock":
				fixture.StopClock()
				if err := os.Rename(clock, clock+".held"); err != nil {
					t.Fatal(err)
				}
			case "profile-conflict":
				conflicting := fixture.Spec
				conflicting.IssuanceAuthority[0] ^= 1
				bundle, err := epochfixture.BuildClosed(conflicting)
				if err != nil {
					t.Fatal(err)
				}
				if bundle.Epoch.Digest != binding.StateDigest {
					t.Fatal("profile fault changed the signed Epoch")
				}
				if _, err := fixture.State.AcceptClosedProfile(bundle.Profile); err == nil || err.Error() != "closed profile has a durable conflict" {
					t.Fatalf("same-Epoch conflicting profile result: %v", err)
				}
			}
			if _, err := observe(); err == nil {
				t.Fatal("fault left fresh receiver authority available")
			}
			called := false
			deadline := time.Now().Add(5 * time.Second)
			if _, err := owner.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) {
				called = true
				return nil, nil
			}); err == nil || called {
				t.Fatalf("refusal = %v, reservation invoked = %v", err, called)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if err := fixture.State.Close(); err != nil {
				t.Fatal(err)
			}
			if fault == "clock" {
				if err := os.Rename(clock+".held", clock); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(networkfixture.RefreshObservedClock(t, clock))
			}
			recovered, err := state.OpenRetained(fixture.Config)
			if err != nil {
				t.Fatal(err)
			}
			fixture.State = recovered
			if fault == "profile-conflict" {
				if _, err := observe(); err == nil {
					t.Fatal("profile conflict disappeared on recovery")
				}
				return
			}
			reopened, err := receiving.Open(root, binding, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			budget := openReceivingBudget(t)
			deadline = time.Now().Add(5 * time.Second)
			grant, err := reopened.Accept(t.Context(), admission.ForwardClass, tokens[0], deadline, func() (func() error, error) {
				return reserveForwardWork(t, budget, deadline)
			})
			if err != nil {
				t.Fatal("same token refused after clock recovery", err)
			}
			if err := grant.Release(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func prepareReceivingSuccessor(t *testing.T, fixture *networkfixture.SignedAdmission, binding receiving.Receiver) epochfixture.Closed {
	t.Helper()
	next := fixture.Spec
	next.Number++
	next.Previous = binding.StateDigest
	next.Seed[0]++
	successor, err := epochfixture.BuildClosed(next)
	if err != nil {
		t.Fatal(err)
	}
	return successor
}

func acceptReceivingSuccessor(ctx context.Context, owner *state.State, successor epochfixture.Closed) error {
	if _, err := owner.AcceptEpoch(ctx, successor.Epoch.Raw, successor.Epoch.Inputs, successor.Epoch.Materials); err != nil {
		return err
	}
	_, err := owner.AcceptClosedProfile(successor.Profile)
	return err
}

func openReceivingBudget(t *testing.T) *hosting.Budget {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosting")
	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(path, hosting.Policy{
		Provider: "Receiving reservation fixture", Start: start, End: start.Add(2 * time.Hour),
		Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(path)
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

func observeReceiver(fixture *networkfixture.SignedAdmission, binding receiving.Receiver) (receiving.Observation, error) {
	view, err := fixture.State.CurrentRuntime()
	if err != nil {
		return receiving.Observation{}, err
	}
	facts, now, err := networkfixture.AdmissionFacts(view)
	if err != nil {
		return receiving.Observation{}, err
	}
	return receiving.ObserveCurrentAuthority(view, facts, now, binding, facts.NotAfter)
}

func reserveForwardWork(t *testing.T, budget *hosting.Budget, deadline time.Time) (func() error, error) {
	t.Helper()
	held, err := budget.Reserve(t.Context(), hosting.ReservationRequest{
		Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
		WorkUntil: deadline, HoldUntil: deadline.Add(time.Second),
	})
	if err != nil {
		return nil, err
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return held.Release(ctx)
	}, nil
}

func issueFixtureTokens(t *testing.T, fixture *networkfixture.SignedAdmission, class admission.Class, count uint32) (receiving.Receiver, [][]byte, time.Time) {
	t.Helper()
	facts, now, err := fixture.ObserveIssuer()
	if err != nil {
		t.Fatal(err)
	}
	member := fixture.Spec.Nodes[1]
	receiver := receiving.Receiver{
		NetworkID: facts.NetworkID, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest,
		ProfileDigest: facts.Digest, NodeID: member.NodeID, DutyGeneration: member.Generation,
	}
	hour := now.UTC().Truncate(time.Hour)
	role := admission.AllocationUser
	if class == admission.RegistrationClass {
		role = admission.AllocationPublisher
	}
	var maxima [3]uint32
	maxima[class-1] = count
	request, holderKey, err := admission.PreparePermissionRequest(facts.IssuanceAuthorityKey, facts.NetworkID,
		facts.IssuerNodeID, facts.IssuerDutyGeneration, role, hour, maxima)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holderKey)
	raw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	prepared, err := allocation.Prepare(raw, facts.NetworkID, now)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, facts.IssuanceAuthorityKey, now)
	if err != nil {
		t.Fatal(err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(fixture.Spec.Authority, admission.PermissionTranscript(permission)))
	challenge := token.ClosedTokenContext{
		NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration,
		Class: uint8(class), WindowStart: hour,
	}
	contexts := make([]token.ClosedTokenContext, count)
	for index := range contexts {
		contexts[index] = challenge
	}
	batch, err := token.PrepareClosedTokenBatch(token.ClosedTokenBatchConfig{
		Profile: facts, Contexts: contexts, Permission: permission, HolderKey: holderKey, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Discard()
	batchRaw := batch.Request()
	defer clear(batchRaw)
	result := issuer.IssueCurrent(t.Context(), fixture.Plan, batchRaw, quota.Bootstrap, fixture.ObserveIssuer)
	defer clear(result.Response)
	if result.Outcome != "issued-offline" {
		t.Fatalf("fixture issue: %s %s", result.Phase, result.Outcome)
	}
	tokens, err := batch.FinalizeEncoded(result.Response)
	if err != nil || len(tokens) != int(count) {
		t.Fatalf("verify fixture issuance: tokens=%d error=%v", len(tokens), err)
	}
	t.Cleanup(func() {
		for _, raw := range tokens {
			clear(raw)
		}
	})
	return receiver, tokens, hour
}
