package recovery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

func verifyReplacementEvidence(value Evidence, prior map[string]bool, hostScope hostScopeEvidence,
	campaignCompletedAt int64) Result {
	var replacement replacementEvidence
	decoder := json.NewDecoder(bytes.NewReader(value.S42))
	decoder.DisallowUnknownFields()
	if len(value.S42) == 0 || len(value.S42) > 3<<20 {
		return invalid("S4.2 evidence extension is malformed")
	}
	if err := decoder.Decode(&replacement); err != nil {
		return invalid("decode S4.2 evidence extension: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return invalid("S4.2 evidence extension contains multiple values")
	}
	routeCommitment, err := commitRouteCase(replacement.RouteCase)
	if err != nil {
		return invalid("encode S4.2 Candidate View commitment: " + err.Error())
	}
	if routeCommitment != value.CandidateView || replacement.RouteCase.NetworkID != value.NetworkID ||
		replacement.RouteCase.Profile != "h3-route-tracer-v1" || replacement.RouteCase.ManifestDigest != [32]byte{} ||
		len(replacement.RouteCase.RawEvidence) != 0 || len(replacement.RouteCase.Candidates) != len(replacement.Candidates) {
		return invalid("S4.2 Candidate View manifest binding is incomplete")
	}
	byRole, err := verifyReplacementCandidates(replacement.Candidates)
	if err != nil || !candidateCasesMatch(replacement.RouteCase.Candidates, replacement.Candidates) {
		return invalid(errors.Join(err, errors.New("S4.2 public candidates differ from the frozen Route case")).Error())
	}
	if err := verifyReplacementTopology(value.Topology); err != nil {
		return invalid(err.Error())
	}
	if len(replacement.Cells) != 10 {
		return invalid("S4.2 requires five replacement cells in each direction")
	}
	switch hostScope.Adapter {
	case "docker-compose-v1":
		if !validDockerReplacementProcesses(replacement, hostScope) {
			return invalid("S4.2 Docker Adapter projection is invalid")
		}
	default:
		return invalid("S4.2 host-observation Adapter is unsupported")
	}
	required := make(map[string]bool, 10)
	var priorHostTerminal int64
	hostEnds := make([]int64, 0, len(replacement.Cells))
	for index := range replacement.Cells {
		cell := replacement.Cells[index]
		hostTerminal := cell.HostStartedAtNanos + cell.TerminalNanos
		if hostTerminal <= cell.HostStartedAtNanos || cell.HostStartedAtNanos < priorHostTerminal {
			return invalid("S4.2 host-observation clock chronology is invalid")
		}
		if result := verifyReplacementCell(cell, byRole, replacement.RouteCase, value.Manifest.RouteManifest,
			value.ImageID, hostScope); result.Verdict != "pass" {
			return result
		}
		hostEnd := replacementCellHostEnd(cell, hostTerminal)
		hostEnds = append(hostEnds, hostEnd)
		key := cell.Direction + ":" + cell.Mode
		if required[key] {
			return invalid("S4.2 replacement cell was duplicated")
		}
		required[key] = true
		for identity := range replacementCellIdentities(cell) {
			if prior[identity] {
				return invalid("S4.2 cells reused a retained process identity")
			}
			prior[identity] = true
		}
		priorHostTerminal = hostEnd
	}
	for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		for _, mode := range []string{"isolated-initiator", "isolated-introduction", "isolated-rendezvous",
			"isolated-responder", "sequential-three"} {
			if !required[direction+":"+mode] {
				return invalid("mandatory S4.2 replacement cell is missing")
			}
		}
	}
	if priorHostTerminal > campaignCompletedAt {
		return invalid("S4.2 host observation follows campaign completion")
	}
	if !validReplacementInterleaving(value.Cells, replacement.Cells, hostEnds) {
		return invalid("S4.2 base and replacement cell chronology is invalid")
	}
	return Result{Verdict: "pass"}
}

