package endpoint

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestAuthenticateSuccessorRequiresOriginalVerifier(t *testing.T) {
	for _, authenticate := range []func(context.Context, *release.Verifier, enrollment.Candidate, release.Inputs, RetainedTarget) (Authorization, error){AuthenticateSuccessor, AuthenticateRetainedCandidate} {
		for _, ctx := range []context.Context{nil, context.Background()} {
			result, err := authenticate(ctx, nil, enrollment.Candidate{}, release.Inputs{}, RetainedTarget{})
			if !errors.Is(err, ErrInput) || len(result.Descriptor()) != 0 {
				t.Fatalf("missing original verifier accepted: %v", err)
			}
		}
	}
}

func TestAuthenticateSuccessorDoesNotBootstrapIncompleteTrust(t *testing.T) {
	verifier, err := release.Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = verifier.Close() })
	previous := RetainedTarget{TargetsVersion: 1, TargetsDigest: strings.Repeat("01", 32), ReleaseVersion: 1}
	for _, authenticate := range []func(context.Context, *release.Verifier, enrollment.Candidate, release.Inputs, RetainedTarget) (Authorization, error){AuthenticateSuccessor, AuthenticateRetainedCandidate} {
		if _, err := authenticate(t.Context(), verifier, enrollment.Candidate{}, release.Inputs{}, previous); !errors.Is(err, release.ErrTrustUnavailable) {
			t.Fatalf("incomplete trust admitted candidate: %v", err)
		}
	}
	floors, err := verifier.CurrentFloors(t.Context())
	if err != nil || floors.RootVersion != 0 || floors.TargetsVersion != 0 {
		t.Fatalf("refusal advanced trust floors: %+v, %v", floors, err)
	}
}

func TestRetainedContinuityRejectsRollbackConflictAndForeignEnvironment(t *testing.T) {
	reference := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	previous := RetainedTarget{Platform: "linux-amd64", Architecture: "amd64", Environment: "closed", Network: "network", ReferenceTime: reference.Format(time.RFC3339Nano), TargetsVersion: 3, TargetsDigest: strings.Repeat("09", 32)}
	floors := release.FloorSet{RootVersion: 1, RootDigest: bytes.Repeat([]byte{1}, 32), TimestampVersion: 1, TimestampDigest: bytes.Repeat([]byte{2}, 32), SnapshotVersion: 1, SnapshotDigest: bytes.Repeat([]byte{3}, 32), TargetsVersion: 3, TargetsDigest: bytes.Repeat([]byte{9}, 32)}
	local := release.LocalEnvironment{Platform: previous.Platform, Architecture: previous.Architecture, Environment: previous.Environment, Network: previous.Network, RefTime: reference}
	cases := []struct {
		name   string
		change func(*RetainedTarget, *release.FloorSet, *release.LocalEnvironment)
		want   error
	}{
		{"retained", func(*RetainedTarget, *release.FloorSet, *release.LocalEnvironment) {}, nil},
		{"newer metadata", func(_ *RetainedTarget, f *release.FloorSet, _ *release.LocalEnvironment) {
			f.TargetsVersion++
			f.TargetsDigest = bytes.Repeat([]byte{4}, 32)
		}, nil},
		{"rollback", func(_ *RetainedTarget, f *release.FloorSet, _ *release.LocalEnvironment) { f.TargetsVersion-- }, release.ErrTrustUnavailable},
		{"conflict", func(_ *RetainedTarget, f *release.FloorSet, _ *release.LocalEnvironment) {
			f.TargetsDigest = bytes.Repeat([]byte{4}, 32)
		}, release.ErrTrustUnavailable},
		{"torn history", func(_ *RetainedTarget, f *release.FloorSet, _ *release.LocalEnvironment) { f.SnapshotDigest = nil }, release.ErrTrustUnavailable},
		{"noncanonical digest", func(p *RetainedTarget, _ *release.FloorSet, _ *release.LocalEnvironment) {
			p.TargetsDigest = strings.Repeat("AA", 32)
		}, release.ErrTrustUnavailable},
		{"foreign platform", func(_ *RetainedTarget, _ *release.FloorSet, l *release.LocalEnvironment) {
			l.Platform = "darwin-arm64"
		}, ErrBinding},
		{"foreign network", func(_ *RetainedTarget, _ *release.FloorSet, l *release.LocalEnvironment) { l.Network = "other" }, ErrBinding},
		{"earlier clock", func(_ *RetainedTarget, _ *release.FloorSet, l *release.LocalEnvironment) {
			l.RefTime = l.RefTime.Add(-time.Nanosecond)
		}, ErrBinding},
		{"invalid reference", func(p *RetainedTarget, _ *release.FloorSet, _ *release.LocalEnvironment) {
			p.ReferenceTime = "invalid"
		}, ErrBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, f, l := previous, floors, local
			tc.change(&p, &f, &l)
			if err := CheckContinuity(p, f, l); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
