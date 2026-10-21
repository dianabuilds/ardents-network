//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

var infrastructureReference = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

func TestInfrastructureCompiledRejectsOverlappingHistory(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	dir, pin, _ := infrastructureDistribution(t, binary, nil, 1, nil)
	history := filepath.Join(dir, "release-history")
	cmd := exec.CommandContext(t.Context(), filepath.Join(dir, "ardents-node-linux-amd64"), "infrastructure", "authenticate-initial", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339))
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

// Independent oracle: literal ordered schemas, independently spelled names and
// ordinary 3-of-5 signatures, without product preparation/descriptor codecs.
func infrastructureDistribution(t *testing.T, binary []byte, authority *consumerReleaseAuthority, version int64, mutate func(map[string]*metadata.TargetFiles), unitOverride ...[]byte) (string, string, map[string][]byte) {
	t.Helper()
	return infrastructureDistributionRoot(t, t.TempDir(), binary, authority, version, mutate, unitOverride...)
}

func infrastructureDistributionRoot(t *testing.T, parent string, binary []byte, authority *consumerReleaseAuthority, version int64, mutate func(map[string]*metadata.TargetFiles), unitOverride ...[]byte) (string, string, map[string][]byte) {
	t.Helper()
	expires := infrastructureReference.Add(365 * 24 * time.Hour)
	if authority == nil {
		authority = newConsumerReleaseAuthority(t, expires)
	}
	unit := []byte("[Unit]\nDescription=Source public template fixture\n[Service]\nType=exec\nRestart=no\nKillMode=control-group\n")
	if len(unitOverride) == 1 {
		unit = unitOverride[0]
	}
	descriptor := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"infrastructure-fixture\",\"release_version\":%d,\"invocation\":\"source\",\"files\":{\"ardents-node-linux-amd64\":\"%x\",\"ardents-source.service\":\"%x\"}}\n", version, sha256.Sum256(binary), sha256.Sum256(unit)))
	targets := metadata.Targets(expires)
	targets.Signed.Version = version
	for target, body := range map[string][]byte{"ardents/linux-amd64/node": binary, "ardents/linux-amd64/infrastructure/source": descriptor} {
		sum := sha256.Sum256(body)
		attestations := []map[string]string{}
		for _, builder := range []string{"fixture-builder-one", "fixture-builder-two"} {
			attestations = append(attestations, map[string]string{"builder_identity": builder, "build_identity": "fixture-build", "source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "target_sha256": hex.EncodeToString(sum[:])})
		}
		fields := map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": "linux-amd64", "architecture": "amd64", "environment": "h3-test", "network": "new-release-test", "release_identity": "infrastructure-fixture", "release_version": version, "source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "build_identity": "fixture-build", "dependency_identity": "fixture-dependencies", "sbom_identity": "fixture-sbom", "attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "protocol_phase": "announced", "build_safety_no_new_work_after": infrastructureReference.Add(30 * 24 * time.Hour), "build_safety_terminate_after": infrastructureReference.Add(180 * 24 * time.Hour), "builder_attestations": attestations}
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		custom := json.RawMessage(raw)
		targets.Signed.Targets[target] = &metadata.TargetFiles{Path: target, Length: int64(len(body)), Hashes: metadata.Hashes{"sha256": sum[:]}, Custom: &custom}
	}
	if mutate != nil {
		mutate(targets.Signed.Targets)
	}
	for _, s := range authority.signers[:3] {
		if _, err := targets.Sign(s); err != nil {
			t.Fatal(err)
		}
	}
	tb, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	td := sha256.Sum256(tb)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Version = version
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(tb)), Hashes: metadata.Hashes{"sha256": td[:]}}
	for _, s := range authority.signers[:3] {
		if _, err := snapshot.Sign(s); err != nil {
			t.Fatal(err)
		}
	}
	sb, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	sd := sha256.Sum256(sb)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Version = version
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: version, Length: int64(len(sb)), Hashes: metadata.Hashes{"sha256": sd[:]}}
	for _, s := range authority.signers[:3] {
		if _, err := timestamp.Sign(s); err != nil {
			t.Fatal(err)
		}
	}
	ts, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": binary, "ardents-source.service": unit, "infrastructure-source.json": descriptor, "root.json": authority.rootBytes, "timestamp.json": ts, "snapshot.json": sb, "targets.json": tb}
	directory := filepath.Join(parent, "distribution")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(directory, "ardents-node-linux-amd64"), 0700); err != nil {
		t.Fatal(err)
	}
	pin := writeInfrastructureOracleInventory(t, directory, files, version)
	return directory, pin, files
}

