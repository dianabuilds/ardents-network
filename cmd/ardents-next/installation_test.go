package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestInstallationFreshPairAndCandidateUseGenuineOwners(t *testing.T) {
	directory, pin, program := signedConsumerTargets(t, "linux-amd64", true, true, 1)
	bundle, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: directory, ExecutablePath: program, ManifestSHA256: pin, Scope: enrollment.Headless})
	if err != nil {
		t.Fatal(err)
	}
	ref := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	in, ok := initialReleaseInputs(bundle, ref)
	if !ok {
		t.Fatal("initial composition refused")
	}
	history := filepath.Join(t.TempDir(), "history")
	v, err := release.Open(history)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if v != nil {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	initial, err := installation.AuthenticateInitial(t.Context(), v, bundle, in)
	if err != nil {
		_ = v.Close()
		t.Fatal(err)
	}
	p, g := initial.Targets()
	programDecision, pOK := p.AcceptedDecision()
	generationDecision, gOK := g.AcceptedDecision()
	if !pOK || !gOK || programDecision.Outcome != release.OutcomeReleaseAccepted || generationDecision.Outcome != release.OutcomeNoUpdate ||
		programDecision.Floors.TargetsVersion != generationDecision.Floors.TargetsVersion || !bytes.Equal(programDecision.Floors.TargetsDigest, generationDecision.Floors.TargetsDigest) {
		t.Fatal("fresh generation proof missing after shared floors committed")
	}
	programDecision.Digest[0] ^= 1
	resource, ok := initial.Resource("ardents-linux-amd64")
	if !ok {
		t.Fatal("program resource missing")
	}
	resource[0] ^= 1
	descriptor := initial.Descriptor()
	descriptor[0] ^= 1
	actualResource, _ := initial.Resource("ardents-linux-amd64")
	actualDescriptor := initial.Descriptor()
	resourceDigest, descriptorDigest := sha256.Sum256(actualResource), sha256.Sum256(actualDescriptor)
	programDecision, _ = p.AcceptedDecision()
	if !bytes.Equal(programDecision.Digest, resourceDigest[:]) || !bytes.Equal(generationDecision.Digest, descriptorDigest[:]) {
		t.Fatal("authorization or generation bytes alias exported copies")
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	// A fresh candidate reloads the actual inventory, then reopens complete
	// established history without borrowing initial pin provenance.
	c, err := enrollment.ReadCandidate(t.Context(), directory, enrollment.Headless)
	if err != nil {
		t.Fatal(err)
	}
	candidateInput, ok := candidateReleaseInputs(c, ref)
	if !ok {
		t.Fatal("candidate composition refused")
	}
	v, err = release.OpenRetained(history)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := installation.AuthenticateCandidate(t.Context(), v, c, candidateInput)
	closeErr := v.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("retained candidate: %v %v", err, closeErr)
	}
	cp, cg := accepted.Targets()
	for _, proof := range []release.Authorization{cp, cg} {
		d, ok := proof.AcceptedDecision()
		if !ok || d.Outcome != release.OutcomeNoUpdate {
			t.Fatal("candidate reused initial provenance or lost fresh no-update proof")
		}
	}
}

func TestInstallationSecondTargetRefusalRetainsRealFloors(t *testing.T) {
	for _, scenario := range []string{"missing-generation", "incoherent-generation"} {
		t.Run(scenario, func(t *testing.T) {
			directory, pin, program := signedConsumerTargets(t, "linux-amd64", true, scenario != "missing-generation", 2)
			bundle, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: directory, ExecutablePath: program, ManifestSHA256: pin, Scope: enrollment.Headless})
			if err != nil {
				t.Fatal(err)
			}
			in, _ := initialReleaseInputs(bundle, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
			history := filepath.Join(t.TempDir(), "history")
			v, err := release.Open(history)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if v != nil {
					if err := v.Close(); err != nil {
						t.Error(err)
					}
				}
			}()
			a, err := installation.AuthenticateInitial(t.Context(), v, bundle, in)
			want := installation.ErrAuthorization
			if scenario == "incoherent-generation" {
				want = installation.ErrBinding
			}
			if !errors.Is(err, want) {
				_ = v.Close()
				t.Fatalf("expected causal refusal %v, got %v", want, err)
			}
			p, g := a.Targets()
			if _, ok := p.AcceptedDecision(); ok {
				t.Fatal("failed pair exposed program authority")
			}
			if _, ok := g.AcceptedDecision(); ok {
				t.Fatal("failed pair exposed generation authority")
			}
			floors, err := v.CurrentFloors(t.Context())
			if err != nil || floors.RootVersion != 1 || floors.TargetsVersion != 1 || len(floors.TargetsDigest) != 32 {
				t.Fatal("second refusal refunded trust floors")
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			v, err = release.OpenRetained(history)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := v.CurrentFloors(t.Context())
			closeErr := v.Close()
			if err != nil || closeErr != nil || retained.TargetsVersion != floors.TargetsVersion || !bytes.Equal(retained.TargetsDigest, floors.TargetsDigest) {
				t.Fatal("reopen changed committed floors")
			}
		})
	}
}

