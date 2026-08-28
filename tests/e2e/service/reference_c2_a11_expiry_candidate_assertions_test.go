//go:build h4_8_a11

package service_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func h48A11RequireCandidateArtifactBytes(t *testing.T, input h48A11CandidateInput, verified enrollment.Verified) {
	t.Helper()
	if sha256.Sum256(verified.Inputs.Artifact) != input.endpointDigest {
		t.Fatal("exact enrolled Endpoint bytes differ from ARDENTS_H4_8_A11_CANDIDATE_ENDPOINT_SHA256")
	}
	if len(verified.ControlArtifact) == 0 || sha256.Sum256(verified.ControlArtifact) != input.controlDigest {
		t.Fatal("exact enrolled control companion bytes differ from ARDENTS_H4_8_A11_CANDIDATE_CONTROL_SHA256")
	}
}

func h48A11RequireInspection(t *testing.T, owner string, report inspection.Report, err error, releaseOutcome release.Outcome,
	authorized bool, outcomes [3]alphacontrol.Outcome,
) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s inspection failed: %v", owner, err)
	}
	if report.Inspection.Catalog != alphacontrol.OutcomeAccepted {
		t.Fatalf("%s catalog outcome=%q, want %q", owner, report.Inspection.Catalog, alphacontrol.OutcomeAccepted)
	}
	for index, expected := range outcomes {
		component := report.Inspection.Components[index]
		if component.Class != alphacontrol.ComponentClass(index+1) || component.Outcome != expected {
			t.Fatalf("%s component %s: class=%d outcome=%q, want class=%d outcome=%q", owner, h48A11CandidateClass(index),
				component.Class, component.Outcome, index+1, expected)
		}
	}
	if report.Release != string(releaseOutcome) || report.ReleaseAuthorizationPresent != authorized {
		t.Fatalf("%s projected Release: outcome=%q authorized=%t, want outcome=%q authorized=%t", owner, report.Release,
			report.ReleaseAuthorizationPresent, releaseOutcome, authorized)
	}
}

func h48A11RequireReferenceIdentity(t *testing.T, input h48A11CandidateInput, verified enrollment.Verified,
	catalog alphacontrol.Catalog, report inspection.Report,
) {
	t.Helper()
	if report.CatalogCohort != input.cohort || report.CatalogGeneration == 0 || report.CatalogGeneration != catalog.Generation ||
		report.CatalogNotBefore != catalog.NotBefore || report.CatalogNotAfter != catalog.NotAfter ||
		report.Inspection.CatalogDigest != sha256.Sum256(verified.ControlCatalog) {
		t.Fatalf("reference catalog identity is incomplete or differs from exact enrolled catalog: cohort=%q generation=%d", report.CatalogCohort, report.CatalogGeneration)
	}
	if report.ReleaseIdentity != input.release || report.BuildIdentity == "" || report.ProtocolPhase == "" ||
		report.ArtifactDigest != input.endpointDigest || !report.ReleaseAuthorizationPresent {
		t.Fatalf("reference Release identity is incomplete: release=%q build=%q protocol=%q digest=%x authorized=%t",
			report.ReleaseIdentity, report.BuildIdentity, report.ProtocolPhase, report.ArtifactDigest, report.ReleaseAuthorizationPresent)
	}
	if report.NetworkID == [32]byte{} || report.NetworkEpoch == 0 || report.NetworkDigest == [32]byte{} ||
		report.NetworkProfile == "" || report.NetworkValidUntil.IsZero() {
		t.Fatalf("reference Network identity is incomplete: id=%x epoch=%d digest=%x profile=%q valid_until=%s",
			report.NetworkID, report.NetworkEpoch, report.NetworkDigest, report.NetworkProfile, report.NetworkValidUntil)
	}
	components := [3][]byte{verified.ControlRelease, verified.ControlNetwork, verified.ControlCompatibility}
	roots := [3][]byte{verified.ControlReleaseRoot, verified.ControlNetworkRoot, verified.ControlCompatibilityRoot}
	for index, reference := range catalog.Components {
		details := report.ComponentDetails[index]
		if reference.Class != alphacontrol.ComponentClass(index+1) || reference.RootID != details.RootID ||
			reference.Generation != details.Generation || reference.Digest != details.Digest || reference.NotAfter != details.NotAfter ||
			sha256.Sum256(components[index]) != details.Digest || sha256.Sum256(roots[index]) != details.RootID ||
			details.NotBefore.IsZero() || !details.NotBefore.Before(details.NotAfter) {
			t.Fatalf("reference component %s identity, exact bytes, or validity is incomplete", h48A11CandidateClass(index))
		}
	}
}

