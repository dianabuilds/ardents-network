package recovery

import (
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

func verifyStressAttempt(value Evidence) Result {
	var manifest stressAttemptManifest
	if err := decodeAttemptValue(value.AttemptManifest, 2<<20, &manifest); err != nil {
		return invalid("decode S4.3 attempt manifest: " + err.Error())
	}
	var receipt replacementAttemptReceipt
	if err := decodeAttemptValue(value.AttemptReceipt, 5<<20, &receipt); err != nil {
		return invalid("decode S4.3 attempt receipt: " + err.Error())
	}
	cellManifest, ok := findStressCell(manifest.Cells, receipt.CellID)
	if manifest.Schema != stressAttemptManifestSchema || len(manifest.SourceCommit) != 40 || manifest.ImageID == "" ||
		manifest.ToolImageID == "" || len(manifest.Topology) == 0 || hexDigest(manifest.Topology) != manifest.TopologyDigest ||
		!ok || receipt.Schema != replacementAttemptReceiptSchema || !strings.HasPrefix(receipt.AttemptID, "attempt-") ||
		receipt.ManifestDigest != cellManifest.ManifestDigest || receipt.Observation != "complete" ||
		receipt.Cleanup != "complete" || receipt.ActiveNanos <= 0 {
		return invalid("S4.3 attempt identity or lifecycle result is invalid")
	}
	if !validStressPrerequisites(manifest.Prerequisites, manifest.SourceCommit) {
		return invalid("S4.3 prerequisite references are invalid")
	}
	hostScope, err := decodeHostScope(manifest.HostScope)
	if err != nil || !validHostScope(hostScope, manifest.SourceCommit, manifest.ImageID) {
		return invalid("S4.3 HostScope is invalid")
	}
	var cleanup cleanup
	if err := decodeAttemptValue(receipt.CleanupEvidence, 64<<10, &cleanup); err != nil ||
		!validCleanupObservation(cleanup, hostScope) || !validDockerCleanupProjection(cleanup, hostScope) {
		return invalid("S4.3 cleanup observation is invalid")
	}
	commitment, err := commitRouteCase(manifest.RouteCase)
	candidates, candidateErr := verifyReplacementCandidates(manifest.Candidates)
	if err != nil || candidateErr != nil || commitment != manifest.RouteManifest ||
		!candidateCasesMatch(manifest.RouteCase.Candidates, manifest.Candidates) {
		return invalid(errors.Join(err, candidateErr, errors.New("S4.3 Route case is invalid")).Error())
	}
	if receipt.Candidate != "pass" {
		return fail(receipt.Reason)
	}
	if cellManifest.Mode == "overlap" {
		var cell replacementCell
		if err := decodeAttemptValue(receipt.Evidence, 4<<20, &cell); err != nil || cell.Mode != "overlap" ||
			cell.Direction != cellManifest.Direction || cell.CellManifestDigest != cellManifest.ManifestDigest ||
			receipt.ActiveNanos != cell.TerminalNanos {
			return invalid("S4.3 overlap evidence differs from its manifest")
		}
		return verifyReplacementCell(cell, candidates, manifest.RouteCase, manifest.RouteManifest,
			manifest.ImageID, hostScope)
	}
	var cell impairedCell
	if err := decodeAttemptValue(receipt.Evidence, 5<<20, &cell); err != nil ||
		cell.Direction != cellManifest.Direction || cell.Mode != cellManifest.Mode ||
		cell.CellManifestDigest != cellManifest.ManifestDigest || receipt.ActiveNanos != cell.TerminalNanos {
		return invalid("S4.3 impaired evidence differs from its manifest")
	}
	return verifyImpairedCell(cell, manifest, hostScope)
}

func verifyImpairedCell(cell impairedCell, manifest stressAttemptManifest, scope hostScopeEvidence) Result {
	if cell.Mode != "impaired" || cell.Direction != "client-to-publisher" && cell.Direction != "publisher-to-client" ||
		cell.Bytes != 192<<20 || cell.Seed == [32]byte{} || cell.ExpectedDigest != workloadDigest(cell.Seed, cell.Bytes) ||
		cell.ObservedDigest != cell.ExpectedDigest || cell.MeasurementCompletedAtNanos-cell.ActiveStartedAtNanos < int64(10*time.Minute) ||
		cell.MeasurementCompletedAtNanos-cell.ActiveStartedAtNanos > int64(10*time.Minute+2*time.Second) ||
		cell.TerminalNanos < cell.MeasurementCompletedAtNanos || !cell.Ordered || !cell.Unique || !cell.SameConnection ||
		cell.ApplicationReconnected || !cell.TerminalClean || cell.ClientRouteGeneration != 1 ||
		cell.PublisherRouteGeneration != 1 || cell.ClientRecoveryCount != 0 || cell.PublisherRecoveryCount != 0 ||
		cell.ClientApplicationAccepts != 1 || cell.PublisherApplicationAccepts != 1 ||
		cell.ClientRouteAccepts != 1 || cell.PublisherRouteAccepts != 1 || cell.ClientContinuity == [32]byte{} ||
		cell.ClientContinuity != cell.PublisherContinuity || cell.ClientQueueHighWater > queueLimit ||
		cell.PublisherQueueHighWater > queueLimit {
		return fail("S4.3 impaired same-connection or workload invariant failed")
	}
	if !impairedByteCounts(cell) {
		return fail("S4.3 impaired byte or acknowledgement invariant failed")
	}
	if result := verifyStressProcesses(cell.HostProcesses, scope,
		[]string{"client-endpoint", "publisher-endpoint", "client-app", "publisher-app", "client", "publisher"}); result.Verdict != "pass" {
		return result
	}
	before, beforeResult := verifyDirectBaseline(cell.DirectBefore, cell.Direction, manifest, scope)
	after, afterResult := verifyDirectBaseline(cell.DirectAfter, cell.Direction, manifest, scope)
	if beforeResult.Verdict != "pass" {
		return beforeResult
	}
	if afterResult.Verdict != "pass" {
		return afterResult
	}
	if math.Max(before, after)/math.Min(before, after) > 1.10 {
		return invalid("S4.3 impaired direct baselines exceed the 10% drift bound")
	}
	threshold := math.Min(2_000_000, .25*(before+after)/2)
	windows, progressResult := verifiedProgress(cell.Progress, int64(10*time.Minute))
	if progressResult.Verdict != "pass" || len(windows) != 10 {
		return progressResult
	}
	sort.Float64s(windows)
	if windows[0] < threshold || cell.MeasurementDelivered == 0 ||
		cell.MeasurementDelivered != cell.Progress[len(cell.Progress)-1].Delivered {
		return fail("S4.3 impaired goodput misses its paired direct-baseline floor")
	}
	if _, result := verifyResourceSamples(cell.ResourceSamples); result.Verdict != "pass" {
		return result
	}
	if result := verifyStressResourceInterval(cell.ResourceSamples,
		cell.MeasurementCompletedAtNanos-cell.ActiveStartedAtNanos); result.Verdict != "pass" {
		return result
	}
	if !validTrafficInterval(cell.TrafficStart, cell.TrafficEnd, cell.MeasurementDelivered) {
		return fail("S4.3 impaired carrier ratio or bitrate bound failed")
	}
	if result := verifyShapers(cell.Shapers, manifest.ToolImageID, scope, cell.ActiveStartedAtNanos,
		cell.MeasurementCompletedAtNanos); result.Verdict != "pass" {
		return result
	}
	return Result{Verdict: "pass"}
}

func impairedByteCounts(cell impairedCell) bool {
	clientSend, clientReceive, publisherSend, publisherReceive := cell.Bytes, uint32(0), uint32(0), cell.Bytes
	if cell.Direction == "publisher-to-client" {
		clientSend, clientReceive, publisherSend, publisherReceive = 0, cell.Bytes, cell.Bytes, 0
	}
	return cell.ClientAcceptedBytes == clientSend && cell.ClientAcknowledgedBytes == clientSend &&
		cell.ClientReceivedBytes == clientReceive && cell.PublisherAcceptedBytes == publisherSend &&
		cell.PublisherAcknowledgedBytes == publisherSend && cell.PublisherReceivedBytes == publisherReceive
}

func verifyDirectBaseline(value directBaseline, direction string, manifest stressAttemptManifest,
	scope hostScopeEvidence) (float64, Result) {
	duration := value.ActiveEndedAtNanos - value.ActiveStartedAtNanos
	if value.Direction != direction || value.Bytes != 256<<20 || value.Seed == [32]byte{} ||
		value.ExpectedDigest != workloadDigest(value.Seed, value.Bytes) || value.ObservedDigest != value.ExpectedDigest ||
		duration < int64(60*time.Second) || duration > int64(62*time.Second) || value.TerminalNanos < value.ActiveEndedAtNanos {
		return 0, invalid("S4.3 direct baseline identity or workload is invalid")
	}
	windows, result := verifiedProgress(value.Progress, int64(time.Minute))
	if result.Verdict != "pass" || len(windows) != 1 || value.MeasurementDelivered != value.Progress[len(value.Progress)-1].Delivered {
		return 0, result
	}
	if result := verifyShapers(value.Shapers, manifest.ToolImageID, scope, value.ActiveStartedAtNanos,
		value.ActiveEndedAtNanos); result.Verdict != "pass" {
		return 0, result
	}
	if result := verifyStressProcesses(value.Processes, scope,
		[]string{"direct-client", "direct-publisher"}); result.Verdict != "pass" {
		return 0, result
	}
	return windows[0], Result{Verdict: "pass"}
}

func validStressPrerequisites(values []replacementPrerequisite, source string) bool {
	if len(values) != 3 {
		return false
	}
	for index, stage := range []string{"S4.1", "S4.2", "Stage 3"} {
		if values[index].Stage != stage || values[index].SourceCommit != source || len(values[index].EvidenceDigest) != 64 {
			return false
		}
	}
	return true
}

func findStressCell(values []replacementAttemptCell, id string) (replacementAttemptCell, bool) {
	want := []replacementAttemptCell{{CellID: "c2p-overlap", Direction: "client-to-publisher", Mode: "overlap"},
		{CellID: "c2p-impaired", Direction: "client-to-publisher", Mode: "impaired"},
		{CellID: "p2c-impaired", Direction: "publisher-to-client", Mode: "impaired"}}
	if len(values) != len(want) {
		return replacementAttemptCell{}, false
	}
	found := replacementAttemptCell{}
	for index := range want {
		if values[index].CellID != want[index].CellID || values[index].Direction != want[index].Direction ||
			values[index].Mode != want[index].Mode || values[index].ManifestDigest == "" {
			return replacementAttemptCell{}, false
		}
		if values[index].CellID == id {
			found = values[index]
		}
	}
	return found, found.CellID != ""
}
