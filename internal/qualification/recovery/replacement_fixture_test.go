package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type replacementEvidence struct {
	RouteCase  routeCase
	Candidates []replacementCandidate
	Cells      []replacementCell
}

type s42TestEvidence struct {
	Evidence
	S42 json.RawMessage
}

func s42Samples(terminal int64) []ResourceSample {
	count := int(terminal / int64(time.Second))
	result := make([]ResourceSample, 0, count)
	for index := 1; index <= count; index++ {
		counter := uint64(streamBytes) * uint64(index) / uint64(count)
		result = append(result, ResourceSample{AtNanos: int64(index) * int64(time.Second), ClientRSS: 1,
			PublisherRSS: 1, ClientCPUPercent: 1, PublisherCPUPercent: 1, ClientReceived: counter,
			ClientSent: counter, PublisherReceived: counter, PublisherSent: counter})
	}
	return result
}

func s42Topology(input []byte) []byte {
	text := strings.Replace(string(input), "  client:\n",
		"  client:\n    volumes: [recovery_introduction_user:/run/ardents/recovery-introduction-user]\n", 1)
	text = strings.Replace(text, "  publisher:\n",
		"  publisher:\n    volumes: [recovery_introduction_service:/run/ardents/recovery-introduction-service]\n", 1)
	var additions strings.Builder
	for _, role := range replacementRoleNames {
		for candidate := 2; candidate <= 3; candidate++ {
			volume := ""
			if role == "introduction" && candidate == 3 {
				volume = "    volumes: [recovery_introduction_user:/run/ardents/recovery-introduction-user, recovery_introduction_service:/run/ardents/recovery-introduction-service]\n"
			}
			fmt.Fprintf(&additions, "  %s-%d:\n    networks: [route_net]\n    restart: \"no\"\n%s", role, candidate, volume)
		}
	}
	return []byte(strings.Replace(text, "networks:\n", additions.String()+"networks:\n", 1))
}

func decodeReplacementTest(t *testing.T, raw []byte) replacementEvidence {
	t.Helper()
	var result replacementEvidence
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func verifyS42Test(value s42TestEvidence) Result {
	extension := replacementEvidence{}
	if err := json.Unmarshal(value.S42, &extension); err != nil {
		return invalid(err.Error())
	}
	manifest := replacementAttemptManifest{Schema: replacementAttemptManifestSchema,
		SourceCommit: value.SourceCommit, ImageID: value.ImageID, HostScope: value.HostScope,
		RouteCase: extension.RouteCase, Candidates: extension.Candidates,
		RouteManifest: value.Manifest.RouteManifest, Topology: value.Topology,
		TopologyDigest: hexDigest(value.Topology), Prerequisites: []replacementPrerequisite{
			{Stage: "S4.1", SourceCommit: value.SourceCommit, EvidenceDigest: strings.Repeat("a", 64)},
			{Stage: "Stage 3", SourceCommit: value.SourceCommit, EvidenceDigest: strings.Repeat("b", 64)},
		}}
	for _, cell := range extension.Cells {
		prefix := map[string]string{"client-to-publisher": "c2p", "publisher-to-client": "p2c"}[cell.Direction]
		manifest.Cells = append(manifest.Cells, replacementAttemptCell{CellID: prefix + "-" + cell.Mode,
			Direction: cell.Direction, Mode: cell.Mode, ManifestDigest: cell.CellManifestDigest})
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return invalid(err.Error())
	}
	hostScope, err := decodeHostScope(value.HostScope)
	if err != nil {
		return invalid(err.Error())
	}
	for index, cell := range extension.Cells {
		cellRaw, err := json.Marshal(cell)
		if err != nil {
			return invalid(err.Error())
		}
		cleanupProjection, err := json.Marshal(dockerCleanupProjection{Project: hostScope.AdapterProjection})
		if err != nil {
			return invalid(err.Error())
		}
		cleanup := cleanup{Adapter: hostScope.Adapter, Scope: hostScope.Commitment,
			ObservedAtNanos: cell.ActiveStartedAtNanos + cell.TerminalNanos + 1, AdapterProjection: cleanupProjection}
		cleanup.Observation = cleanupObservationCommitment(cleanup)
		cleanupRaw, err := json.Marshal(cleanup)
		if err != nil {
			return invalid(err.Error())
		}
		receipt := replacementAttemptReceipt{Schema: replacementAttemptReceiptSchema,
			CellID: manifest.Cells[index].CellID, AttemptID: "attempt-0001",
			ManifestDigest: cell.CellManifestDigest, Candidate: "pass", Observation: "complete",
			Cleanup: "complete", ActiveNanos: cell.TerminalNanos, Evidence: cellRaw,
			CleanupEvidence: cleanupRaw}
		receiptRaw, err := json.Marshal(receipt)
		if err != nil {
			return invalid(err.Error())
		}
		result := Verify(Evidence{Schema: replacementAttemptEnvelopeSchema,
			AttemptManifest: manifestRaw, AttemptReceipt: receiptRaw})
		if result.Verdict != "pass" {
			return result
		}
	}
	if len(extension.Cells) != 10 {
		return invalid(errors.New("test matrix is incomplete").Error())
	}
	return Result{Verdict: "pass"}
}

func decodeHostScopeTest(t *testing.T, raw []byte) hostScopeEvidence {
	t.Helper()
	value, err := decodeHostScope(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func encodeHostScopeTest(t *testing.T, value hostScopeEvidence) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
