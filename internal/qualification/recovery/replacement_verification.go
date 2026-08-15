package recovery

import (
	"strings"
	"time"
)

func verifyReplacementCell(cell replacementCell, candidates map[string][]replacementCandidate,
	routeCase routeCase, routeManifest [32]byte, imageID string, hostScope hostScopeEvidence) Result {
	selectionSeed := routeCase.SelectionSeed
	if cell.Direction != "client-to-publisher" && cell.Direction != "publisher-to-client" ||
		cell.Bytes != streamBytes || cell.Seed == [32]byte{} || cell.ExpectedDigest != workloadDigest(cell.Seed, cell.Bytes) ||
		cell.ObservedDigest != cell.ExpectedDigest || cell.HostStartedAtNanos <= 0 ||
		cell.ActiveStartedAtNanos <= cell.HostStartedAtNanos || cell.TerminalNanos <= 0 {
		return invalid("S4.2 replacement cell identity or workload is incomplete")
	}
	if result := verifyReplacementManifest(cell); result.Verdict != "pass" {
		return result
	}
	if len(cell.Routes) != len(cell.Events)+1 || len(cell.Events) == 0 || len(cell.Events) > 3 ||
		cell.ClientRouteGeneration != uint64(len(cell.Routes)) || cell.PublisherRouteGeneration != uint64(len(cell.Routes)) ||
		cell.ClientRecoveryCount != uint32(len(cell.Events)) || cell.PublisherRecoveryCount != uint32(len(cell.Events)) ||
		cell.ClientApplicationAccepts != 1 || cell.PublisherApplicationAccepts != 1 ||
		cell.ClientRouteAccepts != uint32(len(cell.Routes)) || cell.PublisherRouteAccepts != uint32(len(cell.Routes)) ||
		cell.ClientContinuity == [32]byte{} || cell.ClientContinuity != cell.PublisherContinuity ||
		!cell.Ordered || !cell.Unique || !cell.SameConnection || cell.ApplicationReconnected || !cell.TerminalClean {
		return fail("S4.2 same-connection generation or Application invariant failed")
	}
	clientSend, clientReceive := cell.Bytes, uint32(0)
	publisherSend, publisherReceive := uint32(0), cell.Bytes
	if cell.Direction == "publisher-to-client" {
		clientSend, clientReceive, publisherSend, publisherReceive = 0, cell.Bytes, cell.Bytes, 0
	}
	if cell.ClientAcceptedBytes != clientSend || cell.ClientAcknowledgedBytes != clientSend ||
		cell.ClientReceivedBytes != clientReceive || cell.PublisherAcceptedBytes != publisherSend ||
		cell.PublisherAcknowledgedBytes != publisherSend || cell.PublisherReceivedBytes != publisherReceive ||
		cell.ClientQueueHighWater > 256<<10 || cell.PublisherQueueHighWater > 256<<10 ||
		(clientSend > 0 && cell.ClientQueueHighWater == 0) ||
		(publisherSend > 0 && cell.PublisherQueueHighWater == 0) {
		return fail("S4.2 logical range, acknowledgement, or queue bound failed")
	}
	if cell.FinalCanaryOffset != cell.Bytes-32 || cell.FinalCanary != workloadRange(cell.Seed, cell.FinalCanaryOffset) ||
		cell.FinalCanaryObservedNanos < cell.TerminalNanos ||
		cell.FinalCanaryObservedNanos-cell.TerminalNanos > int64(30*time.Second) {
		return fail("S4.2 final same-stream canary is missing or unbound")
	}
	identities := map[string]bool{}
	if result := verifyReplacementEndpointProcesses(cell, hostScope, identities); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementRouteProcesses(cell, hostScope, identities); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementProposals(cell, candidates, routeCase, routeManifest, hostScope, identities); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementRoutes(cell, candidates, selectionSeed, hostScope, identities); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementTraffic(cell); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementResources(cell); result.Verdict != "pass" {
		return result
	}
	if cell.Mode == "sequential-three" {
		if len(cell.Events) != 3 || cell.TerminalNanos < int64(10*time.Minute) || cell.TerminalNanos > int64(13*time.Minute) {
			return fail("S4.2 sequential connection did not remain continuously loaded for ten minutes")
		}
	} else if !strings.HasPrefix(cell.Mode, "isolated-") || len(cell.Events) != 1 ||
		cell.Mode != "isolated-"+cell.Events[0].Role || cell.TerminalNanos > int64(time.Minute) {
		return invalid("S4.2 isolated role cell is malformed")
	}
	return Result{Verdict: "pass"}
}
