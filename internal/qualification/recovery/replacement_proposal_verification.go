package recovery

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"time"
)

func verifyReplacementProposals(cell replacementCell, candidates map[string][]replacementCandidate,
	routeCase routeCase, routeManifest [32]byte, hostScope hostScopeEvidence, reserved map[string]bool) Result {
	selectionSeed := routeCase.SelectionSeed
	expected := 2
	if cell.Mode == "isolated-rendezvous" {
		expected = 3
	} else if cell.Mode == "overlap" {
		expected = 3
	} else if cell.Mode == "sequential-three" {
		expected = 4
	}
	if len(cell.Proposals) != expected {
		return invalid("S4.2 proposed Route evidence count is incomplete")
	}
	processes := map[[32]byte]candidateProcess{}
	resources := map[string]candidateProcess{}
	for proposalIndex, proposal := range cell.Proposals {
		selected, err := layeredCandidates(candidates, selectionSeed, proposalIndex)
		if err != nil || proposal.Attachment != uint32(proposalIndex+1) ||
			!slices.Equal(proposal.ExcludedIdentities, layeredExcluded(candidates, selectionSeed, proposalIndex)) {
			return invalid("S4.2 proposed Route selection or exclusions are invalid")
		}
		for roleIndex, role := range replacementRoles {
			if proposal.NodeIDs[roleIndex] != selected[role].NodeID ||
				proposal.PublicKeys[roleIndex] != selected[role].PublicKey {
				return invalid("S4.2 proposed Route differs from the authenticated Candidate View")
			}
			process, stopped := proposal.Processes[roleIndex], proposal.Stopped[roleIndex]
			if process.NodeID != selected[role].NodeID || process.PublicKey != selected[role].PublicKey ||
				!validProcessRef(process, hostScope) ||
				process.ObservedAtNanos < cell.HostStartedAtNanos ||
				process.ObservedAtNanos > cell.ActiveStartedAtNanos ||
				!validProcessState(stopped.State, process) ||
				stopped.ObservedAtNanos+cell.ActiveStartedAtNanos != stopped.State.ObservedAtNanos ||
				process.ObservedAtNanos > stopped.State.ObservedAtNanos ||
				reserved[process.Host.Identity] ||
				stopped.Running || stopped.ObservedAtNanos < cell.TerminalNanos ||
				stopped.ObservedAtNanos-cell.TerminalNanos > int64(30*time.Second) {
				return invalid("S4.2 proposed Route process or stopped receipt is invalid")
			}
			if prior, ok := processes[process.NodeID]; ok &&
				(!sameProcessIncarnation(prior, process) || process.ObservedAtNanos < prior.ObservedAtNanos) {
				return invalid("S4.2 one proposed Node changed process incarnation")
			}
			if prior, ok := resources[process.Host.Identity]; ok &&
				(prior.NodeID != process.NodeID || !sameProcessIncarnation(prior, process)) {
				return invalid("S4.2 proposed process identity or projection changed ownership")
			}
			processes[process.NodeID] = process
			resources[process.Host.Identity] = process
		}
		wantCommitted := cell.Mode != "isolated-rendezvous" && cell.Mode != "overlap" || proposalIndex != 1
		wantTerminal := "success"
		if cell.Mode == "overlap" && proposalIndex == 1 {
			wantTerminal = "error"
		}
		if proposal.Committed != wantCommitted || proposal.Terminal != wantTerminal ||
			(proposalIndex == 2) != (proposal.IntroductionReceipt != [32]byte{}) {
			return fail("S4.2 proposal outcome or sealed Introduction receipt is inconsistent")
		}
		if proposalIndex == 2 && !validIntroductionProof(proposal, routeCase, routeManifest, selected) {
			return invalid("S4.2 sealed Introduction proof is not bound to the selected Route")
		}
		if result := verifyReplacementPlanTimings(cell, proposalIndex); result.Verdict != "pass" {
			return result
		}
	}
	for generationIndex, generation := range cell.Routes {
		proposalIndex := generationIndex
		if (cell.Mode == "isolated-rendezvous" || cell.Mode == "overlap") && generationIndex > 0 {
			proposalIndex++
		}
		proposal := cell.Proposals[proposalIndex]
		for roleIndex, role := range replacementRoles {
			proposed, observed := proposal.Processes[roleIndex], generation.Processes[role]
			if !sameProcessIncarnation(proposed, observed) || proposed.ObservedAtNanos > observed.ObservedAtNanos {
				return invalid("S4.2 committed generation process differs from its proposed Route")
			}
		}
	}
	return Result{Verdict: "pass"}
}

func validIntroductionProof(proposal replacementProposal, routeCase routeCase, routeManifest [32]byte,
	selected map[string]replacementCandidate) bool {
	proof := proposal.IntroductionProof
	profile := sha256.Sum256([]byte(routeCase.Profile))
	capabilities := sha256.Sum256([]byte("ardents-h3-recovery-setup-capabilities-v1\x00tls13|single-use|no-application-data"))
	reachability := sha256.Sum256(append([]byte("ardents-h3-rendezvous-reachability-v1\x00"),
		selected["rendezvous"].Endpoint...))
	expires := time.Unix(routeCase.SelectionAt, 0).Add(time.Hour).UnixNano()
	if proof.ManifestDigest != routeManifest || proof.NetworkID != routeCase.NetworkID ||
		proof.EpochDigest != routeCase.EpochDigest || proof.ViewRoot != routeCase.ViewRoot ||
		proof.ProfileDigest != profile || proof.CapabilitiesDigest != capabilities ||
		proof.IntroductionNode != selected["introduction"].NodeID ||
		proof.RendezvousNode != selected["rendezvous"].NodeID || proof.RendezvousReachability != reachability ||
		proof.JoinHandle == [32]byte{} || proof.EndpointHandshake == [32]byte{} || proof.Reply == [32]byte{} ||
		proof.ExpiresAtNanos != expires {
		return false
	}
	body := make([]byte, 397)
	copy(body[:5], "ASIS\x02")
	fields := [][32]byte{proof.ManifestDigest, proof.NetworkID, proof.EpochDigest, proof.ViewRoot, proof.ProfileDigest,
		proof.CapabilitiesDigest, proof.IntroductionNode, proof.RendezvousNode, proof.RendezvousReachability,
		proof.JoinHandle, proof.EndpointHandshake}
	for index, field := range fields {
		copy(body[5+index*32:5+(index+1)*32], field[:])
	}
	binary.BigEndian.PutUint64(body[357:365], uint64(proof.ExpiresAtNanos))
	transcript := sha256.Sum256(append([]byte("ardents-h3-sealed-introduction-transcript-v2\x00"), body[:365]...))
	if proof.TranscriptContext != transcript {
		return false
	}
	copy(body[365:], transcript[:])
	receiptInput := append([]byte("ardents-h3-sealed-introduction-v2\x00"), body...)
	receiptInput = append(receiptInput, proof.Reply[:]...)
	return proposal.IntroductionReceipt == sha256.Sum256(receiptInput)
}
