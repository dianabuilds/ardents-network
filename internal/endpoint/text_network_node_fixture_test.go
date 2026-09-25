//go:build linux

package endpoint

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

// textNetworkNodeRuntime owns one fixture Node's readiness and joined cleanup.
// The role-network builder owns the selected config and port lease.
type textNetworkNodeRuntime struct {
	ready   chan struct{}
	history *textNetworkNodeEvents
}

func newTextNetworkNodeRuntime(history *textNetworkNodeEvents) *textNetworkNodeRuntime {
	return &textNetworkNodeRuntime{ready: make(chan struct{}, 1), history: history}
}

func (runtime *textNetworkNodeRuntime) emit(_ context.Context, event node.Event) error {
	runtime.history.record(event)
	if event.State == "READY" {
		select {
		case runtime.ready <- struct{}{}:
		default:
		}
	}
	return nil
}

func (runtime *textNetworkNodeRuntime) start(t *testing.T, index int, config node.Config,
	runner func(*testing.T, int, node.Config) func() error) {
	t.Helper()
	if runner != nil {
		stop := runner(t, index, config)
		t.Cleanup(func() {
			if err := stop(); err != nil {
				t.Error(err)
			}
		})
		return
	}
	// Nodes are fixture infrastructure. testing.T cancels its Context before
	// Cleanup, which would race an unrelated network shutdown against the
	// Endpoint's normal cleanup. The explicit cleanup below owns each Node.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		result, err := node.Run(ctx, config)
		if err != nil {
			err = fmt.Errorf("Node %d result %+v: %w", index, result, err)
		}
		done <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(4 * time.Second):
			t.Error("Node runtime did not join")
		}
	})
	select {
	case <-runtime.ready:
	case err := <-done:
		done <- err
		t.Fatalf("Node %d failed before READY: %v", index, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("Node %d did not become READY", index)
	}
}
