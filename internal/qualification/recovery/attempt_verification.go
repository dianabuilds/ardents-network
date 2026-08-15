package recovery

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	replacementAttemptEnvelopeSchema = "ardents-qualification-attempt-envelope-v1"
	replacementAttemptManifestSchema = "ardents-h3-s42-attempt-manifest-v1"
	replacementAttemptReceiptSchema  = "ardents-qualification-cell-receipt-v1"
)

type replacementAttemptManifest struct {
	Schema, SourceCommit, ImageID, TopologyDigest string
	Topology                                      []byte
	HostScope                                     json.RawMessage
	RouteCase                                     routeCase
	Candidates                                    []replacementCandidate
	RouteManifest                                 [32]byte
	Prerequisites                                 []replacementPrerequisite
	Cells                                         []replacementAttemptCell
}

type replacementPrerequisite struct {
	Stage, SourceCommit, EvidenceDigest string
}

type replacementAttemptCell struct {
	CellID, Direction, Mode, ManifestDigest string
}

type replacementAttemptReceipt struct {
	Schema, CellID, AttemptID, ManifestDigest string
	Candidate, Observation, Cleanup           string
	Reason                                    string          `json:",omitempty"`
	ActiveNanos                               int64           `json:"active_nanos"`
	Evidence                                  json.RawMessage `json:",omitempty"`
	CleanupEvidence                           json.RawMessage `json:"cleanup_evidence,omitempty"`
}

func verifyReplacementAttempt(value Evidence) Result {
	var manifest replacementAttemptManifest
	if err := decodeAttemptValue(value.AttemptManifest, 2<<20, &manifest); err != nil {
		return invalid("decode replacement attempt manifest: " + err.Error())
	}
	var receipt replacementAttemptReceipt
	if err := decodeAttemptValue(value.AttemptReceipt, 5<<20, &receipt); err != nil {
		return invalid("decode replacement attempt receipt: " + err.Error())
	}
	cellManifest, ok := findAttemptCell(manifest.Cells, receipt.CellID)
	if manifest.Schema != replacementAttemptManifestSchema || len(manifest.SourceCommit) != 40 ||
		manifest.ImageID == "" || len(manifest.Topology) == 0 || len(manifest.Topology) > 1<<20 ||
		hexDigest(manifest.Topology) != manifest.TopologyDigest || !ok ||
		!validAttemptCellID(cellManifest.CellID, cellManifest.Direction, cellManifest.Mode) ||
		receipt.Schema != replacementAttemptReceiptSchema ||
		!strings.HasPrefix(receipt.AttemptID, "attempt-") || receipt.ManifestDigest != cellManifest.ManifestDigest ||
		receipt.Observation != "complete" || receipt.Cleanup != "complete" || receipt.ActiveNanos <= 0 {
		return invalid("replacement attempt identity or lifecycle result is invalid")
	}
	if !validReplacementPrerequisites(manifest.Prerequisites, manifest.SourceCommit) {
		return invalid("replacement campaign prerequisite references are invalid")
	}
	hostScope, err := decodeHostScope(manifest.HostScope)
	if err != nil || !validHostScope(hostScope, manifest.SourceCommit, manifest.ImageID) ||
		hostScope.Adapter != "docker-compose-v1" {
		return invalid("replacement attempt host scope is invalid")
	}
	var cleanup cleanup
	if err := decodeAttemptValue(receipt.CleanupEvidence, 64<<10, &cleanup); err != nil ||
		!validCleanupObservation(cleanup, hostScope) || !validDockerCleanupProjection(cleanup, hostScope) {
		return invalid("replacement attempt cleanup observation is invalid")
	}
	routeCommitment, err := commitRouteCase(manifest.RouteCase)
	if err != nil || routeCommitment != manifest.RouteManifest ||
		manifest.RouteCase.NetworkID == [32]byte{} || manifest.RouteCase.Profile != "h3-route-tracer-v1" ||
		manifest.RouteCase.ManifestDigest != [32]byte{} || len(manifest.RouteCase.RawEvidence) != 0 {
		return invalid("replacement attempt Route case is invalid")
	}
	byRole, err := verifyReplacementCandidates(manifest.Candidates)
	if err != nil || !candidateCasesMatch(manifest.RouteCase.Candidates, manifest.Candidates) {
		return invalid(errors.Join(err, errors.New("replacement candidates differ from the Route case")).Error())
	}
	if receipt.Candidate == "fail" {
		result := verifyReplacementAttemptFailure(receipt, cellManifest, manifest, hostScope, byRole)
		if result.Verdict != "invalid" && cleanup.ObservedAtNanos <= failureHostEnd(receipt.Evidence) {
			return invalid("replacement cleanup observation precedes candidate evidence")
		}
		return result
	}
	if receipt.Candidate != "pass" {
		return invalid("replacement attempt candidate verdict is invalid")
	}
	var cell replacementCell
	if err := decodeAttemptValue(receipt.Evidence, 4<<20, &cell); err != nil {
		return invalid("decode replacement cell evidence: " + err.Error())
	}
	if cell.Direction != cellManifest.Direction || cell.Mode != cellManifest.Mode ||
		cell.CellManifestDigest != cellManifest.ManifestDigest || receipt.ActiveNanos != cell.TerminalNanos {
		return invalid("replacement cell differs from its immutable attempt manifest")
	}
	if cleanup.ObservedAtNanos <= cell.ActiveStartedAtNanos+cell.TerminalNanos {
		return invalid("replacement cleanup observation precedes the active interval")
	}
	if !validDockerReplacementProcesses(cell, hostScope) {
		return invalid("replacement attempt Docker projection is invalid")
	}
	if err := verifyReplacementDockerTopology(manifest.Topology); err != nil {
		return invalid(err.Error())
	}
	result := verifyReplacementCell(cell, byRole, manifest.RouteCase, manifest.RouteManifest,
		manifest.ImageID, hostScope)
	if result.Verdict != receipt.Candidate {
		return invalid("replacement candidate result differs from independently verified evidence")
	}
	return result
}

