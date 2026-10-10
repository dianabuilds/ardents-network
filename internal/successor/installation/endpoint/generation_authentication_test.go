package endpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Failure-only public-boundary checks provide no successful authority fixture.
// Genuine signed-byte/owned-history success and refusal are exercised by the
// production command integration tests in cmd/ardents-next.
func TestAssemblyCannotUseStoredObservationsAsFreshAuthority(t *testing.T) {
	_, binding, files := installedObservationFixture(t)
	request, err := DecodeRequest(t.Context(), files["request.json"], true)
	if err != nil {
		t.Fatal(err)
	}
	// A complete, coherent stored binding and exact static bytes still have no
	// private Release proof. Numeric root facts here are public observations,
	// not a successful native preparation or an installed positive fixture.
	publicProgram, err := binding.Program.observation()
	if err != nil {
		t.Fatal(err)
	}
	publicGeneration, err := binding.Generation.observation()
	if err != nil {
		t.Fatal(err)
	}
	if !generationauthorization.CoherentTargets(publicProgram, publicGeneration, generationDeclaration{
		Platform: "linux-amd64", ReleaseIdentity: "release", ReleaseVersion: 1,
	}) {
		t.Fatal("stored fixture is not coherent")
	}
	var descriptor generationDeclaration
	if err := json.Unmarshal(files["protected-endpoint.json"], &descriptor); err != nil {
		t.Fatal(err)
	}
	resources := make(map[string][]byte, len(descriptor.Files))
	for name := range descriptor.Files {
		resources[name] = files[name]
	}
	if err := enrollment.ValidateProtectedGeneration(files["protected-endpoint.json"], resources, "release"); err != nil {
		t.Fatal(err)
	}
	// Detached stored bytes cannot populate the separate owner's private pair.
	observed := Authorization{}
	prepared := preparedInstallation{uid: binding.UID, gid: binding.GID, roots: binding.MutableRoots}
	assembled, selected, err := assembleGeneration(t.Context(), request, observed, prepared)
	if !errors.Is(err, ErrAuthorization) || assembled != nil || selected != (generationSelection{}) {
		t.Fatalf("stored observations became fresh assembly authority: %v", err)
	}
}

// These public observations exercise continuity rules only. Passing them grants
// no fresh proof, native predecessor custody or successful transition.
func TestSuccessorContinuityRetainsInstalledHistoryAndEnvironment(t *testing.T) {
	_, binding, _ := installedObservationFixture(t)
	binding.Generation.TargetsVersion = 3
	binding.Program.TargetsVersion = 3
	reference, err := time.Parse(time.RFC3339Nano, binding.Generation.ReferenceTime)
	if err != nil {
		t.Fatal(err)
	}
	floors := release.FloorSet{RootVersion: 1, RootDigest: bytes.Repeat([]byte{1}, 32),
		TimestampVersion: 1, TimestampDigest: bytes.Repeat([]byte{2}, 32),
		SnapshotVersion: 1, SnapshotDigest: bytes.Repeat([]byte{3}, 32),
		TargetsVersion: 3, TargetsDigest: bytes.Repeat([]byte{9}, 32)}
	local := release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "closed", Network: binding.Generation.Network, RefTime: reference}
	cases := []struct {
		name   string
		change func(*release.FloorSet, *release.LocalEnvironment)
		want   error
	}{
		{"equal retained history", func(*release.FloorSet, *release.LocalEnvironment) {}, nil},
		{"newer metadata", func(f *release.FloorSet, _ *release.LocalEnvironment) {
			f.TargetsVersion++
			f.TargetsDigest = bytes.Repeat([]byte{4}, 32)
		}, nil},
		{"lower complete history", func(f *release.FloorSet, _ *release.LocalEnvironment) { f.TargetsVersion-- }, release.ErrTrustUnavailable},
		{"equal conflicting history", func(f *release.FloorSet, _ *release.LocalEnvironment) { f.TargetsDigest = bytes.Repeat([]byte{4}, 32) }, release.ErrTrustUnavailable},
		{"lost root", func(f *release.FloorSet, _ *release.LocalEnvironment) { f.RootVersion = 0 }, release.ErrTrustUnavailable},
		{"lost timestamp", func(f *release.FloorSet, _ *release.LocalEnvironment) { f.TimestampDigest = nil }, release.ErrTrustUnavailable},
		{"lost snapshot", func(f *release.FloorSet, _ *release.LocalEnvironment) { f.SnapshotVersion = 0 }, release.ErrTrustUnavailable},
		{"torn targets", func(f *release.FloorSet, _ *release.LocalEnvironment) { f.TargetsDigest = f.TargetsDigest[:31] }, release.ErrTrustUnavailable},
		{"changed platform", func(_ *release.FloorSet, l *release.LocalEnvironment) { l.Platform = "windows-amd64" }, ErrBinding},
		{"changed architecture", func(_ *release.FloorSet, l *release.LocalEnvironment) { l.Architecture = "arm64" }, ErrBinding},
		{"changed environment", func(_ *release.FloorSet, l *release.LocalEnvironment) { l.Environment = "other" }, ErrBinding},
		{"changed Network", func(_ *release.FloorSet, l *release.LocalEnvironment) { l.Network = "other" }, ErrBinding},
		{"earlier reference", func(_ *release.FloorSet, l *release.LocalEnvironment) { l.RefTime = l.RefTime.Add(-time.Nanosecond) }, ErrBinding},
		{"absent reference", func(_ *release.FloorSet, l *release.LocalEnvironment) { l.RefTime = time.Time{} }, ErrBinding},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			observed, environment := floors, local
			test.change(&observed, &environment)
			if err := successorContinuity(binding, observed, environment); !errors.Is(err, test.want) {
				t.Fatalf("continuity: got %v, want %v", err, test.want)
			}
		})
	}
}
