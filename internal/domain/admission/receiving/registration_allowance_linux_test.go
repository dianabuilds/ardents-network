package receiving_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	epochfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	"github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestRegistrationSelectedAllowanceWithSignedNetworkAndDurableSpend(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t, func(spec *epochfixture.ClosedSpec) {
		spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = 4, 3
	})
	binding, tokens, _ := issueFixtureTokens(t, fixture, admission.RegistrationClass, 1)
	budget := openReceivingBudget(t)
	root := t.TempDir()
	observe := func() (receiving.Observation, error) { return observeReceiver(fixture, binding) }
	owner, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	deadline := time.Now().Add(20 * time.Second)
	var returns atomic.Int32
	reserve := func() (func() error, error) {
		release, err := reserveForwardWork(t, budget, deadline)
		if err != nil {
			return nil, err
		}
		return func() error { returns.Add(1); return release() }, err
	}
	grant, err := owner.Accept(t.Context(), admission.RegistrationClass, tokens[0], deadline, reserve)
	if err != nil {
		t.Fatal(err)
	}
	defer grant.Release()
	// Independent selected contract: private-admission.md class 3, not ByteLimit.
	if got := grant.Allowance().Bytes(); got != 1048576 {
		t.Errorf("selected Registration allowance = 1048576 bytes; receiving returned %d", got)
	}
	if !grant.Allowance().Deadline().Equal(deadline) {
		t.Fatal("Registration extended its earlier caller bound")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes == 0 || returns.Load() != 0 {
		t.Fatal("receiving closure returned work-owned capacity", view, err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	if err := grant.Release(); err != nil || returns.Load() != 1 {
		t.Fatal("Registration reservation was not returned exactly once", err)
	}
	reopened, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Accept(t.Context(), admission.RegistrationClass, tokens[0], deadline, reserve); err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
		t.Fatalf("reopened Registration spend did not report replay: %v", err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refused duplicate leaked capacity", view, err)
	}
}

func TestRegistrationExpiryAfterSpendRetainsBurnAndReturnsHosting(t *testing.T) {
	fixture := networkfixture.OpenSignedAdmission(t, func(spec *epochfixture.ClosedSpec) {
		spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = 4, 3
	})
	binding, tokens, _ := issueFixtureTokens(t, fixture, admission.RegistrationClass, 1)
	budget := openReceivingBudget(t)
	root := t.TempDir()
	deadline := time.Now().Add(time.Second)
	journal := filepath.Join(root, "closed-token-spends")
	var originalSize int64
	spent := false
	observe := func() (receiving.Observation, error) {
		if originalSize > 0 && !spent {
			info, err := os.Stat(journal)
			if err != nil {
				return receiving.Observation{}, err
			}
			if info.Size() <= originalSize {
				return observeReceiver(fixture, binding)
			}
			spent = true
			// Cross the real caller deadline only after durable I/O; authority
			// remains the genuine signed Network observation, without a fake clock.
			timer := time.NewTimer(time.Until(deadline))
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-t.Context().Done():
				t.Fatal(t.Context().Err())
			}
		}
		return observeReceiver(fixture, binding)
	}
	owner, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	initial, err := os.Stat(journal)
	if err != nil || initial.Size() <= 0 {
		t.Fatalf("initial Registration spend history unavailable: %v", err)
	}
	originalSize = initial.Size()

	if _, err := owner.Accept(t.Context(), admission.RegistrationClass, tokens[0], deadline, func() (func() error, error) {
		return reserveForwardWork(t, budget, deadline)
	}); err == nil || !spent {
		t.Fatal("Registration did not refuse expiry after its durable spend", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("post-spend expiry leaked Hosting capacity", view, err)
	}
	reopened, err := receiving.Open(root, binding, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	retryDeadline := time.Now().Add(20 * time.Second)
	if _, err := reopened.Accept(t.Context(), admission.RegistrationClass, tokens[0], retryDeadline, func() (func() error, error) {
		return reserveForwardWork(t, budget, retryDeadline)
	}); err == nil || !strings.Contains(err.Error(), "closed token is already spent") {
		t.Fatalf("reopened Registration spend after expiry did not report replay: %v", err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("reopened duplicate leaked Hosting capacity", view, err)
	}
}
