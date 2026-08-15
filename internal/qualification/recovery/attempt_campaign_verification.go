package recovery

import (
	"encoding/json"
	"fmt"
)

const (
	replacementCampaignEnvelopeSchema = "ardents-qualification-campaign-envelope-v1"
	replacementCampaignIndexSchema    = "ardents-qualification-campaign-index-v1"
)

type replacementCampaignIndex struct {
	Schema, ManifestDigest string
	Attempts               []replacementCampaignAttempt
}

type replacementCampaignAttempt struct {
	CellID, AttemptID, ReceiptPath, ReceiptDigest, VerifierPath string
	Receipt                                                     json.RawMessage
	Verifier                                                    Result
}

func verifyReplacementCampaign(value Evidence) Result {
	var manifest replacementAttemptManifest
	if err := decodeAttemptValue(value.AttemptManifest, 2<<20, &manifest); err != nil {
		return invalid("decode replacement campaign manifest: " + err.Error())
	}
	var index replacementCampaignIndex
	if err := decodeAttemptValue(value.AttemptCampaign, 48<<20, &index); err != nil {
		return invalid("decode replacement campaign index: " + err.Error())
	}
	if manifest.Schema != replacementAttemptManifestSchema || len(manifest.Cells) != 10 ||
		index.Schema != replacementCampaignIndexSchema || index.ManifestDigest != hexDigest(value.AttemptManifest) {
		return invalid("replacement campaign identity is invalid")
	}
	for _, cell := range manifest.Cells {
		if _, ok := findAttemptCell(manifest.Cells, cell.CellID); !ok {
			return invalid("replacement campaign cell manifest is invalid")
		}
	}
	hostScope, err := decodeHostScope(manifest.HostScope)
	if err != nil || !validHostScope(hostScope, manifest.SourceCommit, manifest.ImageID) {
		return invalid("replacement campaign HostScope is invalid")
	}
	position := 0
	for _, cell := range manifest.Cells {
		attempt := 1
		for {
			if position >= len(index.Attempts) {
				return invalid("replacement campaign receipt history is incomplete")
			}
			entry := index.Attempts[position]
			var receipt replacementAttemptReceipt
			if err := decodeAttemptValue(entry.Receipt, 5<<20, &receipt); err != nil {
				return invalid("decode replacement campaign receipt: " + err.Error())
			}
			expectedAttempt := fmt.Sprintf("attempt-%04d", attempt)
			expectedRoot := "cells/" + cell.CellID + "/" + expectedAttempt + "/"
			if entry.CellID != cell.CellID || entry.AttemptID != expectedAttempt ||
				receipt.CellID != entry.CellID || receipt.AttemptID != entry.AttemptID ||
				receipt.Schema != replacementAttemptReceiptSchema || receipt.ManifestDigest != cell.ManifestDigest ||
				entry.ReceiptPath != expectedRoot+"receipt.json" ||
				entry.ReceiptDigest != hexDigest(entry.Receipt) {
				return invalid("replacement campaign receipt identity or order is invalid")
			}
			position++
			if receipt.Candidate == "not-run" && receipt.Observation == "invalid" {
				if receipt.Cleanup == "complete" {
					var cleanup cleanup
					if err := decodeAttemptValue(receipt.CleanupEvidence, 64<<10, &cleanup); err != nil ||
						!validCleanupObservation(cleanup, hostScope) || !validDockerCleanupProjection(cleanup, hostScope) {
						return invalid("infrastructure-invalid replacement cleanup observation is invalid")
					}
				} else if receipt.Cleanup != "invalid" {
					return invalid("infrastructure-invalid replacement cleanup status is invalid")
				}
				if entry.VerifierPath != "" || entry.Verifier.Verdict != "" || entry.Verifier.EvidenceDigest != "" {
					return invalid("infrastructure-invalid replacement attempt has a candidate verdict")
				}
				attempt++
				continue
			}
			verified := verifyReplacementAttempt(Evidence{AttemptManifest: value.AttemptManifest,
				AttemptReceipt: entry.Receipt})
			verified.EvidenceDigest = hexDigest(entry.Receipt)
			if receipt.Observation != "complete" || receipt.Cleanup != "complete" ||
				entry.VerifierPath != expectedRoot+"verifier.json" || entry.Verifier != verified ||
				verified.Verdict != receipt.Candidate {
				return invalid("replacement campaign terminal receipt is not independently verified")
			}
			if receipt.Candidate == "fail" {
				if position != len(index.Attempts) {
					return invalid("replacement campaign continued after candidate failure")
				}
				return fail(verified.Reason)
			}
			if receipt.Candidate != "pass" {
				return invalid("replacement campaign terminal candidate result is invalid")
			}
			break
		}
	}
	if position != len(index.Attempts) {
		return invalid("replacement campaign contains an extra or duplicate receipt")
	}
	return Result{Verdict: "pass", EvidenceDigest: hexDigest(value.AttemptCampaign)}
}
