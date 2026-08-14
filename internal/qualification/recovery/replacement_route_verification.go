package recovery

import "time"

func verifyReplacementRoutes(cell replacementCell, candidates map[string][]replacementCandidate,
	selectionSeed [32]byte, hostScope hostScopeEvidence, identities map[string]bool) Result {
	nodeProcesses := map[[32]byte]candidateProcess{}
	resourceProcesses := map[string]candidateProcess{}
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
				!validProcessRef(process, hostScope) ||
				process.ObservedAtNanos < cell.HostStartedAtNanos ||
				process.ObservedAtNanos > cell.HostStartedAtNanos+cell.TerminalNanos {
				return invalid("S4.2 selected candidate process does not match the recomputed Route")
			}
			if prior, ok := nodeProcesses[process.NodeID]; ok &&
				(!sameProcessIncarnation(prior, process) || process.ObservedAtNanos < prior.ObservedAtNanos) {
				return invalid("S4.2 unchanged Node changed process incarnation")
			}
			if prior, ok := resourceProcesses[process.Host.Identity]; ok &&
				(prior.NodeID != process.NodeID || !sameProcessIncarnation(prior, process)) {
				return invalid("S4.2 process identity or projection changed ownership")
			}
			_, knownResource := resourceProcesses[process.Host.Identity]
			if identities[process.Host.Identity] && !knownResource {
				return invalid("S4.2 candidate process overlaps Endpoint or Application identity")
			}
			nodeProcesses[process.NodeID], resourceProcesses[process.Host.Identity] = process, process
			identities[process.Host.Identity] = true
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
			!sameProcessIncarnation(event.Failed, before.Processes[event.Role]) ||
			!sameProcessIncarnation(event.Replacement, after.Processes[event.Role]) ||
			sameProcessIncarnation(event.Failed, event.Replacement) ||
			event.FailedResource.Running ||
			event.FailedResource.ObservedAtNanos < cell.TerminalNanos ||
			!validProcessState(event.FailedResource.State, event.Failed) ||
			!validProcessFault(event.FailedResource.Fault, event.Failed) ||
			event.FailedResource.ObservedAtNanos+cell.HostStartedAtNanos !=
				event.FailedResource.State.ObservedAtNanos ||
			event.Failed.ObservedAtNanos > event.FailedResource.Fault.InvocationStartedNanos ||
			event.FailedResource.Fault.InvocationStartedNanos-cell.HostStartedAtNanos != event.FaultAtNanos ||
			event.FailedResource.Fault.InvocationCompletedNanos-cell.HostStartedAtNanos > event.CanaryNanos ||
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
		if sameProcessIncarnation(cell.Routes[generation].Processes["introduction"], event.Introduction) {
			introductionAttachment++
		}
	}
	if !sameProcessIncarnation(event.RendezvousBefore, before.Processes["rendezvous"]) ||
		!sameProcessIncarnation(event.RendezvousAfter, after.Processes["rendezvous"]) ||
		!sameProcessIncarnation(event.Introduction, after.Processes["introduction"]) ||
		event.IntroductionAttachment == 0 ||
		event.IntroductionAttachment != introductionAttachment {
		return invalid("S4.2 Rendezvous or fresh Introduction evidence is incomplete")
	}
	if event.Role == "rendezvous" {
		if event.Layer != "rendezvous" || sameProcessIncarnation(event.RendezvousBefore, event.RendezvousAfter) ||
			event.IntroductionSetupReceipt == [32]byte{} || event.IntroductionSetupAttachment != 3 ||
			event.IntroductionOpaqueBytes == 0 || event.IntroductionOpaqueDigest == [32]byte{} ||
			len(cell.Proposals) < 3 || event.IntroductionSetupReceipt != cell.Proposals[2].IntroductionReceipt {
			return fail("S4.2 Rendezvous loss did not commit a distinct Rendezvous")
		}
	} else if event.Layer != "leg" || !sameProcessIncarnation(event.RendezvousBefore, event.RendezvousAfter) ||
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
