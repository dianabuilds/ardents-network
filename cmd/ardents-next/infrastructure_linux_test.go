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

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestInfrastructureCompiledRejectsOverlappingHistory(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	dir, pin, _ := infrastructureDistribution(t, binary, nil, 1, nil)
	history := filepath.Join(dir, "release-history")
	cmd := exec.CommandContext(t.Context(), filepath.Join(dir, "ardents-node-linux-amd64"), "infrastructure", "authenticate-initial", dir, pin, history, "h3-test", "new-release-test", nodefixture.InfrastructureReference().Format(time.RFC3339))
	if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("infrastructure-paths-refused")) {
		t.Fatalf("mutable history entered immutable inventory %s %v", out, err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("overlapping history mutated distribution", err)
	}
}

func TestInfrastructureCompiledRejectsRetiredSourceOperations(t *testing.T) {
	program := compiledCommand(t)
	for _, operation := range []string{"preflight-initial", "preflight-candidate", "install-initial-stopped"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent")
			cmd := exec.CommandContext(t.Context(), program, "infrastructure", operation, path)
			out, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 || len(out) != 0 {
				t.Fatalf("retired operation returned %q, %v", out, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("retired operation changed input path", err)
			}
		})
	}
}

// Independent oracle: literal ordered schemas, independently spelled names and
// ordinary 3-of-5 signatures, without product preparation/descriptor codecs.
func infrastructureDistribution(t *testing.T, binary []byte, authority *nodefixture.ReleaseAuthority, version int64, mutate func(map[string]*metadata.TargetFiles), unitOverride ...[]byte) (string, string, map[string][]byte) {
	t.Helper()
	return infrastructureDistributionRoot(t, t.TempDir(), binary, authority, version, mutate, unitOverride...)
}

func infrastructureDistributionRoot(t *testing.T, parent string, binary []byte, authority *nodefixture.ReleaseAuthority, version int64, mutate func(map[string]*metadata.TargetFiles), unitOverride ...[]byte) (string, string, map[string][]byte) {
	return nodefixture.InfrastructureDistribution(t, parent, binary, authority, version, mutate, "source", unitOverride...)
}

// Only public signed test artifacts share this oracle; no runtime, native
// installation policy, private root or accepting role dispatcher is shared.
func writeInfrastructureOracleInventory(t *testing.T, dir string, files map[string][]byte, version int64) string {
	return nodefixture.WriteInfrastructureOracleInvocationInventory(t, dir, files, version, "source")
}

func TestInfrastructureCompiledCommandInitialCandidateAndForeignArtifact(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual compiled infrastructure consumer sha256=%x", sha256.Sum256(binary))
	dir, pin, files := infrastructureDistribution(t, binary, nil, 1, nil)
	program := filepath.Join(dir, "ardents-node-linux-amd64")
	history := filepath.Join(t.TempDir(), "history")
	run := func(executable string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), executable, append([]string{"infrastructure"}, args...)...)
		return cmd.CombinedOutput()
	}
	wantReceipt := func(programOutcome string) []byte {
		return fmt.Appendf(nil, "{\"outcome\":\"authenticated-source-artifact\",\"program_outcome\":%q,\"generation_outcome\":\"no-update\",\"program_sha256\":\"%x\",\"descriptor_sha256\":\"%x\",\"template_sha256\":\"%x\"}\n",
			programOutcome, sha256.Sum256(binary), sha256.Sum256(files["infrastructure-source.json"]), sha256.Sum256(files["ardents-source.service"]))
	}

	args := []string{"authenticate-initial", dir, pin, history, "h3-test", "new-release-test", nodefixture.InfrastructureReference().Format(time.RFC3339)}
	if out, err := run(compiledCommand(t), args...); err == nil || !bytes.Contains(out, []byte("inventory-refused")) {
		t.Fatalf("foreign actual executable accepted %s %v", out, err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("foreign artifact mutated history")
	}
	if out, err := run(program, args...); err != nil || !bytes.Equal(out, wantReceipt("release-accepted")) {
		t.Fatalf("actual self initial %s %v", out, err)
	}
	if out, err := run(program, args...); err == nil {
		t.Fatalf("initial pin reused for retained history %s", out)
	}
	candidate := []string{"authenticate-candidate", dir, history, "h3-test", "new-release-test", nodefixture.InfrastructureReference().Format(time.RFC3339)}
	if out, err := run(compiledCommand(t), candidate...); err != nil || !bytes.Equal(out, wantReceipt("no-update")) {
		t.Fatalf("retained candidate %s %v", out, err)
	}
	cold := append([]string(nil), candidate...)
	cold[2] = filepath.Join(t.TempDir(), "missing-history")
	if out, err := run(program, cold...); err == nil {
		t.Fatalf("candidate cold trust %s", out)
	}
	wrong := append([]string(nil), args...)
	wrong[2] = strings.Repeat("0", 64)
	wrong[3] = filepath.Join(t.TempDir(), "wrong-pin-history")
	if out, err := run(program, wrong...); err == nil {
		t.Fatalf("wrong pin %s", out)
	}
	if _, err := os.Stat(wrong[3]); !os.IsNotExist(err) {
		t.Fatal("wrong pin mutated history")
	}
	wrong = append([]string(nil), args...)
	wrong[4] = "foreign-environment"
	if out, err := run(program, wrong...); err == nil {
		t.Fatalf("wrong local %s", out)
	}
	wrong = append([]string(nil), args...)
	wrong[6] = nodefixture.InfrastructureReference().Add(500 * 24 * time.Hour).Format(time.RFC3339)
	if out, err := run(program, wrong...); err == nil {
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
			in := nodefixture.InfrastructureCandidateInputs(t, c)
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
	authority := nodefixture.NewReleaseAuthority(t, nodefixture.InfrastructureReference().Add(365*24*time.Hour))
	first, _, _ := infrastructureDistribution(t, binary, authority, 1, nil)
	second, _, _ := infrastructureDistribution(t, binary, authority, 2, nil, []byte("[Unit]\nDescription=Forward Source template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n"))
	conflict, _, _ := infrastructureDistribution(t, binary, authority, 2, nil, []byte("[Unit]\nDescription=Conflicting Source template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n"))
	load := func(dir string) (enrollment.InfrastructureCandidate, release.Inputs) {
		c, err := enrollment.ReadInfrastructureCandidate(t.Context(), dir)
		if err != nil {
			t.Fatal(err)
		}
		return c, nodefixture.InfrastructureCandidateInputs(t, c)
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
