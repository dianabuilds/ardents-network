//go:build linux && endpoint_launch_native

package endpoint

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

func TestNET32IdleFreshBoundaryBypassesRecentHostingSample(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hosting")
	now := time.Now().UTC().Truncate(time.Second)
	if err := hosting.Initialize(root, hosting.Policy{
		Provider: "NET-32 fresh boundary fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "MiB", Quantity: 64, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	budget, err := hosting.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := budget.Close(); err != nil {
			t.Error(err)
		}
	})
	initial, err := budget.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := readNet32Sample(t.Context(), budget, root, true, func(attempt int, err error) {
		t.Errorf("unexpected failed Hosting read %d: %v", attempt, err)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.hostingAt.After(initial.ObservedAt()) {
		t.Fatalf("boundary reused the recent persisted sample: initial %v fresh %v", initial.ObservedAt(), fresh.hostingAt)
	}
}

func TestNET32IdleRetainsHostingAttemptsAfterRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		first := errors.New("first Hosting read failed")
		second := errors.New("second Hosting read failed")
		outcomes := []error{first, second, nil}
		var history []net32FailedAttempt
		calls := 0
		_, err := retryNet32Hosting(t.Context(), func() (hosting.Observation, error) {
			defer func() { calls++ }()
			return hosting.Observation{}, outcomes[calls]
		}, func(attempt int, err error) {
			history = append(history, net32FailedAttempt{Stage: "window-hosting", Sample: 1, Attempt: attempt, Error: err.Error()})
		})
		if err != nil || calls != 3 {
			t.Fatalf("repeated read: calls=%d, error=%v", calls, err)
		}
		verdict := evaluateNet32Idle(net32IdleReport{FailedAttempts: history}, nil, 0, nil)
		want := []net32FailedAttempt{
			{Stage: "window-hosting", Sample: 1, Attempt: 1, Error: first.Error()},
			{Stage: "window-hosting", Sample: 1, Attempt: 2, Error: second.Error()},
		}
		if !slices.Equal(verdict.FailedAttempts, want) {
			t.Fatalf("failed attempts = %+v, want %+v", verdict.FailedAttempts, want)
		}
	})
}