func validReplacementInterleaving(base []Cell, cells []replacementCell, ends []int64) bool {
	modes := []string{"isolated-initiator", "isolated-introduction", "isolated-rendezvous",
		"isolated-responder", "sequential-three"}
	if len(base) != 2 || len(cells) != 10 || len(ends) != len(cells) {
		return false
	}
	for index := range cells {
		direction := "client-to-publisher"
		if index >= len(modes) {
			direction = "publisher-to-client"
		}
		if cells[index].Direction != direction || cells[index].Mode != modes[index%len(modes)] {
			return false
		}
	}
	return base[0].Direction == "client-to-publisher" && base[1].Direction == "publisher-to-client" &&
		base[0].HostCompletedAtNanos <= cells[0].HostStartedAtNanos &&
		ends[4] <= base[1].HostStartedAtNanos &&
		base[1].HostCompletedAtNanos <= cells[5].HostStartedAtNanos
}

func replacementCellHostEnd(cell replacementCell, current int64) int64 {
	for _, route := range cell.Routes {
		for _, process := range route.Processes {
			current = max(current, process.ObservedAtNanos)
		}
	}
	for _, proposal := range cell.Proposals {
		for index, process := range proposal.Processes {
			current = max(current, process.ObservedAtNanos, proposal.Stopped[index].State.ObservedAtNanos)
		}
	}
	for _, event := range cell.Events {
		current = max(current, event.FailedResource.State.ObservedAtNanos,
			event.FailedResource.Fault.InvocationCompletedNanos, event.FailedResource.Fault.ObservedAtNanos)
	}
	return current
}

func verifyReplacementCell(cell replacementCell, candidates map[string][]replacementCandidate,
	routeCase routeCase, routeManifest [32]byte, imageID string, hostScope hostScopeEvidence) Result {
	selectionSeed := routeCase.SelectionSeed
	if cell.Direction != "client-to-publisher" && cell.Direction != "publisher-to-client" ||
		cell.Bytes != streamBytes || cell.Seed == [32]byte{} || cell.ExpectedDigest != workloadDigest(cell.Seed, cell.Bytes) ||
		cell.ObservedDigest != cell.ExpectedDigest || cell.HostStartedAtNanos <= 0 || cell.TerminalNanos <= 0 {
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
	if result := verifyReplacementProposals(cell, candidates, routeCase, routeManifest, hostScope, identities); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementRoutes(cell, candidates, selectionSeed, hostScope, identities); result.Verdict != "pass" {
		return result
	}
	if result := verifyReplacementObservers(cell, imageID, identities); result.Verdict != "pass" {
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

func verifyReplacementTopology(raw []byte) error {
	blocks := composeServiceBlocks(string(raw))
	for _, role := range replacementRoles {
		for _, suffix := range []string{"-2", "-3"} {
			block := blocks[role+suffix]
			if block == "" || !strings.Contains(block, "route_net") || !strings.Contains(block, "restart: \"no\"") ||
				strings.Contains(block, "carrier_net") || strings.Contains(block, "network_mode:") {
				return errors.New("S4.2 alternate candidate topology is incomplete or widened")
			}
		}
	}
	if !strings.Contains(blocks["client"], "/run/ardents/recovery-introduction-user") ||
		!strings.Contains(blocks["introduction-3"], "/run/ardents/recovery-introduction-user") ||
		!strings.Contains(blocks["introduction-3"], "/run/ardents/recovery-introduction-service") ||
		!strings.Contains(blocks["publisher"], "/run/ardents/recovery-introduction-service") {
		return errors.New("S4.2 sealed Introduction control path is absent")
	}
	allowedUserControl := map[string]bool{"volume-init": true, "client": true, "introduction-3": true}
	allowedServiceControl := map[string]bool{"volume-init": true, "introduction-3": true, "publisher": true}
	for name, block := range blocks {
		if strings.Contains(block, "/run/ardents/recovery-introduction-user") && !allowedUserControl[name] ||
			strings.Contains(block, "/run/ardents/recovery-introduction-service") && !allowedServiceControl[name] {
			return errors.New("S4.2 sealed Introduction control path reached an unrelated process")
		}
	}
	return nil
}
