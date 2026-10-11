//go:build linux && text_worker_installed

package runtime

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// Requires the existing independently observed ordinary non-root Endpoint
// profile. This exercises real snapshot INIT/READY and original operation join;
// it establishes no Route, Credential, Store ACK or Service delivery.
func testInstalledPublisherSnapshotOperation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: installed non-root Endpoint service required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	principal := [32]byte{2}
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	invocation, err := owner.LaunchPublisher(ctx, principal, []byte("selected Publisher snapshot\n"))
	if err != nil {
		t.Fatal(err)
	}
	operation, err := invocation.BeginOperation(ctx)
	if err != nil {
		invocation.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		operation.Close()
		if err := invocation.Close(); err != nil {
			t.Error(err)
		}
	})
	if operation.CheckPublisher() != nil || invocation.CompletedCurrent() {
		t.Fatal("original snapshot operation unavailable or already joined")
	}
	original, err := operation.AdministrationContext()
	if err != nil || original.Context() == nil || original.Done() == nil || original.Check(operation) != nil {
		t.Fatal("qualified Publisher lacks its original Administration context", err)
	}
	if repeated, err := operation.AdministrationContext(); err != nil || repeated != original {
		t.Fatal("Administration context identity changed", err)
	}
	if original.Check(&Operation{}) == nil {
		t.Fatal("original Administration context accepted a foreign operation")
	}
	invocation.job.Retire()
	if operation.CheckPublisher() == nil {
		t.Fatal("retired original snapshot operation accepted effects")
	}
	if original.Context().Err() != nil || original.Check(operation) == nil {
		t.Fatal("Job retirement changed context lifetime or retained worker authority")
	}
	select {
	case <-original.Done():
		t.Fatal("Job retirement completed its original Administration context")
	default:
	}
	select {
	case <-invocation.done:
		t.Fatal("snapshot worker cleanup bypassed original operation join")
	default:
	}
	operation.Close()
	if err := invocation.Close(); err != nil || !invocation.CompletedCurrent() {
		t.Fatal("original snapshot invocation did not join cleanly", err)
	}
	select {
	case <-original.Done():
		t.Fatal("joined Job erased its still-retained Administration context")
	default:
	}
	if err := owner.Close(); err != nil {
		t.Fatal("original Administration generation did not join", err)
	}
	select {
	case <-original.Done():
	default:
		t.Fatal("original Administration context did not finish with its owner")
	}
}
