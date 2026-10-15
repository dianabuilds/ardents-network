package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

func TestDetachedPreparationGrantsNoPermission(t *testing.T) {
	for _, preparation := range []*Preparation{nil, {}} {
		if preparation.Check() == nil {
			t.Fatal("detached preparation granted authority")
		}
	}
}

// These cases exercise the real admitted Session/Job retirement seam, without
// supplying a qualified worker or installed startup authority.
func TestPreparationCheckRetainsOriginalCancellationAndCleanup(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "joined-preparation", true: "cleanup-failure"}[failed], func(t *testing.T) {
			owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: [32]byte{2}, Surface: execution.Connection}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			capability, err := owner.authority.Admit([32]byte{2}, execution.Connection)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			session, err := owner.supervisor.Activate(ctx, capability, [32]byte{2}, execution.Connection)
			if err != nil {
				t.Fatal(err)
			}
			job, err := session.BeginJob()
			if err != nil {
				t.Fatal(err)
			}
			preparation := &Preparation{session: session, job: job}
			cleanupFailure := errors.New("original cleanup failure")
			var result error
			if failed {
				result = cleanupFailure
			}
			job.Retire()
			if err := job.Finish(result); !errors.Is(err, result) {
				t.Fatal("original completion differs", err)
			}
			if !failed && preparation.Check() != nil {
				t.Fatal("original completed Job lost preparation provenance")
			}
			cancel()
			if err := preparation.Close(); !errors.Is(err, result) {
				t.Fatal("joined cleanup differs", err)
			}
			err = preparation.Check()
			if !errors.Is(err, context.Canceled) || (failed && !errors.Is(err, cleanupFailure)) {
				t.Fatal("retirement lost original cancellation or cleanup", err)
			}
			if !failed && err.Error() != context.Canceled.Error() {
				t.Fatal("ordinary cancellation invented an additional failure", err)
			}
		})
	}
}
