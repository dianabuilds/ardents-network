package recovery

import (
	"fmt"
)

func verifyStressCampaign(value Evidence) Result {
	var manifest stressAttemptManifest
	if err := decodeAttemptValue(value.AttemptManifest, 2<<20, &manifest); err != nil {
		return invalid("decode S4.3 campaign manifest: " + err.Error())
	}
	var index replacementCampaignIndex
	if err := decodeAttemptValue(value.AttemptCampaign, 48<<20, &index); err != nil {
		return invalid("decode S4.3 campaign index: " + err.Error())
	}
	if manifest.Schema != stressAttemptManifestSchema || len(manifest.Cells) != 3 ||
		index.Schema != replacementCampaignIndexSchema || index.ManifestDigest != jsonDigest(value.AttemptManifest) {
		return invalid("S4.3 campaign identity is invalid")
	}
	hostScope, err := decodeHostScope(manifest.HostScope)
	if err != nil || !validHostScope(hostScope, manifest.SourceCommit, manifest.ImageID) {
		return invalid("S4.3 campaign HostScope is invalid")
	}
	position := 0
	for _, cell := range manifest.Cells {
		if _, ok := findStressCell(manifest.Cells, cell.CellID); !ok {
			return invalid("S4.3 campaign cell manifest is invalid")
		}
		attempt := 1
		for {
			if position >= len(index.Attempts) {
				return invalid("S4.3 campaign receipt history is incomplete")
			}
			entry := index.Attempts[position]
			var receipt replacementAttemptReceipt
			if err := decodeAttemptValue(entry.Receipt, 5<<20, &receipt); err != nil {
				return invalid("decode S4.3 campaign receipt: " + err.Error())
			}
			expectedAttempt := fmt.Sprintf("attempt-%04d", attempt)
			expectedRoot := "cells/" + cell.CellID + "/" + expectedAttempt + "/"
			if entry.CellID != cell.CellID || entry.AttemptID != expectedAttempt ||
				receipt.CellID != entry.CellID || receipt.AttemptID != entry.AttemptID ||
				receipt.Schema != replacementAttemptReceiptSchema || receipt.ManifestDigest != cell.ManifestDigest ||
				entry.ReceiptPath != expectedRoot+"receipt.json" || entry.ReceiptDigest != jsonDigest(entry.Receipt) {
				return invalid("S4.3 campaign receipt identity or order is invalid")
			}
			position++
			if receipt.Candidate == "not-run" && receipt.Observation == "invalid" {
				if !validInfrastructureInvalid(receipt, hostScope) || entry.VerifierPath != "" ||
					entry.Verifier.Verdict != "" || entry.Verifier.EvidenceDigest != "" {
					return invalid("infrastructure-invalid S4.3 attempt is inconsistent")
				}
				attempt++
				continue
			}
			verified := verifyStressAttempt(Evidence{AttemptManifest: value.AttemptManifest,
				AttemptReceipt: entry.Receipt})
			verified.EvidenceDigest = jsonDigest(entry.Receipt)
			if receipt.Observation != "complete" || receipt.Cleanup != "complete" ||
				entry.VerifierPath != expectedRoot+"verifier.json" || entry.Verifier != verified ||
				verified.Verdict != receipt.Candidate {
				return invalid("S4.3 campaign terminal receipt is not independently verified")
			}
			if receipt.Candidate == "fail" {
				if position != len(index.Attempts) {
					return invalid("S4.3 campaign continued after candidate failure")
				}
				return fail(verified.Reason)
			}
			if receipt.Candidate != "pass" {
				return invalid("S4.3 campaign terminal candidate result is invalid")
			}
			break
		}
	}
	if position != len(index.Attempts) {
		return invalid("S4.3 campaign contains an extra or duplicate receipt")
	}
	return Result{Verdict: "pass", EvidenceDigest: jsonDigest(value.AttemptCampaign)}
}

func validInfrastructureInvalid(receipt replacementAttemptReceipt, scope hostScopeEvidence) bool {
	if receipt.Cleanup == "invalid" {
		return true
	}
	if receipt.Cleanup != "complete" {
		return false
	}
	var value cleanup
	return decodeAttemptValue(receipt.CleanupEvidence, 64<<10, &value) == nil &&
		validCleanupObservation(value, scope) && validDockerCleanupProjection(value, scope)
}
