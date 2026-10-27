//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// This genuine signed artifact integration supplies no installed execution or
// native authority. Candidate trust is deliberately established by Release, not
// a forged initial Bundle or a Source receipt.
func TestForwardingArtifactCandidateFreshPairAndRoleSeparation(t *testing.T) {
	for _, scenario := range []string{"healthy", "missing-forwarding-target", "source-target", "mixed-policy", "substituted-template", "forged-metadata"} {
		t.Run(scenario, func(t *testing.T) {
			mutate := func(targets map[string]*metadata.TargetFiles) {
				const target = "ardents/linux-amd64/infrastructure/forwarding"
				switch scenario {
				case "missing-forwarding-target":
					delete(targets, target)
				case "source-target":
					targets["ardents/linux-amd64/infrastructure/source"] = targets[target]
					delete(targets, target)
				case "mixed-policy":
					var fields map[string]any
					if err := json.Unmarshal(*targets[target].Custom, &fields); err != nil {
						t.Fatal(err)
					}
					fields["attestation_policy"] = "another-policy"
					raw, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					custom := json.RawMessage(raw)
					targets[target].Custom = &custom
				}
			}
			directory, _, files := infrastructureDistributionInvocation(t, t.TempDir(), []byte("public program fixture"), nil, 1, mutate, "forwarding")
			if scenario == "substituted-template" || scenario == "forged-metadata" {
				name := "ardents-forwarding.service"
				if scenario == "forged-metadata" {
					name = "targets.json"
					files[name] = bytes.Replace(files[name], []byte("fixture-source"), []byte("forgery-source"), 1)
				} else {
					files[name] = []byte("substituted template")
				}
				if err := os.WriteFile(filepath.Join(directory, name), files[name], 0600); err != nil {
					t.Fatal(err)
				}
				writeInfrastructureOracleInvocationInventory(t, directory, files, 1, "forwarding")
			}
			candidate, err := enrollment.ReadInfrastructureCandidate(t.Context(), directory)
			if err != nil {
				t.Fatal(err)
			}
			input := infrastructureCandidateInput(t, candidate)
			verifier, err := release.Open(filepath.Join(t.TempDir(), "history"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := verifier.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := infrastructure.AuthenticateForwardingCandidate(t.Context(), verifier, candidate, input); err == nil {
				t.Fatal("cold candidate obtained initial trust")
			}
			if _, err := infrastructure.AuthenticateCandidate(t.Context(), verifier, candidate, input); err == nil {
				t.Fatal("Source admitted Forwarding inventory")
			}
			decision := verifier.Evaluate(t.Context(), input)
			if scenario == "forged-metadata" {
				if _, ok := decision.Authorization(); ok {
					t.Fatal("forged program metadata authorized")
				}
				return
			}
			if _, ok := decision.Authorization(); !ok {
				t.Fatal("healthy program counterpart", decision.Outcome, decision.Err())
			}
			authorization, err := infrastructure.AuthenticateForwardingCandidate(t.Context(), verifier, candidate, input)
			if scenario != "healthy" {
				if err == nil {
					t.Fatal("invalid Forwarding pair authorized")
				}
				p, g := authorization.Targets()
				for _, proof := range []release.Authorization{p, g} {
					if _, ok := proof.AcceptedDecision(); ok {
						t.Fatal("partial authorization escaped")
					}
				}
				return
			}
			if err != nil || authorization.Check(t.Context(), verifier) != nil {
				t.Fatal("healthy pair", err)
			}
			if _, ok := authorization.InitialFacts(); ok {
				t.Fatal("candidate fabricated initial provenance")
			}
			p, g := authorization.Targets()
			for _, proof := range []release.Authorization{p, g} {
				if d, ok := proof.AcceptedDecision(); !ok || d.Outcome != release.OutcomeNoUpdate {
					t.Fatal("fresh no-update proof absent")
				}
			}
			canceled, cancel := context.WithCancel(t.Context())
			cancel()
			if authorization.Check(canceled, verifier) == nil {
				t.Fatal("canceled handoff authorized")
			}
			if err := verifier.Close(); err != nil {
				t.Fatal(err)
			}
			if authorization.Check(t.Context(), verifier) == nil {
				t.Fatal("closed Release owner authorized")
			}
		})
	}
}
