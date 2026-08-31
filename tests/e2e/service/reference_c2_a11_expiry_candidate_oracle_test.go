//go:build referencec2 && h4_8_a11

package service_test

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func h48A11QualifyCandidateV2(t *testing.T) h48A11ExpiryCandidateV2Report {
	t.Helper()
	input, err := h48A11LoadCandidateInput()
	if err != nil {
		t.Fatal(err)
	}
	verified, err := enrollment.Verify(input.request)
	if err != nil {
		t.Fatalf("verify exact candidate enrollment: %v", err)
	}
	h48A11RequireCandidateArtifactBytes(t, input, verified)
	catalog, _, err := alphacontrol.Verify(verified.ControlCatalog, ed25519.PublicKey(verified.DisclosureRoot), input.referenceAt)
	if err != nil {
		t.Fatalf("verify exact candidate catalog at reference_at: %v", err)
	}

	reference, err := h48A11InspectAt(filepath.Join(t.TempDir(), "reference-inspection"), input, input.referenceAt)
	h48A11RequireInspection(t, "reference", reference, err, release.OutcomeReleaseAccepted, true,
		[3]alphacontrol.Outcome{alphacontrol.OutcomeAccepted, alphacontrol.OutcomeAccepted, alphacontrol.OutcomeAccepted})
	h48A11RequireReferenceIdentity(t, input, verified, catalog, reference)
	noNew, terminal := reference.BuildSafetyNoNewWorkAfter, reference.BuildSafetyTerminateAfter
	referenceDirect := h48A11EvaluateFresh(t, verified.Inputs, input.referenceAt, "reference-release")
	h48A11RequireDirect(t, "reference", referenceDirect, input, input.referenceAt, noNew, terminal,
		release.OutcomeReleaseAccepted, release.OutcomeReleaseAccepted, true, true)

	h48A11RequireBoundaryRelationships(t, input.referenceAt, noNew, terminal, catalog, reference)
	tuf := h48A11ReadTUFExpiries(t, verified.Inputs)
	h48A11RequireTUFBounds(t, terminal, tuf)
	noNewMinusOne, terminalMinusOne, terminalPlusOne := noNew.Add(-time.Second), terminal.Add(-time.Second), terminal.Add(time.Second)

	noNewBefore, err := h48A11InspectAt(filepath.Join(t.TempDir(), "no-new-minus-one-inspection"), input, noNewMinusOne)
	h48A11RequireInspection(t, "no-new-1s", noNewBefore, err, release.OutcomeReleaseAccepted, true,
		[3]alphacontrol.Outcome{alphacontrol.OutcomeAccepted, alphacontrol.OutcomeAccepted, alphacontrol.OutcomeAccepted})
	h48A11RequireSameIdentity(t, reference, noNewBefore)
	noNewBeforeDirect := h48A11EvaluateFresh(t, verified.Inputs, noNewMinusOne, "no-new-minus-one-release")
	h48A11RequireDirect(t, "no-new-1s", noNewBeforeDirect, input, noNewMinusOne, noNew, terminal,
		release.OutcomeReleaseAccepted, release.OutcomeReleaseAccepted, true, true)

	persistedInspectionRoot := filepath.Join(t.TempDir(), "persisted-inspection")
	persistedSeed, err := h48A11InspectAt(persistedInspectionRoot, input, noNewMinusOne)
	h48A11RequireInspection(t, "persisted no-new-1s seed", persistedSeed, err, release.OutcomeReleaseAccepted, true,
		[3]alphacontrol.Outcome{alphacontrol.OutcomeAccepted, alphacontrol.OutcomeAccepted, alphacontrol.OutcomeAccepted})
	h48A11RequireSameIdentity(t, reference, persistedSeed)
	freshNoNew, err := h48A11InspectAt(filepath.Join(t.TempDir(), "fresh-no-new-exact"), input, noNew)
	persistedNoNew, persistedErr := h48A11InspectAt(persistedInspectionRoot, input, noNew)
	persistedRetry, retryErr := h48A11InspectAt(persistedInspectionRoot, input, noNew)
	for _, point := range []struct {
		name   string
		report inspection.Report
		err    error
	}{{"fresh no-new exact", freshNoNew, err}, {"persisted no-new exact", persistedNoNew, persistedErr}, {"persisted no-new retry", persistedRetry, retryErr}} {
		h48A11RequireInspection(t, point.name, point.report, point.err, release.OutcomeUpdateRequired, false,
			[3]alphacontrol.Outcome{alphacontrol.OutcomeExpired, alphacontrol.OutcomeAccepted, alphacontrol.OutcomeUnavailable})
		h48A11RequireSameIdentity(t, reference, point.report)
	}

	directVerifier, err := release.Open(filepath.Join(t.TempDir(), "persisted-direct-release"))
	if err != nil {
		t.Fatalf("open persisted direct Release root: %v", err)
	}
	directSeed := directVerifier.Evaluate(context.Background(), h48A11InputsAt(verified.Inputs, noNewMinusOne))
	directNoNew := directVerifier.Evaluate(context.Background(), h48A11InputsAt(verified.Inputs, noNew))
	directRetry := directVerifier.Evaluate(context.Background(), h48A11InputsAt(verified.Inputs, noNew))
	if closeErr := directVerifier.Close(); closeErr != nil {
		t.Fatalf("close persisted direct Release root: %v", closeErr)
	}
	h48A11RequireDirect(t, "persisted direct seed", directSeed, input, noNewMinusOne, noNew, terminal,
		release.OutcomeReleaseAccepted, release.OutcomeReleaseAccepted, true, true)
	h48A11RequireDirect(t, "direct no-new exact", directNoNew, input, noNew, noNew, terminal,
		release.OutcomeUpdateRequired, release.OutcomeUpdateRequired, false, true)
	h48A11RequireDirect(t, "direct no-new retry", directRetry, input, noNew, noNew, terminal,
		release.OutcomeUpdateRequired, release.OutcomeUpdateRequired, false, true)

	terminalBefore, err := h48A11InspectAt(persistedInspectionRoot, input, terminalMinusOne)
	h48A11RequireInspection(t, "terminal-1s", terminalBefore, err, release.OutcomeUpdateRequired, false,
		[3]alphacontrol.Outcome{alphacontrol.OutcomeExpired, alphacontrol.OutcomeAccepted, alphacontrol.OutcomeUnavailable})
	h48A11RequireSameIdentity(t, reference, terminalBefore)
	terminalBeforeDirect := h48A11EvaluateFresh(t, verified.Inputs, terminalMinusOne, "terminal-minus-one-release")
	h48A11RequireDirect(t, "terminal-1s", terminalBeforeDirect, input, terminalMinusOne, noNew, terminal,
		release.OutcomeUpdateRequired, release.OutcomeUpdateRequired, false, true)

	persistedTerminal, persistedTerminalErr := h48A11InspectAt(persistedInspectionRoot, input, terminal)
	freshTerminal, freshTerminalErr := h48A11InspectAt(filepath.Join(t.TempDir(), "fresh-terminal-exact"), input, terminal)
	h48A11RequireCatalogBoundaryRefusal(t, "persisted terminal", persistedTerminal, persistedTerminalErr)
	h48A11RequireCatalogBoundaryRefusal(t, "fresh terminal", freshTerminal, freshTerminalErr)
	terminalComponents := h48A11RequireTerminalComponents(t, catalog, verified, terminal)
	terminalDirect := h48A11EvaluateFresh(t, verified.Inputs, terminal, "terminal-exact-release")
	h48A11RequireDirect(t, "terminal exact", terminalDirect, input, terminal, noNew, terminal,
		release.OutcomeReleaseRevoked, release.OutcomeReleaseRevoked, false, true)
	terminalPlusDirect := h48A11EvaluateFresh(t, verified.Inputs, terminalPlusOne, "terminal-plus-one-release")
	h48A11RequireDirect(t, "terminal+1s", terminalPlusDirect, input, terminalPlusOne, noNew, terminal,
		release.OutcomeReleaseExpired, "", false, false)

	return h48A11ExpiryCandidateV2Report{Schema: "ardents-h4-8-a11-expiry-candidate-v2", EndpointSHA256: input.endpointDigestRaw,
		ControlSHA256: input.controlDigestRaw, Identity: h48A11CandidateIdentity(reference),
		Bounds: h48A11CandidateBoundsReport{ReferenceAt: h48A11CandidateTime(input.referenceAt), NoNewWorkAfter: h48A11CandidateTime(noNew),
			Terminal: h48A11CandidateTime(terminal), NetworkValidUntil: h48A11CandidateTime(reference.NetworkValidUntil), TUFExpires: tuf},
		Points: h48A11CandidatePointsReport{
			Reference:     h48A11CandidateAcceptedPoint{At: h48A11CandidateTime(input.referenceAt), Inspection: h48A11CandidateInspection(reference), DirectRelease: h48A11CandidateDirect(referenceDirect)},
			NoNewMinusOne: h48A11CandidateAcceptedPoint{At: h48A11CandidateTime(noNewMinusOne), Inspection: h48A11CandidateInspection(noNewBefore), DirectRelease: h48A11CandidateDirect(noNewBeforeDirect)},
			NoNewExact: h48A11CandidateNoNewExactPoint{At: h48A11CandidateTime(noNew), FreshInspection: h48A11CandidateInspection(freshNoNew), PersistedInspection: h48A11CandidateInspection(persistedNoNew),
				PersistedRetry: h48A11CandidateInspection(persistedRetry), DirectRelease: h48A11CandidateDirect(directNoNew), DirectRetry: h48A11CandidateDirect(directRetry)},
			TerminalMinusOne: h48A11CandidateAcceptedPoint{At: h48A11CandidateTime(terminalMinusOne), Inspection: h48A11CandidateInspection(terminalBefore), DirectRelease: h48A11CandidateDirect(terminalBeforeDirect)},
			TerminalExact: h48A11CandidateTerminalExactPoint{At: h48A11CandidateTime(terminal), FreshCatalogOutcome: string(freshTerminal.Inspection.Catalog), FreshFullRefused: freshTerminalErr != nil,
				PersistedCatalogOutcome: string(persistedTerminal.Inspection.Catalog), PersistedFullRefused: persistedTerminalErr != nil,
				DirectComponentOutcomes: terminalComponents, DirectRelease: h48A11CandidateDirect(terminalDirect)},
			TerminalPlusOne: h48A11CandidateDirectPoint{At: h48A11CandidateTime(terminalPlusOne), DirectRelease: h48A11CandidateDirect(terminalPlusDirect)},
		}, Status: "accepted"}
}

