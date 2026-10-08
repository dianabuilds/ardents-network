//go:build linux && text_worker_installed

package runtime

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// This component profile runs as the original non-root Endpoint MainPID with
// genuine fixed installed Text artifacts and activation units. It qualifies
// neither Installation's completion exchange nor a Service Connection.
func TestInstalledExecutionLifecycle(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: installed non-root Endpoint service required")
	}
	for _, surface := range []execution.Surface{execution.Connection, execution.Administration} {
		t.Run(string(surface), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			principal := [32]byte{2}
			owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: surface}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := owner.Close(); err != nil {
					t.Errorf("original Execution cleanup failed: %v", err)
				}
			})
			prepared, err := owner.Prepare(ctx, principal, surface)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := prepared.Close(); err != nil {
					t.Errorf("original Session cleanup failed: %v", err)
				}
			})
			if err := prepared.Check(); err != nil || prepared.job.Check() == nil || len(launchGate) != 0 {
				t.Fatal("joined preparation retained live worker permission or unjoined inventory")
			}
			if err := prepared.Close(); err != nil {
				t.Fatal(err)
			}
			if prepared.Check() == nil {
				t.Fatal("retired Session retained preparation permission")
			}
			t.Run("live-operation", func(t *testing.T) {
				invocation, err := owner.Launch(ctx, principal, surface)
				if err != nil {
					t.Fatal(err)
				}
				operation, err := invocation.BeginOperation(ctx)
				if err != nil {
					_ = invocation.Close()
					t.Fatal(err)
				}
				t.Cleanup(func() {
					operation.Close()
					if err := invocation.Close(); err != nil {
						t.Errorf("original invocation cleanup: %v", err)
					}
				})
				if operation.Check() != nil || invocation.CompletedCurrent() || len(launchGate) != 0 {
					t.Fatal("qualified original invocation is unavailable or prematurely completed")
				}
				if next, err := invocation.BeginOperation(ctx); err == nil || next != nil {
					t.Fatal("one qualified Job yielded another operation")
				}
				joined := make(chan error, 1)
				go func() { joined <- invocation.Close() }()
				select {
				case <-operation.Context().Done():
				case <-ctx.Done():
					t.Fatal("retirement did not interrupt operation")
				}
				if operation.Check() == nil {
					t.Fatal("retired operation accepted effects")
				}
				select {
				case err := <-joined:
					t.Fatalf("invocation finished before operation joined: %v", err)
				default:
				}
				operation.Close()
				select {
				case err := <-joined:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal("original invocation did not join")
				}
				if !invocation.CompletedCurrent() || operation.Check() == nil {
					t.Fatal("joined original result or retired byte permission is wrong")
				}
				if next, err := invocation.BeginOperation(ctx); err == nil || next != nil {
					t.Fatal("completed invocation reclaimed its operation")
				}
			})
			t.Run("attachment-loss", func(t *testing.T) {
				invocation, err := owner.Launch(ctx, principal, surface)
				if err != nil {
					t.Fatal(err)
				}
				operation, err := invocation.BeginOperation(ctx)
				if err != nil {
					_ = invocation.Close()
					t.Fatal(err)
				}
				t.Cleanup(func() { operation.Close(); _ = invocation.Close() })
				// Break the real original socket without first retiring its Job.
				// The actual installed worker and pinned scope must still join.
				if err := invocation.activation.Attachment.Close(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-operation.Context().Done():
				case <-ctx.Done():
					t.Fatal("original attachment loss left live operation permission")
				}
				if operation.Check() == nil {
					t.Fatal("lost attachment accepted effects")
				}
				select {
				case <-invocation.done:
					t.Fatal("attachment loss bypassed operation join")
				default:
				}
				operation.Close()
				if err := invocation.Close(); !errors.Is(err, errUnexpectedWorkerEnd) {
					t.Fatalf("lost original attachment result: %v", err)
				}
				if invocation.CompletedCurrent() {
					t.Fatal("failed worker supplied completed-current provenance")
				}
				if err := invocation.Close(); !errors.Is(err, errUnexpectedWorkerEnd) {
					t.Fatal("repeat close changed worker-loss result", err)
				}
				// This is a worker-use failure, not a physical cleanup failure.
				// Genuine joined descendants do not terminally poison siblings.
				prepared, err := owner.Prepare(ctx, principal, surface)
				if err != nil {
					t.Fatal("joined worker loss prevented a fresh original launch", err)
				}
				if err := prepared.Close(); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
	t.Run("cleanup-failure", testInstalledExecutionCleanupFailure)
}

// The injected error belongs to an already joined operation borrower. Every
// successful launch and worker join still uses the actual installed mechanism.
func testInstalledExecutionCleanupFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	principals := [][32]byte{{3}, {4}}
	surfaces := []execution.Surface{execution.Connection, execution.Administration}
	owner, err := New(execution.Config{ID: [32]byte{5}, Grants: []execution.Grant{
		{Principal: principals[0], Surface: surfaces[0]}, {Principal: principals[1], Surface: surfaces[1]},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var operations []*Operation
	var invocations []*Invocation
	var first error
	t.Cleanup(func() {
		for _, operation := range operations {
			operation.Close()
		}
		for _, invocation := range invocations {
			_ = invocation.Close()
		}
		if err := owner.Close(); first == nil && err != nil || first != nil && !errors.Is(err, first) {
			t.Errorf("generation changed original cleanup result: %v", err)
		}
	})
	for index, surface := range surfaces {
		invocation, err := owner.Launch(ctx, principals[index], surface)
		if err != nil {
			t.Fatal(err)
		}
		invocations = append(invocations, invocation)
		operation, err := invocation.BeginOperation(ctx)
		if err != nil {
			t.Fatal(err)
		}
		operations = append(operations, operation)
	}
	for _, operation := range operations {
		if err := operation.Check(); err != nil {
			t.Fatal(err)
		}
	}
	first = errors.New("fixture: joined borrower cleanup failure")
	operations[0].RetainCleanup(first)
	operations[0].RetainCleanup(errors.New("fixture: later cleanup failure"))
	for _, operation := range operations {
		if operation.Check() == nil {
			t.Fatal("generation failure left a sibling accepting effects")
		}
	}
	joined := make(chan error, 1)
	go func() { joined <- owner.Close() }()
	for _, operation := range operations {
		select {
		case <-operation.Context().Done():
		case <-ctx.Done():
			t.Fatal("generation failure did not interrupt every operation")
		}
	}
	select {
	case err := <-joined:
		t.Fatalf("generation released unjoined operations: %v", err)
	default:
	}
	// An independently joined sibling cannot make the failed generation current.
	operations[1].Close()
	if err := invocations[1].Close(); err != nil {
		t.Fatal(err)
	}
	if invocations[1].CompletedCurrent() {
		t.Fatal("failed generation accepted late sibling completion")
	}
	select {
	case err := <-joined:
		t.Fatalf("generation released the first unjoined operation: %v", err)
	default:
	}
	operations[0].Close()
	select {
	case err := <-joined:
		if !errors.Is(err, first) {
			t.Fatalf("lost first cleanup failure: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("original generation did not join")
	}
	if invocations[0].CompletedCurrent() {
		t.Fatal("failed original invocation supplied completed-current provenance")
	}
	if err := owner.Close(); !errors.Is(err, first) {
		t.Fatalf("repeated close erased first failure: %v", err)
	}
}
