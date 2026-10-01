//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

// The selected hostile artifact uses actual INIT/readiness and Grant, while
// its child and grandchild ignore TERM and retain the accepted attachment.
// Only local caller authorization is a fixture. No Route journey is claimed.
func TestInstalledTextWorkerHostileTree(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	if err := worker.VerifyEndpointService(ctx); err != nil {
		t.Fatalf("invalid installed environment: %v", err)
	}
	for _, role := range []struct {
		name     string
		surface  broker.Surface
		snapshot []byte
	}{
		{"reader", broker.Connection, nil},
		{"publisher", broker.Administration, []byte("victim snapshot")},
	} {
		t.Run(role.name, func(t *testing.T) {
			endpoint, principal := dutyContextEndpoint(t)
			siblingOwner := admittedDutyContext(t, endpoint, principal, broker.Administration)
			document := []byte("sibling publication survives hostile tree cleanup\n")
			sibling := launchInstalledHostileWorker(t, ctx, siblingOwner, document)
			siblingUnit := installedWorkerInstance(t, ctx, sibling, "publisher")
			siblingEvents, siblingPIDs := pinInstalledHostileTree(t, siblingUnit)

			owner := admittedDutyContext(t, endpoint, principal, role.surface)
			bound := launchInstalledHostileWorker(t, ctx, owner, role.snapshot)
			unit := installedWorkerInstance(t, ctx, bound, role.name)
			events, processes := pinInstalledHostileTree(t, unit)
			if unit.UID == siblingUnit.UID || unit.Cgroup == siblingUnit.Cgroup {
				t.Fatal("sibling workers share an isolation identity")
			}
			owner.mu.Lock()
			originalNonce := bound.job.nonce
			owner.mu.Unlock()
			if originalNonce == [32]byte{} {
				t.Fatal("live victim nonce is absent")
			}
			if err := bound.Close(); err != nil {
				t.Fatal(err)
			}
			requireInstalledTreeGone(t, events, processes)
			if bound.grant.Active() != 0 || bound.lease.Context().Err() == nil || !bound.completedCurrent() {
				t.Fatal("cleanup failed to revoke only the victim job")
			}
			requireInstalledWorkerCollected(t, ctx, unit.Name, role.name)
			if current, err := worker.ObserveInstance(ctx, siblingUnit.Name, "publisher"); err != nil || current != siblingUnit {
				t.Fatalf("victim cleanup changed sibling invocation: %v", err)
			}
			if sibling.grant.Active() != 1 || sibling.lease.Context().Err() != nil {
				t.Fatal("victim cleanup revoked sibling authority")
			}
			if gone, populated, err := worker.ReadCgroup(siblingEvents); err != nil || gone || !populated {
				t.Fatal("victim cleanup terminated sibling tree")
			}
			requireInstalledPublisherProgress(t, ctx, sibling, document)
			if err := siblingOwner.Close(); err != nil {
				t.Fatal(err)
			}
			requireInstalledTreeGone(t, siblingEvents, siblingPIDs)
			requireInstalledWorkerCollected(t, ctx, siblingUnit.Name, "publisher")
			if sibling.completedCurrent() || sibling.grant.Active() != 0 {
				t.Fatal("sibling owner revoke retained authority")
			}
			// The peer closes while the Endpoint job is still alive. The actual
			// worker parent exits on EOF; descendants retain the socket and ignore
			// TERM. Observe the manager's cleanup before invoking Endpoint Close.
			replacement := launchInstalledHostileWorker(t, ctx, owner, role.snapshot)
			owner.mu.Lock()
			replacementNonce := replacement.job.nonce
			owner.mu.Unlock()
			if replacement.job == bound.job || replacementNonce == originalNonce || replacementNonce == [32]byte{} || bound.completedCurrent() {
				t.Fatal("replacement inherited the retired job")
			}
			replacementUnit := installedWorkerInstance(t, ctx, replacement, role.name)
			replacementEvents, replacementPIDs := pinInstalledHostileTree(t, replacementUnit)
			if err := replacement.lifetime.attachment.Close(); err != nil {
				t.Fatal(err)
			}
			waitInstalledParentExit(t, ctx, replacementEvents, replacementPIDs, replacementUnit.PID)
			if err := replacement.Close(); err != nil {
				t.Fatal(err)
			}
			requireInstalledTreeGone(t, replacementEvents, replacementPIDs)
			if replacement.grant.Active() != 0 || !replacement.completedCurrent() {
				t.Fatal("parent-exit cleanup retained the old job or lost its context")
			}
			requireInstalledWorkerCollected(t, ctx, replacementUnit.Name, role.name)
			t.Log("hostile descendants removed; sibling served snapshot; owner revoke joined sibling; parent exit triggered manager cleanup before Endpoint Close")
		})
	}
}

func launchInstalledHostileWorker(t *testing.T, ctx context.Context, owner *dutyContext, snapshot []byte) *qualifiedWorker {
	t.Helper()
	bound, err := owner.launchWorker(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := bound.Close(); err != nil {
			t.Error(err)
		}
	})
	if bound.grant.Active() != 1 || bound.lease.Context().Err() != nil {
		t.Fatal("hostile artifact did not pass actual readiness and Grant")
	}
	return bound
}

func requireInstalledTreeGone(t *testing.T, events *os.File, processes []installedHostileProcess) {
	t.Helper()
	gone, populated, err := worker.ReadCgroup(events)
	if err != nil || !gone && populated {
		t.Fatalf("original hostile cgroup still populated: %v", err)
	}
	for _, process := range processes {
		current, err := readInstalledHostileProcess(process.pid)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if current.started == process.started && current.state != "Z" && current.state != "X" {
			t.Fatal("original hostile process survived joined cleanup")
		}
	}
}
