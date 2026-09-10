//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func TestTextContextCloseJoinsPendingLaunchAndRetainsFailure(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	nonce := job.nonce
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	select {
	case <-job.context.Done():
	case <-time.After(time.Second):
		t.Fatal("context close did not cancel pending launch")
	}
	if owner.currentJob(endpoint, broker.Connection, job, nonce) {
		t.Fatal("closing context still admits completions")
	}
	select {
	case <-closed:
		t.Fatal("Close returned before pending launch cleanup")
	default:
	}
	failure := errors.New("original cgroup could not be joined")
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, failure); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if !errors.Is(err, failure) {
			t.Fatalf("Close lost cleanup failure: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not join completed cleanup")
	}
	if err := owner.Close(); !errors.Is(err, failure) {
		t.Fatalf("repeated Close lost failure: %v", err)
	}
	if err := endpoint.Close(); !errors.Is(err, failure) {
		t.Fatalf("Endpoint lost already closed context's failure: %v", err)
	}
}

func TestTextEndpointCloseCancelsAllBeforeJoiningAnyWorker(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	first := admittedTextContext(t, endpoint, principal, broker.Connection)
	second := admittedTextContext(t, endpoint, principal, broker.Administration)
	firstJob, err := beginTextTestJob(t, first, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	secondJob, err := beginTextTestJob(t, second, endpoint, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- endpoint.Close() }()
	for _, job := range []*textJobIdentity{firstJob, secondJob} {
		select {
		case <-job.context.Done():
		case <-time.After(time.Second):
			t.Fatal("Endpoint waited for one cleanup before revoking another worker")
		}
	}
	first.retireJob(firstJob)
	if err := first.finishJobCleanup(firstJob, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
		t.Fatal("Endpoint closed with a second cleanup still pending")
	default:
	}
	second.retireJob(secondJob)
	if err := second.finishJobCleanup(secondJob, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Endpoint did not join worker cleanup")
	}
}

func TestTextContextPendingCleanupKeepsFiniteAdmissionPressure(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	for index := 0; index < 6; index++ {
		parent, cancel := context.WithCancel(context.Background())
		capability, err := endpoint.Admit(principal, broker.Administration)
		if err != nil {
			t.Fatal(err)
		}
		owner, err := endpoint.beginTextContext(parent, capability, principal, broker.Administration)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = owner.Close() })
		job, err := beginTextTestJob(t, owner, endpoint, broker.Administration)
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		owner.lease.Release()
		<-job.context.Done()
	}
	capability, err := endpoint.Admit(principal, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := endpoint.beginTextContext(context.Background(), capability, principal, broker.Administration)
	if err == nil {
		_ = owner.Close()
		t.Fatal("cancelled Broker leases bypassed pending cleanup capacity")
	}
	if endpoint.admission.Active() != 0 {
		t.Fatal("refused context leaked its Broker lease")
	}
}

func TestTextContextCleanupFailureClosesExistingAndFutureJobAdmission(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	failed := admittedTextContext(t, endpoint, principal, broker.Administration)
	idle := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, failed, endpoint, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("worker cgroup remains populated")
	failed.retireJob(job)
	if err := failed.finishJobCleanup(job, failure); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	// Check synchronously before the context shutdown watcher can be relied on:
	// losing the prior worker reservation must never open an admission window.
	if replacement, err := idle.beginJob(endpoint, broker.Connection); err == nil {
		idle.retireJob(replacement)
		_ = idle.finishJobCleanup(replacement, nil)
		t.Fatal("previously idle context admitted work after cleanup failure")
	}
	if err := failed.Close(); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	for index := 0; index < 8; index++ {
		capability, err := endpoint.Admit(principal, broker.Administration)
		if err != nil {
			t.Fatal(err)
		}
		replacement, err := endpoint.beginTextContext(context.Background(), capability, principal, broker.Administration)
		if err == nil {
			_ = replacement.Close()
			t.Fatal("fresh context reused local authority after unjoined cgroup cleanup")
		}
	}
	if err := endpoint.Close(); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
