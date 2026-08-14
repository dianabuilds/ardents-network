package recovery

import (
	"strings"
	"time"
)

func verifyReplacementRoutes(cell replacementCell, candidates map[string][]replacementCandidate,
	selectionSeed [32]byte, identities map[string]bool) Result {
	nodeProcesses := map[[32]byte]candidateProcess{}
	containerNodes := map[string][32]byte{}
	failedRoles := map[string]bool{}
	for generationIndex, generation := range cell.Routes {
		if generation.Generation != uint64(generationIndex+1) || len(generation.Processes) != len(replacementRoles) {
			return invalid("S4.2 Route generation numbering or cardinality is invalid")
		}
		proposal := generationIndex
		if cell.Mode == "isolated-rendezvous" && generationIndex > 0 {
			proposal++
		}
		selected, err := layeredCandidates(candidates, selectionSeed, proposal)
		if err != nil {
			return fail(err.Error())
		}
		for _, role := range replacementRoles {
			process := generation.Processes[role]
			candidate := selected[role]
			if process.NodeID != candidate.NodeID || process.PublicKey != candidate.PublicKey ||
				!strings.HasPrefix(process.Service, role) || !fullContainerID(process.ContainerID) || process.PID == 0 {
				return invalid("S4.2 selected candidate process does not match the recomputed Route")
			}
			if _, ok := processStartedAt(process.Incarnation, process.ContainerID); !ok {
				return invalid("S4.2 selected candidate process incarnation is invalid")
			}
			if prior, ok := nodeProcesses[process.NodeID]; ok && prior != process {
				return invalid("S4.2 unchanged Node changed process incarnation")
			}
			if prior, ok := containerNodes[process.ContainerID]; ok && prior != process.NodeID {
				return invalid("S4.2 one process represented multiple Node candidates")
			}
			if identities[process.ContainerID] && containerNodes[process.ContainerID] == [32]byte{} {
				return invalid("S4.2 candidate process overlaps Endpoint or Application identity")
			}
			nodeProcesses[process.NodeID], containerNodes[process.ContainerID] = process, process.NodeID
			identities[process.ContainerID] = true
		}
		if generationIndex == len(cell.Events) {
			continue
		}
		event := cell.Events[generationIndex]
		if !replacementRole(event.Role) || failedRoles[event.Role] && cell.Mode == "sequential-three" {
			return invalid("S4.2 sequential failures did not strike distinct roles")
		}
		failedRoles[event.Role] = true
		before, after := generation, cell.Routes[generationIndex+1]
		if event.GenerationBefore != before.Generation || event.GenerationAfter != after.Generation ||
			event.Failed != before.Processes[event.Role] || event.Replacement != after.Processes[event.Role] ||
			event.Failed == event.Replacement || event.FailedResource.ContainerID != event.Failed.ContainerID ||
			event.FailedResource.Running || !fullContainerID(event.FailedResource.ContainerID) ||
			event.FailedResource.ObservedAtNanos < cell.TerminalNanos ||
			event.FailedResource.ObservedAtNanos-cell.TerminalNanos > int64(30*time.Second) {
			return fail("S4.2 failed and replacement candidate evidence is inconsistent")
		}
		if result := verifyReplacementEvent(cell, generationIndex, event, before, after); result.Verdict != "pass" {
			return result
		}
	}
	return Result{Verdict: "pass"}
}

func verifyReplacementEvent(cell replacementCell, index int, event replacementEvent,
	before, after routeGeneration) Result {
	expectedOffset := uint32(17 * 16_381)
	if cell.Mode == "sequential-three" {
		expectedOffset = uint32((index + 1) * 64 * 16_381)
	}
	if event.FaultOffset != expectedOffset || event.CanaryOffset != expectedOffset ||
		event.Canary != workloadRange(cell.Seed, event.CanaryOffset) || event.FaultOffset < 256<<10 ||
		event.FaultOffset%16_384 == 0 || event.CanaryOffset+32 > cell.Bytes ||
		event.LastDeliveryNanos <= 0 || event.FaultAtNanos < event.LastDeliveryNanos ||
		event.CanaryNanos <= event.FaultAtNanos || event.CanaryNanos-event.LastDeliveryNanos > int64(5*time.Second) ||
		event.CanaryNanos > cell.TerminalNanos {
		return fail("S4.2 replacement event timing, offset, or canary is invalid")
	}
	introductionAttachment := uint32(0)
	for generation := 0; generation <= index+1; generation++ {
		if cell.Routes[generation].Processes["introduction"] == event.Introduction {
			introductionAttachment++
		}
	}
	if event.RendezvousBefore != before.Processes["rendezvous"] ||
		event.RendezvousAfter != after.Processes["rendezvous"] || event.Introduction != after.Processes["introduction"] ||
		event.IntroductionAttachment == 0 ||
		event.IntroductionAttachment != introductionAttachment {
		return invalid("S4.2 Rendezvous or fresh Introduction evidence is incomplete")
	}
	if event.Role == "rendezvous" {
		if event.Layer != "rendezvous" || event.RendezvousBefore == event.RendezvousAfter ||
			event.IntroductionSetupReceipt == [32]byte{} || event.IntroductionSetupAttachment != 3 ||
			event.IntroductionOpaqueBytes == 0 || event.IntroductionOpaqueDigest == [32]byte{} ||
			len(cell.Proposals) < 3 || event.IntroductionSetupReceipt != cell.Proposals[2].IntroductionReceipt {
			return fail("S4.2 Rendezvous loss did not commit a distinct Rendezvous")
		}
	} else if event.Layer != "leg" || event.RendezvousBefore != event.RendezvousAfter ||
		event.IntroductionSetupReceipt != [32]byte{} || event.IntroductionSetupAttachment != 0 ||
		event.IntroductionOpaqueBytes != 0 || event.IntroductionOpaqueDigest != [32]byte{} {
		return fail("S4.2 leg replacement did not retain the same live Rendezvous")
	}
	return Result{Verdict: "pass"}
}

func candidateCasesMatch(values []routeCandidate, candidates []replacementCandidate) bool {
	if len(values) != len(candidates) {
		return false
	}
	seen := make(map[[32]byte]replacementCandidate, len(candidates))
	for _, candidate := range candidates {
		seen[candidate.NodeID] = candidate
	}
	for _, value := range values {
		candidate, ok := seen[value.NodeID]
		if !ok || candidate.PublicKey != value.PublicKey || candidate.Family != value.Family ||
			candidate.Endpoint != value.Endpoint || candidate.Role != value.Domain || value.Capacity == 0 ||
			value.ValidFrom >= value.ValidUntil {
			return false
		}
	}
	return true
}
