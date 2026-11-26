//go:build linux

package node_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

type hostingPlan struct {
	Root   string         `json:"root"`
	Policy hosting.Policy `json:"policy"`
}

func hostingInitializePlan(t *testing.T) hostingPlan {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return hostingPlan{Root: filepath.Join(t.TempDir(), "budget"), Policy: hosting.Policy{Provider: "private-provider", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "B", Quantity: 1000000000, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}}
}

func admissionObserver(path string) func() (admission.AuthorityFacts, time.Time, error) {
	return func() (admission.AuthorityFacts, time.Time, error) {
		var facts admission.AuthorityFacts
		raw, err := os.ReadFile(path)
		if err == nil {
			err = json.Unmarshal(raw, &facts)
		}
		now := time.Now().UTC()
		if err == nil {
			err = facts.ValidateAt(now)
		}
		return facts, now, err
	}
}

func TestAdmissionHostingReservationAndWork(t *testing.T) {
	runAdmissionCycle(t, func(t *testing.T, facts admission.AuthorityFacts, receiver receiving.Receiver, profile string, first, second, third []byte) {
		checkAdmissionHostingReservation(t, facts, receiver, first)
		checkHostingTransfer(t, facts, receiver, profile, second)
		checkCanceledAdmittedWork(t, facts, receiver, third)
	})
}

func checkAdmissionHostingReservation(t *testing.T, p admission.AuthorityFacts, receiver receiving.Receiver, raw []byte) {
	t.Helper()
	plan := hostingInitializePlan(t)
	if err := hosting.Initialize(plan.Root, plan.Policy); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	current := true
	owner, err := receiving.Open(t.TempDir(), receiver, func() (receiving.Observation, error) {
		if !current {
			return receiving.Observation{}, errors.New("authority withdrawn")
		}
		return receiving.Observation{Profile: p, Receiver: receiver, Now: time.Now().UTC(), NotAfter: p.NotAfter}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	tooLarge := true
	calls := 0
	reserve := func() (func() error, error) {
		calls++
		amount := uint64(100)
		if tooLarge {
			amount = plan.Policy.Quantity + 1
		}
		r, err := budget.Reserve(t.Context(), hosting.ReservationRequest{Work: hosting.Traffic{Tx: amount}, Termination: hosting.Traffic{Rx: 20}, WorkUntil: p.NotAfter, HoldUntil: p.NotAfter})
		if err != nil {
			return nil, err
		}
		return func() error { return r.Release(t.Context()) }, nil
	}
	current = false
	if _, err := owner.Accept(t.Context(), admission.ForwardClass, raw, p.NotAfter, reserve); err == nil || calls != 0 {
		t.Fatal("unavailable authority reached capacity")
	}
	current = true
	if _, err := owner.Accept(t.Context(), admission.ForwardClass, raw, p.NotAfter, reserve); err == nil {
		t.Fatal("exhausted budget accepted")
	}
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("refusal left reserve", view, err)
	}
	tooLarge = false
	grant, err := owner.Accept(t.Context(), admission.ForwardClass, raw, p.NotAfter, reserve)
	if err != nil {
		t.Fatal("capacity refusal burned token", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 120 {
		t.Fatal("receiver close released admitted work", view, err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	if err := grant.Release(); err != nil {
		t.Fatal(err)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("grant release failed", view, err)
	}
}

func checkHostingTransfer(t *testing.T, p admission.AuthorityFacts, receiver receiving.Receiver, profile string, raw []byte) {
	t.Helper()
	plan := hostingInitializePlan(t)
	if err := hosting.Initialize(plan.Root, plan.Policy); err != nil {
		t.Fatal(err)
	}
	config := hostingWorkFixture{Root: t.TempDir(), Observe: admissionObserver(profile), Budget: plan.Root, Receiver: receiver, NotAfter: p.NotAfter, Deadline: time.Now().Add(4 * time.Second), Token: raw, Bytes: 64 << 10}
	if err := runHostingFixture(t.Context(), config); err != nil {
		t.Fatal("actual work failed", err)
	}
	budget, err := hosting.Open(plan.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	view, err := budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 || view.UsedBytes < config.Bytes {
		t.Fatal("work did not charge/join/release", view, err)
	}
	// Reopening with the same token must refuse, retaining no new reserve.
	config.Deadline = time.Now().Add(4 * time.Second)
	if err := runHostingFixture(t.Context(), config); err == nil {
		t.Fatal("replayed work token")
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal(view, err)
	}
}

func checkCanceledAdmittedWork(t *testing.T, facts admission.AuthorityFacts, recipient receiving.Receiver, raw []byte) {
	t.Helper()
	plan := hostingInitializePlan(t)
	if err := hosting.Initialize(plan.Root, plan.Policy); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(plan.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer budget.Close()
	root := t.TempDir()
	reached := make(chan struct{})
	resume := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	blocked := false
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		// The durable spend is a semantic boundary, independent of observer call counts.
		info, err := os.Stat(filepath.Join(root, "closed-token-spends"))
		if !blocked && err == nil && info.Size() > 112 {
			blocked = true
			close(reached)
			<-resume
		}
		return facts, time.Now().UTC(), nil
	}
	work := hostingWorkFixture{Root: root, Budget: plan.Root, Receiver: recipient, NotAfter: facts.NotAfter, Deadline: time.Now().Add(4 * time.Second), Token: raw, Bytes: 64 << 10, Observe: observe}
	done := make(chan error, 1)
	go func() { done <- runHostingFixture(ctx, work) }()
	select {
	case <-reached:
	case err := <-done:
		t.Fatal("work failed before spend", err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	cancel()
	view, err := budget.Observe(t.Context())
	close(resume)
	outcome := <-done
	if err != nil || view.ReservedBytes == 0 {
		t.Fatal("cancellation released still-running work", view, err)
	}
	if !errors.Is(outcome, context.Canceled) {
		t.Fatal("lost cancellation", outcome)
	}
	view, err = budget.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("joined refusal retained reservation", view, err)
	}
	work.Observe = func() (admission.AuthorityFacts, time.Time, error) { return facts, time.Now().UTC(), nil }
	work.Deadline = time.Now().Add(4 * time.Second)
	if err := runHostingFixture(t.Context(), work); err == nil {
		t.Fatal("canceled spent token became reusable")
	}
}
