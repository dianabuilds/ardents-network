//go:build linux

package infrastructure_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestInfrastructureCandidateRequiresFloorsAndFreshCoherentPair(t *testing.T) {
	binary := []byte("signed candidate program fixture")
	for _, scenario := range []string{"healthy", "missing-generation", "mixed-release", "wrong-target", "wrong-resource", "forged-metadata"} {
		t.Run(scenario, func(t *testing.T) {
			mutate := func(targets map[string]*metadata.TargetFiles) {
				switch scenario {
				case "missing-generation":
					delete(targets, "ardents/linux-amd64/infrastructure/source")
				case "wrong-target":
					targets["ardents/linux-amd64/protected-endpoint"] = targets["ardents/linux-amd64/infrastructure/source"]
					delete(targets, "ardents/linux-amd64/infrastructure/source")
				case "mixed-release":
					g := targets["ardents/linux-amd64/infrastructure/source"]
					var fields map[string]any
					if err := json.Unmarshal(*g.Custom, &fields); err != nil {
						t.Fatal(err)
					}
					fields["release_version"] = 2
					raw, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					custom := json.RawMessage(raw)
					g.Custom = &custom
				}
			}
			dir, _, files := releasefixture.InfrastructureDistribution(t, t.TempDir(), binary, nil, 1, mutate, "source")
			if scenario == "wrong-resource" {
				files["ardents-source.service"] = []byte("substituted unit")
				if err := os.WriteFile(filepath.Join(dir, "ardents-source.service"), files["ardents-source.service"], 0600); err != nil {
					t.Fatal(err)
				}
				releasefixture.WriteInfrastructureOracleInvocationInventory(t, dir, files, 1, "source")
			}
			if scenario == "forged-metadata" {
				files["targets.json"] = bytes.Replace(files["targets.json"], []byte("fixture-source"), []byte("forgery-source"), 1)
				if err := os.WriteFile(filepath.Join(dir, "targets.json"), files["targets.json"], 0600); err != nil {
					t.Fatal(err)
				}
				releasefixture.WriteInfrastructureOracleInvocationInventory(t, dir, files, 1, "source")
			}
			c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			in := releasefixture.InfrastructureCandidateInputs(t, c)
			v, err := release.Open(filepath.Join(t.TempDir(), "history"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := v.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c, in); err == nil {
				t.Fatal("cold candidate acquired trust")
			}
			// Genuine Release establishes complete floors; candidate still needs two
			// new evaluations, and cannot obtain initial provenance from these floors.
			d := v.Evaluate(t.Context(), in)
			if scenario == "forged-metadata" {
				if _, ok := d.Authorization(); ok {
					t.Fatal("forged metadata authorized")
				}
				return
			}
			if _, ok := d.Authorization(); !ok {
				t.Fatalf("healthy program counterpart %s %v", d.Outcome, d.Err())
			}
			a, err := infrastructure.AuthenticateCandidate(t.Context(), v, c, in)
			if scenario != "healthy" {
				if err == nil {
					t.Fatal("invalid pair accepted")
				}
				p, g := a.Targets()
				if _, ok := p.AcceptedDecision(); ok {
					t.Fatal("partial program proof escaped")
				}
				if _, ok := g.AcceptedDecision(); ok {
					t.Fatal("partial generation proof escaped")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := a.InitialFacts(); ok {
				t.Fatal("candidate invented initial provenance")
			}
			p, g := a.Targets()
			for _, proof := range []release.Authorization{p, g} {
				d, ok := proof.AcceptedDecision()
				if !ok || d.Outcome != release.OutcomeNoUpdate {
					t.Fatal("fresh no-update missing")
				}
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c, in); err == nil {
				t.Fatal("closed verifier accepted")
			}
		})
	}
}

func TestInfrastructureRetainedReopenRollbackAndConflict(t *testing.T) {
	binary := []byte("signed retained program fixture")
	authority := releasefixture.NewReleaseAuthority(t, releasefixture.InfrastructureReference().Add(365*24*time.Hour))
	first, _, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), binary, authority, 1, nil, "source")
	second, _, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), binary, authority, 2, nil, "source", []byte("[Unit]\nDescription=Forward Source template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n"))
	conflict, _, _ := releasefixture.InfrastructureDistribution(t, t.TempDir(), binary, authority, 2, nil, "source", []byte("[Unit]\nDescription=Conflicting Source template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n"))
	load := func(dir string) (enrollment.InfrastructureCandidate, release.Inputs) {
		c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return c, releasefixture.InfrastructureCandidateInputs(t, c)
	}
	c1, in1 := load(first)
	c2, in2 := load(second)
	cc, ic := load(conflict)
	history := filepath.Join(t.TempDir(), "history")
	v, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	if d := v.Evaluate(t.Context(), in1); d.Outcome != release.OutcomeReleaseAccepted {
		t.Fatalf("actual bootstrap prerequisite %s %v", d.Outcome, d.Err())
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c1, in1); err != nil {
		t.Fatal("healthy first", err)
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c2, in2); err != nil {
		t.Fatal("forward candidate", err)
	}
	floor, err := v.CurrentFloors(t.Context())
	if err != nil || floor.TargetsVersion != 2 {
		t.Fatal("forward floor", err)
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c1, in1); err == nil {
		t.Fatal("rollback authorized")
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, cc, ic); err == nil {
		t.Fatal("same version conflict authorized")
	}
	after, err := v.CurrentFloors(t.Context())
	if err != nil || !bytes.Equal(floor.TargetsDigest, after.TargetsDigest) {
		t.Fatal("refusal changed floor", err)
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), v, c2, in2); err != nil {
		t.Fatal("retained no-update counterpart", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := infrastructure.AuthenticateCandidate(ctx, v, c2, in2); err == nil {
		t.Fatal("canceled original handoff")
	}
}
