package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestSuccessorRecoveryRejectsInvalidAndCanceledOpening(t *testing.T) {
	var nilContext context.Context
	for _, test := range []struct {
		ctx       context.Context
		root      string
		reference time.Time
	}{
		{nilContext, "/installation", time.Now()},
		{t.Context(), "relative", time.Now()},
		{t.Context(), "/", time.Now()},
		{t.Context(), "/installation", time.Time{}},
	} {
		if owner, err := OpenSuccessorRecovery(test.ctx, test.root, test.reference); owner != nil || !errors.Is(err, ErrInput) {
			t.Fatal("invalid opening obtained custody", owner, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if owner, err := OpenSuccessorRecovery(ctx, "/installation", time.Now()); owner != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled opening entered native custody", owner, err)
	}
}

func TestSuccessorRecoveryCopiesRetainClosedAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	original := &SuccessorRecovery{state: &successorRecoveryOperation{ctx: ctx, cancel: cancel}}
	copy := *original
	if err := original.Close(); err != nil {
		t.Fatal(err)
	}
	if !copy.state.used {
		t.Fatal("copy renewed admission")
	}
	if result, err := copy.Complete(nil, enrollment.Candidate{}, release.Inputs{}); result != (ProvisionResult{}) || !errors.Is(err, ErrInput) {
		t.Fatal("closed copy completed", result, err)
	}
	var absent *SuccessorRecovery
	if absent.BundleRoot() != "" || absent.ReleaseHistoryRoot() != "" || !absent.ReferenceTime().IsZero() || absent.Close() != nil {
		t.Fatal("nil recovery retained inputs")
	}
	if _, err := absent.Complete(nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
}

// A retained post-acceptance result is bookkeeping, not a simulated ACK.
func TestSuccessorRecoveryCopyPreservesRetainedPostAcceptanceFailure(t *testing.T) {
	original := &SuccessorRecovery{state: &successorRecoveryOperation{ctx: t.Context(), used: true,
		result: ProvisionResult{Status: "installed-started-recovery-required", GenerationDigest: "retained-generation"}, terminal: context.Canceled}}
	copy := *original
	result, err := copy.Complete(nil, enrollment.Candidate{}, release.Inputs{})
	if result != original.state.result || !errors.Is(err, ErrInput) || !errors.Is(err, context.Canceled) {
		t.Fatal("copy erased retained admission outcome", result, err)
	}
}
