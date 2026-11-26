package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
)

func TestZeroAndDetachedFactsCannotCreateAuthority(t *testing.T) {
	if _, err := AuthenticateInitial(context.Background(), nil, enrollment.InfrastructureBundle{}, release.Inputs{}); err == nil {
		t.Fatal("zero initial accepted")
	}
	if _, err := AuthenticateCandidate(context.Background(), nil, enrollment.InfrastructureCandidate{}, release.Inputs{}); err == nil {
		t.Fatal("zero candidate accepted")
	}
	a := Authorization{descriptor: []byte("public"), executable: []byte("program"), template: []byte("template")}
	a.Descriptor()[0] ^= 1
	a.Program()[0] ^= 1
	a.Template()[0] ^= 1
	if string(a.Descriptor()) != "public" || string(a.Program()) != "program" || string(a.Template()) != "template" {
		t.Fatal("copies aliased")
	}
	p, g := a.Targets()
	for _, proof := range []release.Authorization{p, g} {
		if _, ok := proof.AcceptedDecision(); ok {
			t.Fatal("facts forged proof")
		}
	}
	if _, ok := a.InitialFacts(); ok {
		t.Fatal("facts forged initial provenance")
	}
}

func TestCoherenceIncludesCommonSafetyAndReference(t *testing.T) {
	d := Declaration{Platform: "linux-amd64", ReleaseIdentity: "one", ReleaseVersion: 1}
	p := release.Decision{Path: ProgramTarget, Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: 1, Floors: release.FloorSet{TargetsVersion: 1, TargetsDigest: make([]byte, 32)}}
	g := p
	g.Path = GenerationTarget
	if !coherentTarget(p, g, d, GenerationTarget) {
		t.Fatal("healthy counterpart")
	}
	for _, change := range []func(*release.Decision){func(g *release.Decision) { g.Network = "other" }, func(g *release.Decision) { g.AttestationPolicy = "other" }, func(g *release.Decision) { g.BuildState = "revoked" }, func(g *release.Decision) { g.SourceRevision = "other" }, func(g *release.Decision) { g.ReleaseVersion++ }, func(g *release.Decision) { g.Floors.TargetsVersion++ }, func(g *release.Decision) { g.Path = "ardents/linux-amd64/protected-endpoint" }} {
		other := g
		change(&other)
		if coherentTarget(p, other, d, GenerationTarget) {
			t.Fatal("incoherent pair accepted")
		}
	}
}

// Every role's candidate admission applies the full platform/target/arch/
// ref-time outer guard before observing retained trust, at the same depth as
// Join/Descriptor (G5, #772). A zero verifier refuses CurrentFloors with
// ErrClosed, so an ErrInput result proves the outer guard refused first;
// healthy inputs still reach trust observation unchanged.
func TestCandidateOuterGuardRefusesBeforeTrustObservation(t *testing.T) {
	roles := []struct {
		invocation, template, descriptor string
		authenticate                     func(context.Context, *release.Verifier, enrollment.InfrastructureCandidate, release.Inputs) error
	}{
		{"source", TemplateName, DescriptorName, func(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) error {
			_, err := AuthenticateCandidate(ctx, v, c, in)
			return err
		}},
		{"forwarding", ForwardingTemplateName, ForwardingDescriptorName, func(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) error {
			_, err := AuthenticateForwardingCandidate(ctx, v, c, in)
			return err
		}},
		{"join", JoinTemplateName, JoinDescriptorName, func(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) error {
			_, err := AuthenticateJoinCandidate(ctx, v, c, in)
			return err
		}},
		{"descriptor", DescriptorTemplateName, DescriptorArtifactName, func(ctx context.Context, v *release.Verifier, c enrollment.InfrastructureCandidate, in release.Inputs) error {
			_, err := AuthenticateDescriptorCandidate(ctx, v, c, in)
			return err
		}},
	}
	for _, role := range roles {
		t.Run(role.invocation, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{
				ProgramName:      []byte("node program"),
				role.template:    []byte("unit template"),
				role.descriptor:  []byte("descriptor"),
				"root.json":      []byte("root"),
				"snapshot.json":  []byte("snapshot"),
				"targets.json":   []byte("targets"),
				"timestamp.json": []byte("timestamp"),
			}
			facts := enrollment.InfrastructureFacts{Platform: "linux-amd64", ReleaseIdentity: "guard", ReleaseVersion: 1, Invocation: role.invocation}
			inventory, err := enrollment.PrepareInfrastructureInventory(facts, files)
			if err != nil {
				t.Fatal(err)
			}
			for name, raw := range files {
				if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, enrollment.InfrastructureInventoryName), inventory, 0600); err != nil {
				t.Fatal(err)
			}
			c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			v := &release.Verifier{}
			healthy := release.Inputs{TargetPath: ProgramTarget, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", RefTime: time.Unix(1700000000, 0).UTC()}}
			if err := role.authenticate(t.Context(), v, c, healthy); !errors.Is(err, release.ErrClosed) {
				t.Fatalf("healthy input did not reach trust observation: %v", err)
			}
			for _, mutate := range []struct {
				name   string
				mutate func(*release.Inputs)
			}{
				{"wrong target", func(in *release.Inputs) { in.TargetPath = "ardents/linux-amd64/other" }},
				{"wrong platform", func(in *release.Inputs) { in.Local.Platform = "windows-amd64" }},
				{"wrong architecture", func(in *release.Inputs) { in.Local.Architecture = "arm64" }},
				{"zero reference time", func(in *release.Inputs) { in.Local.RefTime = time.Time{} }},
			} {
				in := healthy
				mutate.mutate(&in)
				err := role.authenticate(t.Context(), v, c, in)
				if !errors.Is(err, ErrInput) || errors.Is(err, release.ErrClosed) {
					t.Fatalf("%s: outer guard did not refuse before trust observation: %v", mutate.name, err)
				}
			}
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			if err := role.authenticate(cancelled, v, c, healthy); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled context not refused by the outer guard: %v", err)
			}
		})
	}
}

func TestSourceReleaseHistoryIsIndependent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "distribution")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, filepath.Dir(dir), filepath.Join(dir, "absent"), "relative-history"} {
		if independentSourceReleaseHistory(dir, path) {
			t.Fatal("overlap admitted", path)
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	if independentSourceReleaseHistory(dir, filepath.Join(alias, "history")) {
		t.Fatal("symlink alias admitted")
	}
	if !independentSourceReleaseHistory(dir, filepath.Join(t.TempDir(), "separate")) {
		t.Fatal("separate healthy history refused")
	}
}
