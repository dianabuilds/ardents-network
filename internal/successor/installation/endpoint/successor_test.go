package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestSuccessorRefusesMissingAndCanceledOpening(t *testing.T) {
	var missing context.Context
	if owner, err := OpenSuccessor(missing, "/missing-request"); owner != nil || !errors.Is(err, ErrInput) {
		t.Fatal("missing caller accepted", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if owner, err := OpenSuccessor(ctx, "/missing-request"); owner != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation lost", err)
	}
	for _, owner := range []*Successor{nil, {}} {
		if result, err := owner.Complete(nil, enrollment.Candidate{}, release.Inputs{}); result != (ProvisionResult{}) || !errors.Is(err, ErrInput) {
			t.Fatal("detached handle accepted", result, err)
		}
		if owner.Request().BundleRoot() != "" || owner.Close() != nil {
			t.Fatal("detached handle has custody")
		}
	}
}