func writeInfrastructureOracleInventory(t *testing.T, dir string, files map[string][]byte, version int64) string {
	t.Helper()
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := []string{}
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		pairs = append(pairs, fmt.Sprintf("%q:%q", name, hex.EncodeToString(sum[:])))
	}
	raw := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-enrollment-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"infrastructure-fixture\",\"release_version\":%d,\"invocation\":\"source\",\"files\":{%s}}\n", version, strings.Join(pairs, ",")))
	if err := os.WriteFile(filepath.Join(dir, "infrastructure-enrollment.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func infrastructureCandidateInput(t *testing.T, c enrollment.InfrastructureCandidate) release.Inputs {
	t.Helper()
	root, _ := c.File("root.json")
	ts, _ := c.File("timestamp.json")
	sn, _ := c.File("snapshot.json")
	targets, _ := c.File("targets.json")
	program, _ := c.File("ardents-node-linux-amd64")
	files, err := release.StaticMetadataFiles(root, ts, sn, targets)
	if err != nil {
		t.Fatal(err)
	}
	return release.Inputs{RootBytes: root, Files: files, Artifact: program, TargetPath: "ardents/linux-amd64/node", Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "h3-test", Network: "new-release-test", RefTime: infrastructureReference}}
}

func TestInfrastructureCompiledCommandInitialCandidateAndForeignArtifact(t *testing.T) {
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("actual compiled infrastructure consumer sha256=%x", sha256.Sum256(binary))
	dir, pin, _ := infrastructureDistribution(t, binary, nil, 1, nil)
	program := filepath.Join(dir, "ardents-node-linux-amd64")
	history := filepath.Join(t.TempDir(), "history")
	run := func(executable string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(t.Context(), executable, append([]string{"infrastructure"}, args...)...)
		return cmd.CombinedOutput()
	}
	args := []string{"authenticate-initial", dir, pin, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)}
	if out, err := run(compiledCommand(t), args...); err == nil || !bytes.Contains(out, []byte("inventory-refused")) {
		t.Fatalf("foreign actual executable accepted %s %v", out, err)
	}
	if _, err := os.Stat(history); !os.IsNotExist(err) {
		t.Fatal("foreign artifact mutated history")
	}
	if out, err := run(program, args...); err != nil || !bytes.Contains(out, []byte("authenticated-source-artifact")) {
		t.Fatalf("actual self initial %s %v", out, err)
	} else if !bytes.Contains(out, []byte(`"program_outcome":"release-accepted"`)) || !bytes.Contains(out, []byte(`"generation_outcome":"no-update"`)) {
		t.Fatalf("initial consumer lacks fresh second no-update proof %s", out)
	}
	if out, err := run(program, args...); err == nil {
		t.Fatalf("initial pin reused for retained history %s", out)
	}
	candidate := []string{"authenticate-candidate", dir, history, "h3-test", "new-release-test", infrastructureReference.Format(time.RFC3339)}
	if out, err := run(compiledCommand(t), candidate...); err != nil || !bytes.Contains(out, []byte("authenticated-source-artifact")) {
		t.Fatalf("retained candidate %s %v", out, err)
	} else if !bytes.Contains(out, []byte(`"program_outcome":"no-update"`)) || !bytes.Contains(out, []byte(`"generation_outcome":"no-update"`)) {
		t.Fatalf("candidate consumer lacks two fresh no-update proofs %s", out)
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
	wrong[6] = infrastructureReference.Add(500 * 24 * time.Hour).Format(time.RFC3339)
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
