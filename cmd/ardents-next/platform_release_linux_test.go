//go:build linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Signed in-process byte fixtures isolate the Release handoff. No process,
// system manager, network, initial independent pin or startup is simulated.
func TestPlatformRuntimeReleaseUsesRetainedFloorsAndOriginalProof(t *testing.T) {
	for _, role := range []string{"source", "forwarding", "issuer", "descriptor", "introduction", "join"} {
		t.Run(role, func(t *testing.T) {
			program := []byte("signed program fixture")
			dir, _, _ := infrastructureDistributionInvocation(t, t.TempDir(), program, nil, 1, nil, role, []byte(platformUnitTemplate))
			candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			in := infrastructureCandidateInput(t, candidate)
			history := filepath.Join(t.TempDir(), "release")
			v, err := release.Open(history)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, candidate, in); err == nil {
				t.Fatal("candidate supplied initial trust")
			}
			// Test setup explicitly seeds the genuine verifier, not a serialized
			// accepting fact. Production first trust requires Enrollment's pin.
			decision := v.Evaluate(t.Context(), in)
			if _, ok := decision.Authorization(); !ok {
				t.Fatal(decision.Outcome, decision.Err())
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			v, err = release.OpenRetained(history)
			if err != nil {
				t.Fatal(err)
			}
			a, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, candidate, in)
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Check(t.Context(), v); err != nil {
				t.Fatal("original pair failed handoff", err)
			}
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			if a.Check(cancelled, v) == nil {
				t.Fatal("canceled handoff accepted")
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			if a.Check(t.Context(), v) == nil {
				t.Fatal("closed proof reused")
			}
			if err := os.RemoveAll(history); err != nil {
				t.Fatal(err)
			}
			if v, err := release.OpenRetained(history); err == nil {
				_ = v.Close()
				t.Fatal("lost trust recreated")
			}
			if _, err := os.Stat(history); !os.IsNotExist(err) {
				t.Fatal("retained opening created missing history", err)
			}
		})
	}
}

func TestPlatformRuntimeReleaseRefusesRollbackAfterReopen(t *testing.T) {
	authority := newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
	program := []byte("same signed runtime")
	load := func(version int64) enrollment.InfrastructureCandidate {
		dir, _, _ := infrastructureDistributionInvocation(t, t.TempDir(), program, authority, version, nil, "introduction", []byte(platformUnitTemplate))
		c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	old, current := load(1), load(2)
	history := filepath.Join(t.TempDir(), "release")
	v, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	in := infrastructureCandidateInput(t, current)
	if d := v.Evaluate(t.Context(), in); d.Err() != nil {
		t.Fatal(d.Outcome, d.Err())
	}
	if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, current, in); err != nil {
		t.Fatal(err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := infrastructure.AuthenticateRuntimeCandidate(t.Context(), v, old, infrastructureCandidateInput(t, old)); err == nil {
		t.Fatal("older signed generation authorized after reopen")
	}
	floors, err := v.CurrentFloors(t.Context())
	if err != nil || floors.TargetsVersion != 2 {
		t.Fatal("rollback changed retained floor", floors, err)
	}
}
