//go:build linux

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func TestCompletedTextWorkerResultCannotCrossReplacement(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	worker := &qualifiedTextWorker{job: job}
	if worker.completedCurrent() {
		t.Fatal("unjoined job supplied a result")
	}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	if !worker.completedCurrent() {
		t.Fatal("joined current result refused")
	}
	replacement, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	if worker.completedCurrent() {
		t.Fatal("old result entered a live replacement")
	}
	owner.retireJob(replacement)
	if err := owner.finishJobCleanup(replacement, nil); err != nil {
		t.Fatal(err)
	}
	if worker.completedCurrent() {
		t.Fatal("old result resurrected after replacement ended")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if (&qualifiedTextWorker{job: replacement}).completedCurrent() {
		t.Fatal("result survived context revoke")
	}
}

func TestCancelledTextJobClosesLateGrantWithoutCrossingReplacement(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	claimed := job.claimWorkerLocked(owner)
	owner.mu.Unlock()
	if !claimed {
		t.Fatal("current job did not claim its worker handoff")
	}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	replacement, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	late, err := broker.New(broker.Config{ID: fixtureID(197), Grants: []broker.Grant{{Principal: principal, Surface: broker.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(late.Close)
	capability, err := late.Admit(principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	lease, _, err := late.Activate(t.Context(), capability, principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Release)
	owner.mu.Lock()
	accepted := job.handoffGrantLocked(owner, late, lease)
	replacementGrant := replacement.workerGrant
	owner.mu.Unlock()
	if accepted || replacementGrant != nil {
		t.Fatal("late Grant crossed into replacement job")
	}
	if late.Active() != 0 || lease.Context().Err() == nil {
		t.Fatal("rejected late Grant retained its active session")
	}
	if _, err := late.Admit(principal, broker.Connection); err == nil {
		t.Fatal("rejected late Grant remained open")
	}
}

func TestTextWorkerOperationCannotReserveTwiceOrAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	lifetime := &textWorkerLifetime{context: ctx}
	finish, err := lifetime.beginUse()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lifetime.beginUse(); err == nil {
		t.Fatal("worker operation was consumed twice")
	}
	finish()
	cancel()
	if _, err := lifetime.beginUse(); err == nil {
		t.Fatal("finished operation was reused")
	}
	lifetime = &textWorkerLifetime{context: ctx}
	if _, err := lifetime.beginUse(); err == nil {
		t.Fatal("cancelled lifetime admitted work")
	}
}

func TestAlreadyCancelledTextLaunchHasNoEffects(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	release, err := endpoint.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if worker, err := owner.launchTextWorker(ctx, nil); err == nil || worker != nil {
		t.Fatal("cancelled launch accepted")
	}
	owner.mu.Lock()
	pending := owner.job != nil
	owner.mu.Unlock()
	if pending {
		t.Fatal("cancelled launch retained a job")
	}
	if !endpoint.textAvailable() {
		t.Fatal("no-effect cancellation terminalized Endpoint")
	}
}

func TestTextLaunchCancellationReleasesWaitingReservation(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	release, err := endpoint.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := owner.launchTextWorker(ctx, nil); finished <- err }()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		owner.mu.Lock()
		reserved := owner.job != nil
		owner.mu.Unlock()
		if reserved {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("launch did not reserve its waiting job")
		case <-ticker.C:
		}
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled waiting launch accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("waiting launch did not join cancellation")
	}
	owner.mu.Lock()
	pending := owner.job != nil
	owner.mu.Unlock()
	if pending || !endpoint.textAvailable() {
		t.Fatal("no-effect waiting cancellation retained pressure or terminalized Endpoint")
	}
}