func h48A11InspectAt(root string, input h48A11CandidateInput, at time.Time) (inspection.Report, error) {
	return inspection.Inspect(context.Background(), inspection.Config{Root: root, Enrollment: input.request, At: at})
}

func h48A11InputsAt(inputs release.Inputs, at time.Time) release.Inputs {
	inputs.Local.RefTime = at.UTC()
	return inputs
}

func h48A11EvaluateFresh(t *testing.T, inputs release.Inputs, at time.Time, owner string) release.Decision {
	t.Helper()
	verifier, err := release.Open(filepath.Join(t.TempDir(), owner))
	if err != nil {
		t.Fatalf("open fresh %s root: %v", owner, err)
	}
	decision := verifier.Evaluate(context.Background(), h48A11InputsAt(inputs, at))
	if err := verifier.Close(); err != nil {
		t.Fatalf("close fresh %s root: %v", owner, err)
	}
	return decision
}

type h48A11TUFEnvelope struct {
	Signed struct {
		Expires time.Time `json:"expires"`
		Meta    map[string]struct {
			Version int64 `json:"version"`
		} `json:"meta"`
	} `json:"signed"`
}

func h48A11ReadTUFExpiries(t *testing.T, inputs release.Inputs) h48A11CandidateTUFExpiries {
	t.Helper()
	timestamp := h48A11ReadTUFEnvelope(t, inputs.Files[release.MetadataURL("timestamp.json")], "timestamp")
	snapshotRef, ok := timestamp.Signed.Meta["snapshot.json"]
	if !ok || snapshotRef.Version < 1 {
		t.Fatal("authenticated timestamp lacks a positive snapshot version")
	}
	snapshotName := fmt.Sprintf("%d.snapshot.json", snapshotRef.Version)
	snapshot := h48A11ReadTUFEnvelope(t, inputs.Files[release.MetadataURL(snapshotName)], snapshotName)
	targetsRef, ok := snapshot.Signed.Meta["targets.json"]
	if !ok || targetsRef.Version < 1 {
		t.Fatal("authenticated snapshot lacks a positive targets version")
	}
	targetsName := fmt.Sprintf("%d.targets.json", targetsRef.Version)
	targets := h48A11ReadTUFEnvelope(t, inputs.Files[release.MetadataURL(targetsName)], targetsName)
	return h48A11CandidateTUFExpiries{Timestamp: h48A11CandidateTime(timestamp.Signed.Expires),
		Snapshot: h48A11CandidateTime(snapshot.Signed.Expires), Targets: h48A11CandidateTime(targets.Signed.Expires)}
}

func h48A11ReadTUFEnvelope(t *testing.T, raw []byte, owner string) h48A11TUFEnvelope {
	t.Helper()
	var envelope h48A11TUFEnvelope
	if len(raw) == 0 {
		t.Fatalf("exact candidate lacks %s metadata bytes", owner)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Signed.Expires.IsZero() ||
		!envelope.Signed.Expires.Equal(envelope.Signed.Expires.UTC().Truncate(time.Second)) {
		t.Fatalf("decode exact %s expiry: expires=%s err=%v", owner, envelope.Signed.Expires, err)
	}
	return envelope
}
