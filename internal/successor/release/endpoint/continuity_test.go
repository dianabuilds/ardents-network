package endpoint

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestRetainedContinuityRejectsRollbackConflictAndForeignEnvironment(t *testing.T) {
	reference := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	previous := RetainedGeneration{Platform: "linux-amd64", Architecture: "amd64", Environment: "closed", Network: "network", ReferenceTime: reference.Format(time.RFC3339Nano), TargetsVersion: 3, TargetsDigest: strings.Repeat("09", 32)}
	floors := release.FloorSet{RootVersion: 1, RootDigest: bytes.Repeat([]byte{1}, 32), TimestampVersion: 1, TimestampDigest: bytes.Repeat([]byte{2}, 32), SnapshotVersion: 1, SnapshotDigest: bytes.Repeat([]byte{3}, 32), TargetsVersion: 3, TargetsDigest: bytes.Repeat([]byte{9}, 32)}
	local := release.LocalEnvironment{Platform: previous.Platform, Architecture: previous.Architecture, Environment: previous.Environment, Network: previous.Network, RefTime: reference}
	cases := []struct {
		name   string
		change func(*RetainedGeneration, *release.FloorSet, *release.LocalEnvironment)
		want   error
	}{
		{"retained", func(*RetainedGeneration, *release.FloorSet, *release.LocalEnvironment) {}, nil},
		{"newer metadata", func(_ *RetainedGeneration, f *release.FloorSet, _ *release.LocalEnvironment) {
			f.TargetsVersion++
			f.TargetsDigest = bytes.Repeat([]byte{4}, 32)
		}, nil},
		{"rollback", func(_ *RetainedGeneration, f *release.FloorSet, _ *release.LocalEnvironment) { f.TargetsVersion-- }, release.ErrTrustUnavailable},
		{"conflict", func(_ *RetainedGeneration, f *release.FloorSet, _ *release.LocalEnvironment) {
			f.TargetsDigest = bytes.Repeat([]byte{4}, 32)
		}, release.ErrTrustUnavailable},
		{"torn history", func(_ *RetainedGeneration, f *release.FloorSet, _ *release.LocalEnvironment) { f.SnapshotDigest = nil }, release.ErrTrustUnavailable},
		{"noncanonical digest", func(p *RetainedGeneration, _ *release.FloorSet, _ *release.LocalEnvironment) {
			p.TargetsDigest = strings.Repeat("AA", 32)
		}, release.ErrTrustUnavailable},
		{"foreign platform", func(_ *RetainedGeneration, _ *release.FloorSet, l *release.LocalEnvironment) {
			l.Platform = "darwin-arm64"
		}, ErrBinding},
		{"foreign network", func(_ *RetainedGeneration, _ *release.FloorSet, l *release.LocalEnvironment) { l.Network = "other" }, ErrBinding},
		{"earlier clock", func(_ *RetainedGeneration, _ *release.FloorSet, l *release.LocalEnvironment) {
			l.RefTime = l.RefTime.Add(-time.Nanosecond)
		}, ErrBinding},
		{"invalid reference", func(p *RetainedGeneration, _ *release.FloorSet, _ *release.LocalEnvironment) {
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
