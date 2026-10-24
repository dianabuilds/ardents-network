//go:build linux && network_signing_native

package main

import (
	"bytes"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// Deliberately hostile threshold-signed metadata exercises real admission.
// The maintained signer must refuse this plan. Only this fixture signs the
// mismatched policy; no signing command, installed authority or recovery is added.
func exerciseSourceReleaseCoherence(t *testing.T, input sourceTimestampNativeInput) {
	t.Helper()
	read := func(directory, name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	plan := input.plan
	var custom map[string]json.RawMessage
	if err := json.Unmarshal(plan.GenerationCustom, &custom); err != nil {
		t.Fatal(err)
	}
	var bound time.Time
	if err := json.Unmarshal(custom["build_safety_no_new_work_after"], &bound); err != nil {
		t.Fatal(err)
	}
	custom["build_safety_no_new_work_after"], _ = json.Marshal(bound.Add(time.Minute))
	plan.GenerationCustom, _ = json.Marshal(custom)
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(input.parent, "incoherent-plan")
	if err := os.WriteFile(planPath, planRaw, 0600); err != nil {
		t.Fatal(err)
	}
	output, diagnostic := filepath.Join(input.parent, "incoherent-signer-output"), filepath.Join(input.parent, "incoherent-signer-history")
	if raw, err := signSourceRelease(t.Context(), []string{planPath, input.keys, output, diagnostic}, sourceReleaseInitial, nil); err == nil || raw != nil {
		t.Fatal("maintained signer admitted mismatched target safety", err)
	}
	for _, path := range []string{output, diagnostic} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("incoherent plan created dependent output", path, err)
		}
	}
	targets, err := metadata.Targets().FromBytes(read(input.distribution, "targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	targets.Signed.Version++
	targets.Signatures = nil
	targets.Signed.Targets[infrastructure.GenerationTarget].Custom = &plan.GenerationCustom
	sign := func(value sourceSignableMetadata) []byte {
		t.Helper()
		for number := 1; number <= 3; number++ {
			raw := read(input.keys, fmt.Sprintf("release-key-%d.pem", number))
			key, err := parseSourceSigningKey(raw)
			clear(raw)
			if err != nil {
				t.Fatal(err)
			}
			signer, loadErr := signature.LoadSigner(key, crypto.Hash(0))
			if loadErr != nil {
				clear(key)
				t.Fatal(loadErr)
			}
			_, signErr := value.Sign(signer)
			clear(key)
			if signErr != nil {
				t.Fatal(signErr)
			}
		}
		raw, err := value.ToBytes(false)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	targetBytes := sign(targets)
	snapshot := metadata.Snapshot(input.plan.Expires)
	snapshot.Signed.Version = 2
	snapshot.Signed.Meta["targets.json"] = sourceMetadataReference(targetBytes)
	snapshot.Signed.Meta["targets.json"].Version = targets.Signed.Version
	snapshotBytes := sign(snapshot)
	timestamp := metadata.Timestamp(input.plan.Expires)
	timestamp.Signed.Version = 2
	timestamp.Signed.Meta["snapshot.json"] = sourceMetadataReference(snapshotBytes)
	timestamp.Signed.Meta["snapshot.json"].Version = snapshot.Signed.Version
	distribution := filepath.Join(input.parent, "incoherent-distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	roles := map[string][]byte{"targets.json": targetBytes, "snapshot.json": snapshotBytes, "timestamp.json": sign(timestamp)}
	for _, name := range []string{"ardents-node-linux-amd64", "infrastructure-source.json", "ardents-source.service", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, ok := roles[name]
		if !ok {
			raw = read(input.distribution, name)
		}
		if err := os.WriteFile(filepath.Join(distribution, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	inventory := filepath.Join(input.parent, "incoherent-inventory")
	if _, err := prepareSourceArtifact(t.Context(), []string{"prepare-inventory", distribution, inventory}, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(distribution, "infrastructure-enrollment.json"), read(inventory, "infrastructure-enrollment.json"), 0600); err != nil {
		t.Fatal(err)
	}
	history := filepath.Join(input.parent, "incoherent-retained-release")
	verifier, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	inputs := func(directory string) release.Inputs {
		t.Helper()
		files, err := release.StaticMetadataFiles(read(directory, "root.json"), read(directory, "timestamp.json"), read(directory, "snapshot.json"), read(directory, "targets.json"))
		if err != nil {
			t.Fatal(err)
		}
		return release.Inputs{RootBytes: read(directory, "root.json"), Files: files, Artifact: input.program, TargetPath: infrastructure.ProgramTarget,
			Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: input.plan.Network, RefTime: time.Now().UTC()}}
	}
	// Establish genuine pre-existing floors, then prove a healthy candidate is
	// admitted by the same public consumer used before canonical recovery.
	baseline := inputs(input.distribution)
	if _, ok := verifier.Evaluate(t.Context(), baseline).Authorization(); !ok {
		t.Fatal("original signed program did not establish retained floors")
	}
	if err := verifier.Close(); err != nil {
		t.Fatal(err)
	}
	verifier, err = release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	healthy, err := enrollment.ReadInfrastructureCandidate(t.Context(), input.distribution)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := infrastructure.AuthenticateCandidate(t.Context(), verifier, healthy, baseline); err != nil {
		t.Fatal("healthy signed counterpart refused", err)
	}
	bad, err := enrollment.ReadInfrastructureCandidate(t.Context(), distribution)
	if err != nil {
		t.Fatal(err)
	}
	request := inputs(distribution)
	var decisions []release.Decision
	for number, target := range []string{infrastructure.ProgramTarget, infrastructure.GenerationTarget} {
		request.TargetPath, request.Artifact = target, [][]byte{input.program, input.descriptor}[number]
		decision := verifier.Evaluate(t.Context(), request)
		if _, ok := decision.Authorization(); !ok {
			t.Fatal("hostile pair must have two individually genuine authorizations", target, decision.Outcome, decision.Err())
		}
		decisions = append(decisions, decision)
	}
	if decisions[0].BuildSafetyNoNewWorkAfter.Equal(decisions[1].BuildSafetyNoNewWorkAfter) {
		t.Fatal("fixture did not isolate target safety incoherence")
	}
	before, err := verifier.CurrentFloors(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	request.TargetPath, request.Artifact = infrastructure.ProgramTarget, input.program
	for attempt := 0; attempt < 2; attempt++ {
		authorization, err := infrastructure.AuthenticateCandidate(t.Context(), verifier, bad, request)
		if !errors.Is(err, infrastructure.ErrBinding) || authorization.Check(t.Context(), verifier) == nil {
			t.Fatal("genuine individually authorized incoherent pair admitted", attempt, err)
		}
		after, err := verifier.CurrentFloors(t.Context())
		a, aErr := json.Marshal(before)
		b, bErr := json.Marshal(after)
		if err != nil || aErr != nil || bErr != nil || !bytes.Equal(a, b) {
			t.Fatal("repeated pair refusal changed admitted floors", err, aErr, bErr)
		}
	}
	t.Log("genuine retained Release: healthy candidate admitted; two individually authorized threshold-signed targets with different safety bounds refused twice by AuthenticateCandidate; admitted floors retained; no recovery or installed execution")
}
