//go:build linux

package issuance

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// Cancel the real context at one existing observation boundary, without sleeps.
// Once canceled, every later Err call retains the same terminal error.
type resultCancellation struct {
	context.Context
	cancel context.CancelFunc
	polls  atomic.Int32
	at     int32
}

func (c *resultCancellation) Err() error {
	if c.polls.Add(1) == c.at {
		c.cancel()
	}
	return c.Context.Err()
}

func TestResultCancellationDuringValidationAndReopen(t *testing.T) {
	store, inventory, root := resultOwners(t)
	raw, facts, binding, verify := resultFixture(t, inventory, 1, 1, 1)
	ledgerRoot := filepath.Join(t.TempDir(), "admission")
	if err := quota.Initialize(ledgerRoot, binding); err != nil {
		t.Fatal(err)
	}
	ledger, err := quota.Open(ledgerRoot, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	outcome, confirmation := ledger.DebitVerified(t.Context(), raw, facts, quota.Bootstrap)
	if outcome != quota.Debited {
		t.Fatal(outcome)
	}
	if err := InitializeResults(t.Context(), root, store, binding); err != nil {
		t.Fatal(err)
	}
	results, err := OpenResults(t.Context(), root, store, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer results.Close()
	before, err := os.ReadFile(filepath.Join(root, "results.journal"))
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []int32{2, 3, 4} {
		base, cancel := context.WithCancel(t.Context())
		ctx := &resultCancellation{Context: base, cancel: cancel, at: at}
		response, replay, err := results.Issue(ctx, confirmation, facts.Now)
		cancel()
		if !errors.Is(err, context.Canceled) || response != nil || replay {
			t.Errorf("issue cancellation at observation %d: response=%t replay=%t error=%v", at, response != nil, replay, err)
		}
	}
	after, err := os.ReadFile(filepath.Join(root, "results.journal"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("canceled validation changed results", err)
	}
	response, replay, err := results.Issue(t.Context(), confirmation, facts.Now)
	if err != nil || replay {
		t.Fatal("retry after cancellation", err, replay)
	}
	verify(response)
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
	for _, at := range []int32{3, 4, 5, 6} {
		base, cancel := context.WithCancel(t.Context())
		ctx := &resultCancellation{Context: base, cancel: cancel, at: at}
		opened, err := OpenResults(ctx, root, store, binding)
		cancel()
		if err == nil {
			_ = opened.Close()
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("reopen cancellation at observation %d: %v", at, err)
		}
		// Cancellation must release the lease and leave the retained result usable.
		opened, err = OpenResults(t.Context(), root, store, binding)
		if err != nil {
			t.Fatal("reopen after cancellation", err)
		}
		saved, replay, issueErr := opened.Issue(t.Context(), confirmation, facts.Now)
		closeErr := opened.Close()
		if issueErr != nil || closeErr != nil || !replay || !bytes.Equal(response, saved) {
			t.Fatal("retained result after cancellation", issueErr, closeErr, replay)
		}
	}
}