func validReplacementPrerequisites(values []replacementPrerequisite, sourceCommit string) bool {
	if len(values) != 2 {
		return false
	}
	for index, stage := range []string{"S4.1", "Stage 3"} {
		decoded, err := hex.DecodeString(values[index].EvidenceDigest)
		if values[index].Stage != stage || values[index].SourceCommit != sourceCommit || err != nil || len(decoded) != 32 {
			return false
		}
	}
	return true
}

func decodeAttemptValue(raw json.RawMessage, maximum int, target any) error {
	if len(raw) == 0 || len(raw) > maximum {
		return errors.New("value is empty or exceeds its byte bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("value contains multiple JSON values")
	}
	return nil
}

func validAttemptCellID(cellID, direction, mode string) bool {
	prefix := map[string]string{"client-to-publisher": "c2p", "publisher-to-client": "p2c"}[direction]
	return prefix != "" && cellID == prefix+"-"+mode
}

func findAttemptCell(values []replacementAttemptCell, cellID string) (replacementAttemptCell, bool) {
	if len(values) != 10 {
		return replacementAttemptCell{}, false
	}
	var result replacementAttemptCell
	found, seen := false, make(map[string]bool, len(values))
	modes := []string{"isolated-initiator", "isolated-introduction", "isolated-rendezvous",
		"isolated-responder", "sequential-three"}
	for index, value := range values {
		direction := "client-to-publisher"
		if index >= len(modes) {
			direction = "publisher-to-client"
		}
		if value.Direction != direction || value.Mode != modes[index%len(modes)] ||
			!validAttemptCellID(value.CellID, value.Direction, value.Mode) || value.ManifestDigest == "" || seen[value.CellID] {
			return replacementAttemptCell{}, false
		}
		seen[value.CellID] = true
		if value.CellID == cellID {
			if found {
				return replacementAttemptCell{}, false
			}
			result, found = value, true
		}
	}
	return result, found
}