func h48A11RequireSameIdentity(t *testing.T, reference, actual inspection.Report) {
	t.Helper()
	if actual.Inspection.CatalogDigest != reference.Inspection.CatalogDigest || actual.CatalogCohort != reference.CatalogCohort ||
		actual.CatalogGeneration != reference.CatalogGeneration || actual.CatalogNotBefore != reference.CatalogNotBefore ||
		actual.CatalogNotAfter != reference.CatalogNotAfter || actual.ComponentDetails != reference.ComponentDetails ||
		actual.ReleaseIdentity != reference.ReleaseIdentity || actual.BuildIdentity != reference.BuildIdentity ||
		actual.ArtifactDigest != reference.ArtifactDigest || actual.ProtocolPhase != reference.ProtocolPhase ||
		actual.BuildSafetyNoNewWorkAfter != reference.BuildSafetyNoNewWorkAfter ||
		actual.BuildSafetyTerminateAfter != reference.BuildSafetyTerminateAfter || actual.NetworkID != reference.NetworkID ||
		actual.NetworkEpoch != reference.NetworkEpoch || actual.NetworkDigest != reference.NetworkDigest ||
		actual.NetworkProfile != reference.NetworkProfile || actual.NetworkValidUntil != reference.NetworkValidUntil {
		t.Fatal("inspection catalog, component, Release, or Network identity differs from the exact reference candidate")
	}
}

func h48A11RequireBoundaryRelationships(t *testing.T, referenceAt, noNew, terminal time.Time,
	catalog alphacontrol.Catalog, report inspection.Report,
) {
	t.Helper()
	if !referenceAt.Before(noNew) || !noNew.Before(terminal) {
		t.Fatalf("candidate bounds are not reference_at < no-new < terminal: reference=%s no_new=%s terminal=%s", referenceAt, noNew, terminal)
	}
	for owner, value := range map[string]time.Time{
		"reference_at": referenceAt, "no-new": noNew, "terminal": terminal, "catalog NotAfter": catalog.NotAfter,
		"inspection catalog NotAfter": report.CatalogNotAfter, "Release terminal": report.BuildSafetyTerminateAfter,
		"Network ValidUntil": report.NetworkValidUntil,
	} {
		if value.IsZero() || !value.Equal(value.UTC().Truncate(time.Second)) {
			t.Fatalf("%s is not one exact UTC second: %s", owner, value)
		}
	}
	if catalog.NotAfter != terminal || report.CatalogNotAfter != terminal || report.BuildSafetyTerminateAfter != terminal ||
		report.NetworkValidUntil != terminal {
		t.Fatalf("authenticated terminal owners differ: catalog=%s report_catalog=%s release=%s network=%s",
			catalog.NotAfter, report.CatalogNotAfter, report.BuildSafetyTerminateAfter, report.NetworkValidUntil)
	}
	for index, component := range catalog.Components {
		if component.NotAfter != terminal || report.ComponentDetails[index].NotAfter != terminal {
			t.Fatalf("component %s NotAfter=%s/%s, want terminal %s", h48A11CandidateClass(index), component.NotAfter,
				report.ComponentDetails[index].NotAfter, terminal)
		}
	}
}

