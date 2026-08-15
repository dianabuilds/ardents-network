package recovery

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyReplacementCampaignHistory(t *testing.T) {
	bundle := validS42Evidence(t)
	extension := decodeReplacementTest(t, bundle.S42)
	manifest := replacementAttemptManifest{Schema: replacementAttemptManifestSchema,
		SourceCommit: bundle.SourceCommit, ImageID: bundle.ImageID, HostScope: bundle.HostScope,
		RouteCase: extension.RouteCase, Candidates: extension.Candidates,
		RouteManifest: bundle.Manifest.RouteManifest, Topology: bundle.Topology,
		TopologyDigest: hexDigest(bundle.Topology), Prerequisites: []replacementPrerequisite{
			{Stage: "S4.1", SourceCommit: bundle.SourceCommit, EvidenceDigest: strings.Repeat("a", 64)},
			{Stage: "Stage 3", SourceCommit: bundle.SourceCommit, EvidenceDigest: strings.Repeat("b", 64)},
		}}
	for _, cell := range extension.Cells {
		prefix := map[string]string{"client-to-publisher": "c2p", "publisher-to-client": "p2c"}[cell.Direction]
		manifest.Cells = append(manifest.Cells, replacementAttemptCell{CellID: prefix + "-" + cell.Mode,
			Direction: cell.Direction, Mode: cell.Mode, ManifestDigest: cell.CellManifestDigest})
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := decodeHostScope(bundle.HostScope)
	if err != nil {
		t.Fatal(err)
	}
	index := replacementCampaignIndex{Schema: replacementCampaignIndexSchema, ManifestDigest: hexDigest(manifestRaw)}
	for cellIndex, cell := range extension.Cells {
		cleanupProjection, marshalErr := json.Marshal(dockerCleanupProjection{Project: scope.AdapterProjection})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		cleanup := cleanup{Adapter: scope.Adapter, Scope: scope.Commitment,
			ObservedAtNanos: cell.ActiveStartedAtNanos + cell.TerminalNanos + 1, AdapterProjection: cleanupProjection}
		cleanup.Observation = cleanupObservationCommitment(cleanup)
		cellRaw, marshalErr := json.Marshal(cell)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		cleanupRaw, marshalErr := json.Marshal(cleanup)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		manifestCell := manifest.Cells[cellIndex]
		receipt := replacementAttemptReceipt{Schema: replacementAttemptReceiptSchema,
			CellID: manifestCell.CellID, AttemptID: "attempt-0001", ManifestDigest: manifestCell.ManifestDigest,
			Candidate: "pass", Observation: "complete", Cleanup: "complete", ActiveNanos: cell.TerminalNanos,
			Evidence: cellRaw, CleanupEvidence: cleanupRaw}
		receiptRaw, marshalErr := json.Marshal(receipt)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		verifier := Verify(Evidence{Schema: replacementAttemptEnvelopeSchema,
			AttemptManifest: manifestRaw, AttemptReceipt: receiptRaw})
		if verifier.Verdict != "pass" {
			t.Fatalf("attempt fixture %d is invalid: %+v", cellIndex, verifier)
		}
		root := "cells/" + receipt.CellID + "/attempt-0001/"
		index.Attempts = append(index.Attempts, replacementCampaignAttempt{CellID: receipt.CellID,
			AttemptID: receipt.AttemptID, ReceiptPath: root + "receipt.json", ReceiptDigest: hexDigest(receiptRaw),
			VerifierPath: root + "verifier.json", Receipt: receiptRaw, Verifier: verifier})
	}
	assertReplacementCampaignVerdict(t, manifestRaw, index, "pass")

	failIndex := replacementCampaignIndex{Schema: index.Schema, ManifestDigest: index.ManifestDigest,
		Attempts: append([]replacementCampaignAttempt(nil), index.Attempts[:1]...)}
	var failReceipt replacementAttemptReceipt
	if err := json.Unmarshal(failIndex.Attempts[0].Receipt, &failReceipt); err != nil {
		t.Fatal(err)
	}
	failCell := extension.Cells[0]
	failCell.Events, failCell.Routes = nil, failCell.Routes[:1]
	failEvidence := replacementFailureEvidence{Cell: failCell, Faults: map[string]processFaultEvidence{},
		Failure: replacementFailureObservation{Kind: "progress", EventIndex: 0,
			ExpectedOffset: failCell.FaultOffsets[0], ObservedOffset: failCell.FaultOffsets[0] - 1,
			ObservedAtNanos: failCell.TerminalNanos}}
	failReceipt.Candidate, failReceipt.Reason = "fail", "receiver did not drain to the exact replacement gate"
	failReceipt.Evidence, err = json.Marshal(failEvidence)
	if err != nil {
		t.Fatal(err)
	}
	failIndex.Attempts[0].Receipt, err = json.Marshal(failReceipt)
	if err != nil {
		t.Fatal(err)
	}
	failIndex.Attempts[0].ReceiptDigest = hexDigest(failIndex.Attempts[0].Receipt)
	failIndex.Attempts[0].Verifier = Verify(Evidence{Schema: replacementAttemptEnvelopeSchema,
		AttemptManifest: manifestRaw, AttemptReceipt: failIndex.Attempts[0].Receipt})
	assertReplacementCampaignVerdict(t, manifestRaw, failIndex, "fail")

	first := index.Attempts[0]
	var receipt replacementAttemptReceipt
	if err := json.Unmarshal(first.Receipt, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Candidate, receipt.Observation, receipt.Cleanup = "not-run", "invalid", "invalid"
	receipt.Evidence, receipt.CleanupEvidence = nil, nil
	invalidRaw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	invalid := replacementCampaignAttempt{CellID: receipt.CellID, AttemptID: receipt.AttemptID,
		ReceiptPath: first.ReceiptPath, ReceiptDigest: hexDigest(invalidRaw), Receipt: invalidRaw}
	index.Attempts = append([]replacementCampaignAttempt{invalid}, index.Attempts...)
	index.Attempts[1].AttemptID = "attempt-0002"
	index.Attempts[1].ReceiptPath = "cells/" + receipt.CellID + "/attempt-0002/receipt.json"
	index.Attempts[1].VerifierPath = "cells/" + receipt.CellID + "/attempt-0002/verifier.json"
	var retryReceipt replacementAttemptReceipt
	if err := json.Unmarshal(index.Attempts[1].Receipt, &retryReceipt); err != nil {
		t.Fatal(err)
	}
	retryReceipt.AttemptID = "attempt-0002"
	index.Attempts[1].Receipt, err = json.Marshal(retryReceipt)
	if err != nil {
		t.Fatal(err)
	}
	index.Attempts[1].ReceiptDigest = hexDigest(index.Attempts[1].Receipt)
	index.Attempts[1].Verifier = Verify(Evidence{Schema: replacementAttemptEnvelopeSchema,
		AttemptManifest: manifestRaw, AttemptReceipt: index.Attempts[1].Receipt})
	assertReplacementCampaignVerdict(t, manifestRaw, index, "pass")

	index.Attempts = append(index.Attempts, index.Attempts[len(index.Attempts)-1])
	assertReplacementCampaignVerdict(t, manifestRaw, index, "invalid")
}

func assertReplacementCampaignVerdict(t *testing.T, manifest json.RawMessage,
	index replacementCampaignIndex, wanted string) {
	t.Helper()
	indexRaw, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	result := Verify(Evidence{Schema: replacementCampaignEnvelopeSchema,
		AttemptManifest: manifest, AttemptCampaign: indexRaw})
	if result.Verdict != wanted {
		t.Fatalf("campaign verdict = %+v, want %s", result, wanted)
	}
}
