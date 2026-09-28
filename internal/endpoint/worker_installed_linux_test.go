//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

// This selected installed profile supplies only local caller authorization as
// a fixture. Platform, service identity, artifacts, worker activation, readiness,
// scoped Grant and cgroup cleanup all cross the actual production boundary.
// It is not a protected Service journey or the complete hostile-worker matrix.
func TestInstalledTextWorkerLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 110*time.Second)
	defer cancel()
	if err := worker.VerifyEndpointService(ctx); err != nil {
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
			var previous *qualifiedWorker
			var previousNonce [32]byte
			for attempt := 0; attempt < 12; attempt++ {
				bound, err := owner.launchWorker(ctx, role.snapshot)
				if err != nil {
					t.Fatalf("attempt %d launch: %v", attempt, err)
				}
				t.Cleanup(func() {
					if err := bound.Close(); err != nil {
						t.Error(err)
					}
				})
				if bound.grant.Active() != 1 || bound.lease.Context().Err() != nil {
					t.Fatal("verified readiness did not acquire a live scoped Grant")
				}
				owner.mu.Lock()
				currentNonce := bound.job.nonce
				owner.mu.Unlock()
				if currentNonce == [32]byte{} {
					t.Fatal("live job nonce is absent")
				}
				if previous != nil && (previous.job == bound.job ||
					previousNonce == currentNonce || previous.completedCurrent()) {
					t.Fatal("replacement inherited a prior job or completion")
				}
				instance := installedWorkerInstance(t, ctx, bound, role.name)
				if attempt == 0 {
					requireInstalledWorkerCollectionPolicy(t, ctx, instance)
				}
				events, err := worker.PinCgroup(instance)
				if err != nil {
					t.Fatal(err)
				}
				gone, populated, err := worker.ReadCgroup(events)
				if err != nil || gone || !populated {
					_ = events.Close()
					t.Fatalf("missing live cgroup positive control: %v", err)
				}
				if attempt == 11 {
					err = owner.Close()
				} else {
					err = bound.Close()
				}
				if err != nil {
					_ = events.Close()
					t.Fatal(err)
				}
				gone, populated, err = worker.ReadCgroup(events)
				closeErr := events.Close()
				if err != nil || closeErr != nil || !gone && populated {
					t.Fatalf("cleanup returned before cgroup became empty: %v; %v", err, closeErr)
				}
				if bound.grant.Active() != 0 || bound.lease.Context().Err() == nil {
					t.Fatal("worker authority survived joined cleanup")
				}
				if attempt != 11 && !bound.completedCurrent() {
					t.Fatal("worker cleanup lost the surviving context")
				}
				if err := bound.Close(); err != nil {
					t.Fatal(err)
				}
				requireInstalledWorkerCollected(t, ctx, instance.Name, role.name)
				previous, previousNonce = bound, currentNonce
			}
			if previous.completedCurrent() {
				t.Fatal("completion survived Endpoint context revocation")
			}
			if bound, err := owner.launchWorker(ctx, role.snapshot); err == nil || bound != nil {
				t.Fatal("revoked context launched another worker")
			}
			t.Log("12 installed activations: real readiness and Grant; fresh jobs; pinned cgroups empty; context revoke refused late launch")
		})
	}
}

func installedWorkerInstance(t *testing.T, ctx context.Context, bound *qualifiedWorker, role string) worker.Instance {
	t.Helper()
	listing, err := worker.ListInstances(ctx, role, worker.Text)
	if err != nil {
		t.Fatal(err)
	}
	for name, state := range listing {
		if state.Active != "active" {
			continue
		}
		instance, err := worker.ObserveInstance(ctx, name, role)
		if err == nil && instance.PID == bound.lifetime.attachment.PID() && instance.UID == bound.lifetime.attachment.UID() {
			return instance
		}
	}
	t.Fatal("exact verified worker invocation unavailable")
	return worker.Instance{}
}

// Collection is a resource assertion after pinned cgroup cleanup has succeeded.
// An absent unit is never substituted for the kernel's cleanup observation.
func requireInstalledWorkerCollected(t *testing.T, parent context.Context, name, role string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		listing, err := worker.ListInstances(ctx, role, worker.Text)
		if err != nil {
			t.Fatalf("retired worker inventory: %v", err)
		}
		state, retained := listing[name]
		if !retained {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("cleaned worker unit retained in state %s: %s", state.Active, name)
		case <-tick.C:
		}
	}
}

func requireInstalledWorkerCollectionPolicy(t *testing.T, ctx context.Context, instance worker.Instance) {
	t.Helper()
	version, err := worker.ManagerVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unit, service, err := worker.ReadProperties(ctx, instance.Name, instance.Role)
	if err != nil {
		t.Fatal(err)
	}
	verify := func() error {
		return worker.VerifyPropertiesVersion(unit, service, instance.Name, instance.Role, instance.Cgroup, instance.PID, version)
	}
	if err := verify(); err != nil {
		t.Fatalf("installed property positive control: %v", err)
	}
	var slice string
	if json.Unmarshal(service["Slice"].Data, &slice) != nil {
		t.Fatal("verified worker Slice could not be decoded")
	}
	t.Logf("verified installed %s worker Slice=%q ControlGroup=%q", instance.Role, slice, instance.Cgroup)
	for _, invalid := range []worker.Value{
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
