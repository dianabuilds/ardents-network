package recovery

import "time"

func validReplacementFailurePrefix(cell replacementCell, faults map[string]processFaultEvidence,
	candidates map[string][]replacementCandidate, routeCase routeCase, hostScope hostScopeEvidence) bool {
	if cell.HostStartedAtNanos <= 0 || cell.ActiveStartedAtNanos <= cell.HostStartedAtNanos ||
		cell.TerminalNanos <= 0 || len(cell.Routes) != len(cell.Events)+1 || len(cell.Routes) == 0 {
		return false
	}
	identities := map[string]bool{}
	if verifyReplacementEndpointProcesses(cell, hostScope, identities).Verdict != "pass" ||
		verifyReplacementRouteProcesses(cell, hostScope, identities).Verdict != "pass" {
		return false
	}
	nodes := map[[32]byte]candidateProcess{}
	resources := map[string]candidateProcess{}
	for generationIndex, generation := range cell.Routes {
		proposal := generationIndex
		if cell.Mode == "isolated-rendezvous" && generationIndex > 0 {
			proposal++
		}
		selected, err := layeredCandidates(candidates, routeCase.SelectionSeed, proposal)
		if err != nil || generation.Generation != uint64(generationIndex+1) ||
			len(generation.Processes) != len(replacementRoles) {
			return false
		}
		for _, role := range replacementRoles {
			process, candidate := generation.Processes[role], selected[role]
			latestObservation := cell.ActiveStartedAtNanos
			if generationIndex > 0 {
				latestObservation = cell.ActiveStartedAtNanos + cell.TerminalNanos
			}
			if process.NodeID != candidate.NodeID || process.PublicKey != candidate.PublicKey ||
				!validProcessRef(process, hostScope) || process.ObservedAtNanos < cell.HostStartedAtNanos ||
				process.ObservedAtNanos > latestObservation {
				return false
			}
			if prior, ok := nodes[process.NodeID]; ok && !sameProcessIncarnation(prior, process) {
				return false
			}
			if prior, ok := resources[process.Host.Identity]; ok &&
				(prior.NodeID != process.NodeID || !sameProcessIncarnation(prior, process)) {
				return false
			}
			nodes[process.NodeID], resources[process.Host.Identity] = process, process
		}
		if generationIndex >= len(cell.Events) {
			continue
		}
		event := cell.Events[generationIndex]
		before, after := generation, cell.Routes[generationIndex+1]
		fault, ok := faults[event.Failed.ContainerID]
		if !ok || event.GenerationBefore != before.Generation || event.GenerationAfter != after.Generation ||
			!sameProcessIncarnation(event.Failed, before.Processes[event.Role]) ||
			!sameProcessIncarnation(event.Replacement, after.Processes[event.Role]) ||
			!validProcessFault(fault, event.Failed) ||
			fault.InvocationStartedNanos-cell.ActiveStartedAtNanos != event.FaultAtNanos ||
			fault.InvocationCompletedNanos-cell.ActiveStartedAtNanos > event.CanaryNanos ||
			!validReplacementFailureEvent(cell, generationIndex, event, before, after) {
			return false
		}
	}
	return true
}

func validReplacementFailureEvent(cell replacementCell, index int, event replacementEvent,
	before, after routeGeneration) bool {
	if index >= len(cell.FaultOffsets) || event.FaultOffset != cell.FaultOffsets[index] ||
		event.CanaryOffset != event.FaultOffset || event.Canary != workloadRange(cell.Seed, event.CanaryOffset) ||
		event.LastDeliveryNanos <= 0 || event.FaultAtNanos < event.LastDeliveryNanos ||
		event.CanaryNanos <= event.FaultAtNanos || event.CanaryNanos-event.LastDeliveryNanos > int64(5*time.Second) ||
		event.CanaryNanos > cell.TerminalNanos ||
		!sameProcessIncarnation(event.RendezvousBefore, before.Processes["rendezvous"]) ||
		!sameProcessIncarnation(event.RendezvousAfter, after.Processes["rendezvous"]) ||
		!sameProcessIncarnation(event.Introduction, after.Processes["introduction"]) {
		return false
	}
	if event.Role == "rendezvous" {
		return event.Layer == "rendezvous" &&
			!sameProcessIncarnation(event.RendezvousBefore, event.RendezvousAfter)
	}
	return event.Layer == "leg" && sameProcessIncarnation(event.RendezvousBefore, event.RendezvousAfter)
}

func validCurrentFailureFault(cell replacementCell, failure replacementFailureObservation,
	faults map[string]processFaultEvidence) bool {
	if failure.EventIndex >= len(cell.FailureRoles) || len(cell.Routes) == 0 {
		return false
	}
	process := cell.Routes[len(cell.Routes)-1].Processes[cell.FailureRoles[failure.EventIndex]]
	fault, ok := faults[process.ContainerID]
	return ok && validProcessFault(fault, process) &&
		fault.InvocationStartedNanos >= cell.ActiveStartedAtNanos+failure.LastDeliveryNanos &&
		fault.InvocationCompletedNanos <= cell.ActiveStartedAtNanos+failure.ObservedAtNanos
}