func h48A11RequireTUFBounds(t *testing.T, terminal time.Time, expiries h48A11CandidateTUFExpiries) {
	t.Helper()
	want := h48A11CandidateTime(terminal)
	if expiries.Timestamp != want || expiries.Snapshot != want || expiries.Targets != want {
		t.Fatalf("authenticated TUF expiries differ from terminal: timestamp=%q snapshot=%q targets=%q terminal=%q",
			expiries.Timestamp, expiries.Snapshot, expiries.Targets, want)
	}
}

func h48A11RequireDirect(t *testing.T, owner string, decision release.Decision, input h48A11CandidateInput,
	at, noNew, terminal time.Time, outcome, buildSafety release.Outcome, authorized, authenticatedIdentity bool,
) {
	t.Helper()
	_, hasAuthorization := decision.Authorization()
	if decision.Outcome != outcome || hasAuthorization != authorized {
		t.Fatalf("%s direct Release: outcome=%q authorized=%t, want outcome=%q authorized=%t", owner, decision.Outcome,
			hasAuthorization, outcome, authorized)
	}
	if buildSafety != "" && decision.BuildSafety != buildSafety {
		t.Fatalf("%s direct Release build_safety=%q, want %q", owner, decision.BuildSafety, buildSafety)
	}
	if !authenticatedIdentity {
		return
	}
	if decision.ReferenceTime != at.UTC() || decision.ReleaseIdentity != input.release || decision.Path != input.targetPath ||
		decision.Platform != input.platform || decision.Architecture != input.architecture || decision.Environment != input.environment ||
		decision.Network != input.network || decision.BuildIdentity == "" || decision.ProtocolPhase == "" ||
		len(decision.Digest) != sha256.Size {
		t.Fatalf("%s direct Release authenticated identity is incomplete: reference=%s release=%q path=%q build=%q protocol=%q",
			owner, decision.ReferenceTime, decision.ReleaseIdentity, decision.Path, decision.BuildIdentity, decision.ProtocolPhase)
	}
	var digest [32]byte
	copy(digest[:], decision.Digest)
	if digest != input.endpointDigest || decision.BuildSafetyNoNewWorkAfter != noNew || decision.BuildSafetyTerminateAfter != terminal {
		t.Fatalf("%s direct Release digest or authenticated safety bounds differ: digest=%x no_new=%s terminal=%s", owner, digest,
			decision.BuildSafetyNoNewWorkAfter, decision.BuildSafetyTerminateAfter)
	}
}

func h48A11RequireCatalogBoundaryRefusal(t *testing.T, owner string, report inspection.Report, err error) {
	t.Helper()
	if err == nil || report.Inspection.Catalog != alphacontrol.OutcomeInvalid ||
		!strings.Contains(err.Error(), "alpha control catalog is invalid or outside validity") {
		t.Fatalf("%s did not refuse at the exact catalog expiry: catalog=%q err=%v", owner, report.Inspection.Catalog, err)
	}
}

func h48A11RequireTerminalComponents(t *testing.T, catalog alphacontrol.Catalog, verified enrollment.Verified,
	terminal time.Time,
) []h48A11CandidateClassOutcome {
	t.Helper()
	raws := [3][]byte{verified.ControlRelease, verified.ControlNetwork, verified.ControlCompatibility}
	roots := [3][]byte{verified.ControlReleaseRoot, verified.ControlNetworkRoot, verified.ControlCompatibilityRoot}
	result := make([]h48A11CandidateClassOutcome, len(raws))
	for index := range raws {
		outcome := alphacontrol.VerifyComponent(catalog.Components[index], raws[index], ed25519.PublicKey(roots[index]), terminal)
		if outcome != alphacontrol.OutcomeExpired {
			t.Fatalf("direct component %s at terminal=%q, want %q", h48A11CandidateClass(index), outcome, alphacontrol.OutcomeExpired)
		}
		result[index] = h48A11CandidateClassOutcome{Class: h48A11CandidateClass(index), Outcome: string(outcome)}
	}
	return result
}
