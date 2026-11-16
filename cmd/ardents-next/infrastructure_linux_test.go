//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestInfrastructureCompiledRejectsOverlappingHistory(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	dir, _, _ := infrastructureDistribution(t, binary, nil, 1, nil)
	history := filepath.Join(dir, "release-history")
	cmd := exec.CommandContext(t.Context(), filepath.Join(dir, "ardents-node-linux-amd64"), "infrastructure", "authenticate-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339))
	if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("infrastructure-paths-refused")) {
		t.Fatalf("mutable history entered immutable inventory %s %v", out, err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("overlapping history mutated distribution", err)
	}
	for _, path := range []string{dir, filepath.Dir(dir), filepath.Join(dir, "absent"), "relative-history"} {
		if independentInfrastructureHistory(dir, path) {
			t.Fatal("overlap admitted", path)
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	if independentInfrastructureHistory(dir, filepath.Join(alias, "history")) {
		t.Fatal("symlink alias admitted")
	}
	if !independentInfrastructureHistory(dir, filepath.Join(t.TempDir(), "separate")) {
		t.Fatal("separate healthy history refused")
	}
}

func TestInfrastructureDetachedInitialCommandsRefuseBeforeEffects(t *testing.T) {
	for _, operation := range []string{"authenticate-initial", "preflight-initial"} {
		for _, retained := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/retained=%v", operation, retained), func(t *testing.T) {
				root := t.TempDir()
				history := filepath.Join(root, "history")
				marker := filepath.Join(history, "retained")
				if retained {
					if err := os.Mkdir(history, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(marker, []byte("original history"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"infrastructure", operation, filepath.Join(root, "distribution"), strings.Repeat("0", 64), history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)}
				if operation == "preflight-initial" {
					args = append(args, filepath.Join(root, "request"))
				}
				var out, diagnostic bytes.Buffer
				if code := run(t.Context(), args, &out, &diagnostic); code != 2 || out.Len() != 0 || diagnostic.Len() != 0 {
					t.Fatalf("retired command reached input/native/Release work: code=%d out=%q diagnostic=%q", code, out.String(), diagnostic.String())
				}
				if retained {
					entries, err := os.ReadDir(history)
					if err != nil || len(entries) != 1 || entries[0].Name() != "retained" {
						t.Fatal("retained history changed", err)
					}
					if raw, err := os.ReadFile(marker); err != nil || string(raw) != "original history" {
						t.Fatal("retained bytes changed", err)
					}
				} else if _, err := os.Lstat(history); !os.IsNotExist(err) {
					t.Fatal("retired command created history", err)
				}
			})
		}
	}
}

func TestInfrastructureCompiledCommandRetainedCandidate(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual compiled infrastructure consumer sha256=%x", sha256.Sum256(binary))
	dir, _, _ := infrastructureDistribution(t, binary, nil, 1, nil)
	history := filepath.Join(t.TempDir(), "history")
	retainSourceFixtureRelease(t, dir, history)
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), append([]string{"infrastructure"}, args...)...)
		return cmd.CombinedOutput()
	}
	candidate := []string{"authenticate-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)}
	if out, err := run(candidate...); err != nil || !bytes.Contains(out, []byte("authenticated-source-artifact")) {
		t.Fatalf("retained candidate %s %v", out, err)
	} else if !bytes.Contains(out, []byte(`"program_outcome":"no-update"`)) || !bytes.Contains(out, []byte(`"generation_outcome":"no-update"`)) {
		t.Fatalf("candidate consumer lacks two fresh no-update proofs %s", out)
	}
	cold := append([]string(nil), candidate...)
	cold[2] = filepath.Join(t.TempDir(), "missing-history")
	if out, err := run(cold...); err == nil {
		t.Fatalf("candidate cold trust %s", out)
	}
	if _, err := os.Lstat(cold[2]); !os.IsNotExist(err) {
		t.Fatal("cold candidate created history", err)
	}
	wrong := append([]string(nil), candidate...)
	wrong[3] = "foreign-environment"
	if out, err := run(wrong...); err == nil {
		t.Fatalf("wrong local %s %v", out, err)
	}
	wrong = append([]string(nil), candidate...)
	wrong[5] = infrastructureReference.Add(500 * 24 * time.Hour).Format(time.RFC3339)
	if out, err := run(wrong...); err == nil {
		t.Fatalf("expiry %s", out)
	}
}

