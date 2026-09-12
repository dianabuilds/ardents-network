//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

func textContextEndpoint(t *testing.T) (*endpoint, [32]byte) {
	t.Helper()
	principal := fixtureID(211)
	admission, err := broker.New(broker.Config{ID: fixtureID(212), Grants: []broker.Grant{
		{Principal: principal, Surface: broker.Connection},
		{Principal: principal, Surface: broker.Administration},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admission.Close)
	return &endpoint{admission: admission}, principal
}

func admittedTextContext(t *testing.T, endpoint *endpoint, principal [32]byte, surface broker.Surface) *textContext {
	t.Helper()
	capability, err := endpoint.Admit(principal, surface)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := endpoint.beginTextContext(context.Background(), capability, principal, surface)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	return owner
}

func TestTextContextRequiresExactOneUseLocalAuthority(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	for _, surface := range []broker.Surface{broker.Connection, broker.Administration} {
		capability, err := endpoint.Admit(principal, surface)
		if err != nil {
			t.Fatal(err)
		}
		owner, err := endpoint.beginTextContext(context.Background(), capability, principal, surface)
		if err != nil {
			t.Fatal(err)
		}
		defer owner.Close()
		if _, err := endpoint.beginTextContext(context.Background(), capability, principal, surface); err == nil {
			t.Fatal("replayed capability created a context")
		}
	}
	for _, foreign := range []struct {
		principal [32]byte
		surface   broker.Surface
	}{{fixtureID(213), broker.Connection}, {principal, broker.Administration}} {
		capability, err := endpoint.Admit(principal, broker.Connection)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := endpoint.beginTextContext(context.Background(), capability, foreign.principal, foreign.surface); err == nil {
			t.Fatal("foreign Principal or Publisher role acquired context authority")
		}
	}
}

func TestTextWorkerLossPreservesContextButNotInvocation(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	for _, surface := range []broker.Surface{broker.Connection, broker.Administration} {
		owner := admittedTextContext(t, endpoint, principal, surface)
		job, err := beginTextTestJob(t, owner, endpoint, surface)
		if err != nil {
			t.Fatal(err)
		}
		nonce := job.nonce
		if !owner.currentJob(endpoint, surface, job, nonce) {
			t.Fatal("current invocation was refused")
		}
		if _, err := owner.beginJob(endpoint, surface); err == nil {
			t.Fatal("concurrent replacement silently displaced a live worker")
		}
		owner.retireJob(job)
		if _, err := owner.beginJob(endpoint, surface); err == nil {
			t.Fatal("replacement began before joined cleanup")
		}
		if err := owner.finishJobCleanup(job, nil); err != nil {
			t.Fatal(err)
		}
		replacement, err := beginTextTestJob(t, owner, endpoint, surface)
		if err != nil {
			t.Fatalf("surviving Endpoint context was lost with its worker: %v", err)
		}
		if replacement.nonce == nonce || owner.currentJob(endpoint, surface, job, nonce) ||
			owner.currentJob(endpoint, surface, replacement, nonce) {
			t.Fatal("replacement inherited the retired worker invocation")
		}
		owner.retireJob(job)
		if !owner.currentJob(endpoint, surface, replacement, replacement.nonce) {
			t.Fatal("late retirement detached the replacement")
		}
		other := admittedTextContext(t, endpoint, principal, surface)
		if other.currentJob(endpoint, surface, replacement, replacement.nonce) {
			t.Fatal("separate context acquired a foreign invocation")
		}
	}
}

func TestTextContextCleanupFailureCannotRestoreAuthority(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.finishJobCleanup(job, nil); err == nil {
		t.Fatal("cleanup completed before retirement")
	}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, errors.New("cgroup remains populated")); err == nil {
		t.Fatal("failed cleanup was reported as success")
	}
	if _, err := owner.beginJob(endpoint, broker.Connection); err == nil {
		t.Fatal("failed cleanup allowed a replacement")
	}
	if err := owner.finishJobCleanup(job, nil); err == nil {
		t.Fatal("late cleanup erased the first failure")
	}
	if _, err := owner.beginJob(endpoint, broker.Connection); err == nil {
		t.Fatal("late cleanup resurrected closed authority")
	}
}

func TestTextContextRejectsRevokedLostAndForeignOwners(t *testing.T) {
	for _, stop := range []string{"revoke", "endpoint loss", "context close"} {
		t.Run(stop, func(t *testing.T) {
			endpoint, principal := textContextEndpoint(t)
			owner := admittedTextContext(t, endpoint, principal, broker.Connection)
			job, err := beginTextTestJob(t, owner, endpoint, broker.Connection)
			if err != nil {
				t.Fatal(err)
			}
			nonce := job.nonce
			foreign, _ := textContextEndpoint(t)
			if owner.currentJob(foreign, broker.Connection, job, nonce) ||
				owner.currentJob(endpoint, broker.Administration, job, nonce) {
				t.Fatal("foreign Endpoint or role acquired a worker")
			}
			switch stop {
			case "revoke":
				if err := endpoint.admission.Revoke(principal, broker.Connection); err != nil {
					t.Fatal(err)
				}
			case "endpoint loss":
				endpoint.admission.Close()
			case "context close":
				go func() { _ = owner.Close() }()
				<-job.context.Done()
			}
			if owner.currentJob(endpoint, broker.Connection, job, nonce) {
				t.Fatal("late completion retained lost local authority")
			}
			if _, err := owner.beginJob(endpoint, broker.Connection); err == nil {
				t.Fatal("lost owner accepted a replacement worker")
			}
		})
	}
}

// The pure local-owner tests have no launched process. They must still finish
// their reservation before the enclosing context can report joined shutdown.
func beginTextTestJob(t *testing.T, owner *textContext, endpoint *endpoint, surface broker.Surface) (*textJobIdentity, error) {
	t.Helper()
	job, err := owner.beginJob(endpoint, surface)
	if err == nil {
		t.Cleanup(func() { owner.retireJob(job); _ = owner.finishJobCleanup(job, nil) })
	}
	return job, err
}
