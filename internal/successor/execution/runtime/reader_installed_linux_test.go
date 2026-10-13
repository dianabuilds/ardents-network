//go:build linux && text_worker_installed

package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// This qualified local component deliberately stops before any Service. It
// proves the ordinary Reader's original mode/one-use claim and physical join,
// not successful Connection bytes, completion or two-Endpoint acceptance.
func testInstalledReaderOperation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	principal := [32]byte{2}
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	invocation, err := owner.LaunchReader(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := invocation.BeginOperation(ctx)
	if err != nil {
		_ = invocation.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); operation.Close(); _ = invocation.Close() })
	if err := operation.CheckReader(); err != nil || invocation.CompletedCurrent() {
		t.Fatal("original ordinary Reader unavailable or prematurely complete", err)
	}
	local, err := operation.ConnectionContext()
	if err != nil || local == [32]byte{} {
		t.Fatal("original Reader logical context absent", err)
	}
	if err := operation.ClaimConnection(local); err != nil {
		t.Fatal(err)
	}
	if err := operation.ClaimConnection(local); err == nil {
		t.Fatal("Reader yielded a second logical construction")
	}
	altered := local
	altered[0] ^= 1
	if err := operation.ClaimConnection(altered); err == nil {
		t.Fatal("foreign context acquired original Reader construction")
	}
	if current, err := operation.ConnectionContext(); err != nil || current != local || operation.CheckReader() != nil {
		t.Fatal("claim changed original Reader authority/context", err)
	}
	cancel()
	select {
	case <-operation.Context().Done():
	case <-t.Context().Done():
		t.Fatal("original Reader cancellation did not interrupt operation")
	}
	if operation.CheckReader() == nil {
		t.Fatal("canceled Reader accepted effects")
	}
	operation.Close()
	result := invocation.Close()
	if !errors.Is(result, context.Canceled) || invocation.CompletedCurrent() {
		t.Fatal("canceled pre-Service Reader supplied completion", result)
	}
	if physical, joined := invocation.CleanupResult(); !joined || physical != nil {
		t.Fatal("original Reader descendants did not join", physical)
	}
	if repeated := invocation.Close(); !errors.Is(repeated, context.Canceled) {
		t.Fatal("Reader lost retained cancellation result", repeated)
	}
}
