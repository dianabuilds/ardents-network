//go:build linux

package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestInfrastructureOriginalLoaderAndVerifierSurviveToHandoff(t *testing.T) {
	dir, _, _ := infrastructureDistribution(t, []byte("signed candidate program"), nil, 1, nil)
	loader, cancel := context.WithCancel(t.Context())
	defer cancel()
	c, err := enrollment.ReadInfrastructureCandidate(loader, dir)
	if err != nil {
		t.Fatal(err)
	}
	in := infrastructureCandidateInput(t, c)
	v, err := release.Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	if d := v.Evaluate(t.Context(), in); d.Outcome != release.OutcomeReleaseAccepted {
		t.Fatalf("healthy prerequisite %s %s %v", d.Outcome, d.Notice, d.Err())
	}
	a, err := infrastructure.AuthenticateCandidate(t.Context(), v, c, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Check(t.Context(), v); err != nil {
		t.Fatal("healthy original", err)
	}
	if err := (infrastructure.Authorization{}).Check(t.Context(), v); err == nil {
		t.Fatal("zero handoff")
	}
	cancel()
	if err := a.Check(t.Context(), v); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled original snapshot revived", err)
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c, in); !errors.Is(err, context.Canceled) {
		t.Fatal("new caller revived canceled loader", err)
	}
	fresh, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err = infrastructure.AuthenticateCandidate(t.Context(), v, fresh, in)
	if err != nil {
		t.Fatal("independently loaded healthy counterpart", err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Check(t.Context(), v); !errors.Is(err, release.ErrClosed) {
		t.Fatal("closed original verifier revived", err)
	}
}
