package recovery

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyIndependentReplacementAttempt(t *testing.T) {
	bundle := validS42Evidence(t)
	extension := decodeReplacementTest(t, bundle.S42)
	cell := extension.Cells[2]
	manifest := replacementAttemptManifest{Schema: replacementAttemptManifestSchema,
		SourceCommit: bundle.SourceCommit, ImageID: bundle.ImageID, HostScope: bundle.HostScope,
		RouteCase: extension.RouteCase, Candidates: extension.Candidates,
		RouteManifest: bundle.Manifest.RouteManifest, Topology: bundle.Topology,
		TopologyDigest: hexDigest(bundle.Topology), Prerequisites: []replacementPrerequisite{
			{Stage: "S4.1", SourceCommit: bundle.SourceCommit, EvidenceDigest: strings.Repeat("a", 64)},
			{Stage: "Stage 3", SourceCommit: bundle.SourceCommit, EvidenceDigest: strings.Repeat("b", 64)},
		}}
	for _, manifestCell := range extension.Cells {
		prefix := map[string]string{"client-to-publisher": "c2p", "publisher-to-client": "p2c"}[manifestCell.Direction]
		manifest.Cells = append(manifest.Cells, replacementAttemptCell{CellID: prefix + "-" + manifestCell.Mode,
			Direction: manifestCell.Direction, Mode: manifestCell.Mode, ManifestDigest: manifestCell.CellManifestDigest})
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	cellRaw, err := json.Marshal(cell)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := decodeHostScope(bundle.HostScope)
	if err != nil {
		t.Fatal(err)
	}
	cleanupProjection, err := json.Marshal(dockerCleanupProjection{Project: scope.AdapterProjection})
	if err != nil {
		t.Fatal(err)
	}
	cleanup := cleanup{Adapter: scope.Adapter, Scope: scope.Commitment,
		ObservedAtNanos: cell.ActiveStartedAtNanos + cell.TerminalNanos + 1, AdapterProjection: cleanupProjection}
	cleanup.Observation = cleanupObservationCommitment(cleanup)
	cleanupRaw, err := json.Marshal(cleanup)
	if err != nil {
		t.Fatal(err)
	}
	receipt := replacementAttemptReceipt{Schema: replacementAttemptReceiptSchema,
		CellID: manifest.Cells[2].CellID, AttemptID: "attempt-0001", ManifestDigest: cell.CellManifestDigest,
		Candidate: "pass", Observation: "complete", Cleanup: "complete",
		ActiveNanos: cell.TerminalNanos, Evidence: cellRaw, CleanupEvidence: cleanupRaw}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}

	value := Evidence{Schema: replacementAttemptEnvelopeSchema,
		AttemptManifest: manifestRaw, AttemptReceipt: receiptRaw}
	if result := Verify(value); result.Verdict != "pass" {
		t.Fatalf("valid replacement attempt was rejected: %+v", result)
	}
	badCleanup := cleanup
	badCleanup.AdapterProjection, err = json.Marshal(dockerCleanupProjection{Project: "another-project"})
	if err != nil {
		t.Fatal(err)
	}
	badCleanup.Observation = cleanupObservationCommitment(badCleanup)
	receipt.CleanupEvidence, err = json.Marshal(badCleanup)
	if err != nil {
		t.Fatal(err)
	}
	value.AttemptReceipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict != "invalid" {
		t.Fatalf("wrong-project cleanup observation was accepted: %+v", result)
	}
	receipt.CleanupEvidence = cleanupRaw

	failureCell := cell
	failureCell.Events = nil
	failureCell.Routes = failureCell.Routes[:1]
	failure := replacementFailureEvidence{Cell: failureCell,
		Failure: replacementFailureObservation{Kind: "progress", EventIndex: 0,
			ExpectedOffset: failureCell.FaultOffsets[0], ObservedOffset: failureCell.FaultOffsets[0] - 1,
			ObservedAtNanos: failureCell.TerminalNanos}}
	receipt.Evidence, err = json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Candidate, receipt.Reason = "fail", "receiver did not drain to the exact replacement gate"
	value.AttemptReceipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict != "fail" {
		t.Fatalf("observed replacement failure was not independently verified: %+v", result)
	}
	failure.Failure.ObservedOffset = failure.Failure.ExpectedOffset
	receipt.Evidence, err = json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	value.AttemptReceipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict != "invalid" {
		t.Fatalf("unobserved replacement failure was accepted: %+v", result)
	}

	completedFailure := cell
	completedFailure.TerminalClean = false
	receipt.Evidence, err = json.Marshal(completedFailure)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Candidate, receipt.Reason = "fail", "replacement candidate violated the cell contract"
	value.AttemptReceipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict != "fail" {
		t.Fatalf("completed candidate failure was not independently verified: %+v", result)
	}

	receipt.Evidence = cellRaw
	receipt.Candidate = "fail"
	value.AttemptReceipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict == "pass" {
		t.Fatal("receipt candidate result was not bound to its evidence")
	}

	receipt.Candidate = "pass"
	receipt.ManifestDigest = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	value.AttemptReceipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if result := Verify(value); result.Verdict == "pass" {
		t.Fatal("receipt passed with the wrong manifest")
	}
}