func TestInstallationCandidateCannotBootstrapCompleteTrust(t *testing.T) {
	directory, _, _ := signedConsumerTargets(t, "linux-amd64", true, true, 1)
	c, err := enrollment.ReadCandidate(t.Context(), directory, enrollment.Headless)
	if err != nil {
		t.Fatal(err)
	}
	in, _ := candidateReleaseInputs(c, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	v, err := release.Open(filepath.Join(t.TempDir(), "fresh-history"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	_, err = installation.AuthenticateCandidate(t.Context(), v, c, in)
	floors, floorErr := v.CurrentFloors(t.Context())
	closeErr := v.Close()
	if !errors.Is(err, release.ErrTrustUnavailable) || floorErr != nil || closeErr != nil || floors.RootVersion != 0 || floors.TargetsVersion != 0 {
		t.Fatalf("candidate bootstrapped trust: %v %+v", err, floors)
	}
}

func TestInstallationSnapshotSubstitutionRefusesBeforeTrustFloors(t *testing.T) {
	directory, pin, program := signedConsumerTargets(t, "linux-amd64", true, true, 1)
	bundle, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: directory, ExecutablePath: program, ManifestSHA256: pin, Scope: enrollment.Headless})
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"root", "program", "metadata", "missing-metadata", "duplicate-location"} {
		t.Run(scenario, func(t *testing.T) {
			in, _ := initialReleaseInputs(bundle, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
			switch scenario {
			case "root":
				in.RootBytes[0] ^= 1
			case "program":
				in.Artifact[0] ^= 1
			case "metadata":
				in.Files["https://release.invalid/metadata/timestamp.json"][0] ^= 1
			case "missing-metadata":
				delete(in.Files, "https://release.invalid/metadata/timestamp.json")
			case "duplicate-location":
				data := in.Files["https://release.invalid/metadata/timestamp.json"]
				delete(in.Files, "https://release.invalid/metadata/1.snapshot.json")
				in.Files["https://other.invalid/timestamp.json"] = data
			}
			v, err := release.Open(filepath.Join(t.TempDir(), "history"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := v.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := installation.AuthenticateInitial(t.Context(), v, bundle, in); !errors.Is(err, installation.ErrBinding) {
				t.Fatalf("snapshot substitution: %v", err)
			}
			floors, err := v.CurrentFloors(t.Context())
			if err != nil || floors.RootVersion != 0 || floors.TargetsVersion != 0 {
				t.Fatal("snapshot mismatch advanced trust floors")
			}
		})
	}
}

type installationCancelContext struct {
	context.Context
	calls    atomic.Int64
	cancelAt int64
}

func (c *installationCancelContext) Err() error {
	n := c.calls.Add(1)
	if c.cancelAt > 0 && n >= c.cancelAt {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestInstallationOriginalCancellationAtPairHandoffRetainsFloors(t *testing.T) {
	directory, pin, program := signedConsumerTargets(t, "linux-amd64", true, true, 1)
	bundle, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: directory, ExecutablePath: program, ManifestSHA256: pin, Scope: enrollment.Headless})
	if err != nil {
		t.Fatal(err)
	}
	in, _ := initialReleaseInputs(bundle, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	// Measure genuine Release owner calls independently of Installation. The
	// composition owes its own pre-effect and final-handoff checks in addition.
	observed := &installationCancelContext{Context: context.Background()}
	v, err := release.Open(filepath.Join(t.TempDir(), "baseline"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if v != nil {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	if _, ok := v.Evaluate(observed, in).Authorization(); !ok {
		t.Fatal("baseline program refused")
	}
	second := in
	second.TargetPath = "ardents/linux-amd64/protected-endpoint"
	second.Artifact, _ = bundle.File("protected-endpoint.json")
	if _, ok := v.Evaluate(observed, second).Authorization(); !ok {
		t.Fatal("baseline generation refused")
	}
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}
	canceled := &installationCancelContext{Context: context.Background(), cancelAt: observed.calls.Load() + 2}
	v, err = release.Open(filepath.Join(t.TempDir(), "actual"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := installation.AuthenticateInitial(canceled, v, bundle, in)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late original cancellation lost: %v", err)
	}
	p, g := a.Targets()
	if _, ok := p.AcceptedDecision(); ok {
		t.Fatal("canceled pair exposed program authority")
	}
	if _, ok := g.AcceptedDecision(); ok {
		t.Fatal("canceled pair exposed generation authority")
	}
	floors, err := v.CurrentFloors(t.Context())
	if err != nil || floors.RootVersion != 1 || floors.TargetsVersion != 1 {
		t.Fatal("late cancellation refunded actual floors")
	}
}

func TestInstallationCompiledCandidateAndInitialUseIndependentTrust(t *testing.T) {
	directory, pin, program := signedConsumerTargets(t, "linux-amd64", true, true, 1)
	history := filepath.Join(t.TempDir(), "history")
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		// Actual native-platform admission refusal runs on every other host;
		// portable successful pair verification above still uses genuine owners.
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "installation", "authenticate-candidate", directory, history, "2030-01-02T03:04:05Z")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "installation-incompatible") {
			t.Fatalf("foreign platform accepted: %v %s", err, out)
		}
		if _, err := os.Lstat(history); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("foreign platform created history")
		}
		return
	}
	for _, invocation := range []struct {
		args     []string
		want     string
		accepted bool
	}{
		{[]string{"authenticate-candidate", directory, history, "2030-01-02T03:04:05Z"}, "installation-trust-unavailable", false},
		{[]string{"authenticate-initial", directory, strings.Repeat("0", 64), history, "2030-01-02T03:04:05Z"}, "installation-refused", false},
		{[]string{"authenticate-initial", directory, pin, history, "2030-01-02T03:04:05Z"}, "authenticated-generation", true},
		{[]string{"authenticate-candidate", directory, history, "2030-01-02T03:04:05Z"}, "authenticated-generation", true},
	} {
		cmd := exec.CommandContext(t.Context(), program, append([]string{"installation"}, invocation.args...)...)
		out, err := cmd.CombinedOutput()
		if (err == nil) != invocation.accepted || !strings.Contains(string(out), invocation.want) {
			t.Fatalf("actual authentication consumer: %v %s", err, out)
		}
		if !invocation.accepted {
			if _, err := os.Lstat(history); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refusal created initial trust history")
			}
		}
	}
}