func TestInfrastructureCandidateRequiresFloorsAndFreshCoherentPair(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
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
			dir, _, files := infrastructureDistribution(t, binary, nil, 1, mutate)
			if scenario == "wrong-resource" {
				files["ardents-source.service"] = []byte("substituted unit")
				if err := os.WriteFile(filepath.Join(dir, "ardents-source.service"), files["ardents-source.service"], 0600); err != nil {
					t.Fatal(err)
				}
				writeInfrastructureOracleInventory(t, dir, files, 1)
			}
			if scenario == "forged-metadata" {
				files["targets.json"] = bytes.Replace(files["targets.json"], []byte("fixture-source"), []byte("forgery-source"), 1)
				if err := os.WriteFile(filepath.Join(dir, "targets.json"), files["targets.json"], 0600); err != nil {
					t.Fatal(err)
				}
				writeInfrastructureOracleInventory(t, dir, files, 1)
			}
			c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
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
			if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c, in); err == nil {
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
			a, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c, in)
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
			if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c, in); err == nil {
				t.Fatal("closed verifier accepted")
			}
		})
	}
}

func TestInfrastructureInventoryPhysicalAndCanonicalRefusals(t *testing.T) {
	for _, scenario := range []string{"extra", "symlink", "hardlink", "noncanonical", "duplicate", "unknown", "invocation", "hash", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			dir, _, _ := infrastructureDistribution(t, []byte("candidate bytes"), nil, 1, nil)
			path := filepath.Join(dir, "ardents-source.service")
			switch scenario {
			case "extra":
				if err := os.WriteFile(filepath.Join(dir, "extra"), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("targets.json", path); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(t.TempDir(), "linked")); err != nil {
					t.Fatal(err)
				}
			case "hash":
				if err := os.WriteFile(path, []byte("substitute"), 0600); err != nil {
					t.Fatal(err)
				}
			case "noncanonical", "duplicate", "unknown", "invocation":
				p := filepath.Join(dir, "infrastructure-enrollment.json")
				raw, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "noncanonical":
					raw = append(raw, '\n')
				case "duplicate":
					raw = bytes.Replace(raw, []byte("\"release_version\":1"), []byte("\"release_version\":1,\"release_version\":1"), 1)
				case "unknown":
					raw = bytes.Replace(raw, []byte("\"schema\":"), []byte("\"extra\":0,\"schema\":"), 1)
				case "invocation":
					raw = bytes.Replace(raw, []byte("\"source\""), []byte("\"issuer\""), 1)
				}
				if err := os.WriteFile(p, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx := t.Context()
			if scenario == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := enrollment.ReadInfrastructureCandidate(ctx, dir); err == nil {
				t.Fatal("invalid inventory accepted")
			}
		})
	}
}

func TestInfrastructureRetainedReopenRollbackAndConflict(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	authority := newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
	first, _, _ := infrastructureDistribution(t, binary, authority, 1, nil)
	second, _, _ := infrastructureDistribution(t, binary, authority, 2, nil, []byte("[Unit]\nDescription=Forward Source template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n"))
	conflict, _, _ := infrastructureDistribution(t, binary, authority, 2, nil, []byte("[Unit]\nDescription=Conflicting Source template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n"))
	load := func(dir string) (enrollment.InfrastructureCandidate, release.Inputs) {
		c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return c, infrastructureCandidateInput(t, c)
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
	if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c1, in1); err != nil {
		t.Fatal("healthy first", err)
	}
	if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c2, in2); err != nil {
		t.Fatal("forward candidate", err)
	}
	floor, err := v.CurrentFloors(t.Context())
	if err != nil || floor.TargetsVersion != 2 {
		t.Fatal("forward floor", err)
	}
	if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c1, in1); err == nil {
		t.Fatal("rollback authorized")
	}
	if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, cc, ic); err == nil {
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
	if _, err := releaseartifact.AuthenticateCandidate(t.Context(), v, c2, in2); err != nil {
		t.Fatal("retained no-update counterpart", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := releaseartifact.AuthenticateCandidate(ctx, v, c2, in2); err == nil {
		t.Fatal("canceled original handoff")
	}
}

func TestInfrastructureDuplicateLifecycleRefusesBeforeEffects(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"start-source", filepath.Join(root, "distribution"), filepath.Join(root, "history"), "closed", "network", filepath.Join(root, "binding")},
		{"install-initial-stopped", filepath.Join(root, "distribution"), strings.Repeat("0", 64), filepath.Join(root, "history"), "closed", "network", "2030-01-02T03:04:05Z", filepath.Join(root, "request")},
	} {
		var output bytes.Buffer
		if code := runInfrastructure(t.Context(), args, &output); code != 2 || output.Len() != 0 {
			t.Fatalf("duplicate lifecycle accepted: %d %s", code, output.String())
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("retired lifecycle created files: %v %v", entries, err)
	}
}
