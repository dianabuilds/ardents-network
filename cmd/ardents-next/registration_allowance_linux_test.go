//go:build linux

package main

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
)

func TestRegistrationSelectedAllowanceWithSignedNetworkAndDurableSpend(t *testing.T) {
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) {
		f.spec.Nodes[1].RoleDomain, f.spec.Nodes[1].Subrole = 4, 3
	})
	_, tokens := f.tokensForClass(t, admission.RegistrationClass, 1)
	budget := networkTestBudget(t)
	root := t.TempDir()
	observe := func() (receiving.Observation, error) { return f.authority.receiver(f.receiver, f.profile.NotAfter) }
	owner, err := receiving.Open(root, f.receiver, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	deadline := time.Now().Add(20 * time.Second)
	var returns atomic.Int32
	reserve := func() (func() error, error) {
		release, err := networkTestReservation(t, budget, deadline)
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
	reopened, err := receiving.Open(root, f.receiver, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Accept(t.Context(), admission.RegistrationClass, tokens[0], deadline, reserve); err == nil {
		t.Fatal("reopening refunded a spent Registration token")
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refused duplicate leaked capacity", view, err)
	}
}

func TestRegistrationExpiryAfterSpendRetainsBurnAndReturnsHosting(t *testing.T) {
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) {
		f.spec.Nodes[1].RoleDomain, f.spec.Nodes[1].Subrole = 4, 3
	})
	_, tokens := f.tokensForClass(t, admission.RegistrationClass, 1)
	budget := networkTestBudget(t)
	root := t.TempDir()
	deadline := time.Now().Add(time.Second)
	spent := false
	observe := func() (receiving.Observation, error) {
		info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
		if err == nil && info.Size() > 112 && !spent {
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
		return f.authority.receiver(f.receiver, f.profile.NotAfter)
	}
	owner, err := receiving.Open(root, f.receiver, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Accept(t.Context(), admission.RegistrationClass, tokens[0], deadline, func() (func() error, error) {
		return networkTestReservation(t, budget, deadline)
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
	reopened, err := receiving.Open(root, f.receiver, observe)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Accept(t.Context(), admission.RegistrationClass, tokens[0], time.Now().Add(20*time.Second), func() (func() error, error) {
		return networkTestReservation(t, budget, time.Now().Add(20*time.Second))
	}); err == nil {
		t.Fatal("post-spend expiry refunded Registration token on reopen")
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("reopened duplicate leaked Hosting capacity", view, err)
	}
}
