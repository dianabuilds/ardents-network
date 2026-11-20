//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

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
