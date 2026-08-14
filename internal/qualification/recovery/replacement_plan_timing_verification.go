package recovery

import "time"

var replacementPlanRoles = [...]string{
	"client", "initiator", "introduction", "rendezvous", "responder", "publisher",
}

func verifyReplacementPlanTimings(cell replacementCell, proposalIndex int) Result {
	proposal := cell.Proposals[proposalIndex]
	if len(proposal.PlanTimings) != len(replacementPlanRoles) ||
		cell.SetupDeadlineNanos%int64(time.Millisecond) != 0 ||
		cell.LifetimeNanos%int64(time.Millisecond) != 0 {
		return invalid("S4.2 Route plan timing evidence is incomplete")
	}
	deadline := uint32(cell.SetupDeadlineNanos / int64(time.Millisecond))
	lifetime := uint32(cell.LifetimeNanos / int64(time.Millisecond))
	for _, role := range replacementPlanRoles {
		timing, ok := proposal.PlanTimings[role]
		process, found := replacementPlanProcess(cell, proposal, role)
		if !proposal.Committed && role == "rendezvous" {
			prior, priorOK := cell.Proposals[proposalIndex-1].PlanTimings[role]
			if !ok || !found || timing.Process != process || !priorOK || timing != prior {
				return invalid("S4.2 stopped Rendezvous plan timing is not bound to its prior runtime evidence")
			}
			continue
		}
		localAttachment := replacementLocalAttachment(cell.Proposals, proposalIndex, role, process)
		if !ok || !found || localAttachment == 0 || timing.Process != process || timing.Attachment != localAttachment ||
			timing.DeadlineMillis != deadline || timing.LifetimeMillis != lifetime {
			return invalid("S4.2 observed Route plan timing differs from the frozen manifest")
		}
	}
	return Result{Verdict: "pass"}
}

func replacementLocalAttachment(proposals []replacementProposal, current int, role string,
	process processEvidenceRef) uint32 {
	if role == "client" || role == "publisher" {
		return proposals[current].Attachment
	}
	var result uint32
	for proposalIndex := 0; proposalIndex <= current; proposalIndex++ {
		candidate, ok := replacementPlanProcess(replacementCell{}, proposals[proposalIndex], role)
		if ok && candidate == process {
			result++
		}
	}
	return result
}

func replacementPlanProcess(cell replacementCell, proposal replacementProposal,
	role string) (processEvidenceRef, bool) {
	if role == "client" || role == "publisher" {
		value, ok := cell.HostProcesses[role]
		return value.Host, ok
	}
	for index, candidateRole := range replacementRoles {
		if role == candidateRole {
			return proposal.Processes[index].Host, true
		}
	}
	return processEvidenceRef{}, false
}
