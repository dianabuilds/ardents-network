//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// This selected installed profile supplies only local caller authorization as
// a fixture. Platform, service identity, artifacts, worker activation, readiness,
// scoped Grant and cgroup cleanup all cross the actual production boundary.
// It is not a protected Service journey or the complete hostile-worker matrix.
func TestInstalledTextWorkerLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 110*time.Second)
	defer cancel()
	if err := verifyTextEndpointService(ctx); err != nil {
		t.Fatalf("invalid installed environment: %v", err)
	}
	endpoint, principal := textContextEndpoint(t)
	for _, role := range []struct {
		name     string
		surface  broker.Surface
		snapshot []byte
	}{
		{"reader", broker.Connection, nil},
		{"publisher", broker.Administration, []byte("one immutable installed snapshot\n")},
	} {
		t.Run(role.name, func(t *testing.T) {
			owner := admittedTextContext(t, endpoint, principal, role.surface)
			var previous *qualifiedTextWorker
			var previousNonce [32]byte
			for attempt := 0; attempt < 12; attempt++ {
				worker, err := owner.launchTextWorker(ctx, role.snapshot)
				if err != nil {
					t.Fatalf("attempt %d launch: %v", attempt, err)
				}
				t.Cleanup(func() {
					if err := worker.Close(); err != nil {
						t.Error(err)
					}
				})
				if worker.grant.Active() != 1 || worker.lease.Context().Err() != nil {
					t.Fatal("verified readiness did not acquire a live scoped Grant")
				}
				owner.mu.Lock()
				currentNonce := worker.job.nonce
				owner.mu.Unlock()
				if currentNonce == [32]byte{} {
					t.Fatal("live job nonce is absent")
				}
				if previous != nil && (previous.job == worker.job ||
					previousNonce == currentNonce || previous.completedCurrent()) {
					t.Fatal("replacement inherited a prior job or completion")
				}
				instance := installedTextWorkerInstance(t, ctx, worker, role.name)
				if attempt == 0 {
					requireInstalledTextWorkerCollectionPolicy(t, ctx, instance)
				}
				events, err := pinTextWorkerCgroup(instance)
				if err != nil {
					t.Fatal(err)
				}
				gone, populated, err := readTextWorkerCgroup(events)
				if err != nil || gone || !populated {
					_ = events.Close()
					t.Fatalf("missing live cgroup positive control: %v", err)
				}
				if attempt == 11 {
					err = owner.Close()
				} else {
					err = worker.Close()
				}
				if err != nil {
					_ = events.Close()
					t.Fatal(err)
				}
				gone, populated, err = readTextWorkerCgroup(events)
				closeErr := events.Close()
				if err != nil || closeErr != nil || !gone && populated {
					t.Fatalf("cleanup returned before cgroup became empty: %v; %v", err, closeErr)
				}
				if worker.grant.Active() != 0 || worker.lease.Context().Err() == nil {
					t.Fatal("worker authority survived joined cleanup")
				}
				if attempt != 11 && !worker.completedCurrent() {
					t.Fatal("worker cleanup lost the surviving context")
				}
				if err := worker.Close(); err != nil {
					t.Fatal(err)
				}
				requireInstalledTextWorkerCollected(t, ctx, instance.name, role.name)
				previous, previousNonce = worker, currentNonce
			}
			if previous.completedCurrent() {
				t.Fatal("completion survived Endpoint context revocation")
			}
			if worker, err := owner.launchTextWorker(ctx, role.snapshot); err == nil || worker != nil {
				t.Fatal("revoked context launched another worker")
			}
			t.Log("12 installed activations: real readiness and Grant; fresh jobs; pinned cgroups empty; context revoke refused late launch")
		})
	}
}

func installedTextWorkerInstance(t *testing.T, ctx context.Context, worker *qualifiedTextWorker, role string) textWorkerInstance {
	t.Helper()
	listing, err := listTextWorkerInstances(ctx, role)
	if err != nil {
		t.Fatal(err)
	}
	for name, state := range listing {
		if state.active != "active" {
			continue
		}
		instance, err := observeTextWorkerInstance(ctx, name, role)
		if err == nil && instance.pid == worker.lifetime.attachment.pid && instance.uid == worker.lifetime.attachment.uid {
			return instance
		}
	}
	t.Fatal("exact verified worker invocation unavailable")
	return textWorkerInstance{}
}

// Collection is a resource assertion after pinned cgroup cleanup has succeeded.
// An absent unit is never substituted for the kernel's cleanup observation.
func requireInstalledTextWorkerCollected(t *testing.T, parent context.Context, name, role string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		listing, err := listTextWorkerInstances(ctx, role)
		if err != nil {
			t.Fatalf("retired worker inventory: %v", err)
		}
		state, retained := listing[name]
		if !retained {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("cleaned worker unit retained in state %s: %s", state.active, name)
		case <-tick.C:
		}
	}
}

func requireInstalledTextWorkerCollectionPolicy(t *testing.T, ctx context.Context, instance textWorkerInstance) {
	t.Helper()
	unit, service, err := readTextWorkerProperties(ctx, instance.name, instance.role)
	if err != nil {
		t.Fatal(err)
	}
	verify := func() error {
		return verifyTextWorkerProperties(unit, service, instance.name, instance.role, instance.cgroup, instance.pid)
	}
	if err := verify(); err != nil {
		t.Fatalf("installed property positive control: %v", err)
	}
	for _, invalid := range []textManagerValue{
		{Type: "s", Data: json.RawMessage(`"inactive"`)},
		{Type: "s", Data: json.RawMessage(`"unknown"`)},
		{Type: "s", Data: json.RawMessage(`null`)},
		{Type: "as", Data: json.RawMessage(`["inactive-or-failed"]`)},
	} {
		unit["CollectMode"] = invalid
		if verify() == nil {
			t.Fatal("unverified collection policy admitted")
		}
	}
	delete(unit, "CollectMode")
	if verify() == nil {
		t.Fatal("missing collection policy admitted")
	}
}
