package placement

import (
	"sort"
	"time"
)

const candidateFreshness = 15 * time.Minute

const minimumCapacityHeadroom int64 = 64 * 1024

func SelectTargets(request SelectionRequest, candidates []Candidate) SelectionDecision {
	decision := SelectionDecision{}
	eligible := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		reason := candidateDenial(request, candidate)
		if reason != "" {
			decision.Denials = append(decision.Denials, Denial{NodeID: candidate.NodeID, Reason: reason})
			continue
		}
		eligible = append(eligible, candidate)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].FailureDomain != eligible[j].FailureDomain {
			return eligible[i].FailureDomain < eligible[j].FailureDomain
		}
		return eligible[i].NodeID < eligible[j].NodeID
	})
	decision.Selected = selectDiverse(eligible, request.Count)
	return decision
}

func candidateDenial(request SelectionRequest, candidate Candidate) string {
	switch {
	case candidate.NodeID == "" || candidate.NodeID == request.OwnerNodeID:
		return "node_ineligible"
	case request.ExcludedNodes[candidate.NodeID]:
		return ReasonExisting
	case candidate.DenialReason != "":
		return candidate.DenialReason
	case !candidate.Trusted:
		return ReasonUntrusted
	case !candidate.CapabilityValid:
		return ReasonCapability
	case !candidate.PolicyAllowed:
		return ReasonPolicy
	case !candidate.Usable:
		return "route_unusable"
	case candidate.CapacityBytes < requiredCapacity(request.EncryptedSize):
		return ReasonQuota
	case candidate.ObservedAt.IsZero() || request.Now.Sub(candidate.ObservedAt) > candidateFreshness || candidate.ObservedAt.After(request.Now.Add(5*time.Minute)):
		return "observation_stale"
	default:
		return ""
	}
}

func requiredCapacity(size int64) int64 {
	headroom := (size + 19) / 20
	if headroom < minimumCapacityHeadroom {
		headroom = minimumCapacityHeadroom
	}
	if size <= 0 || size > int64(^uint64(0)>>1)-headroom {
		return int64(^uint64(0) >> 1)
	}
	return size + headroom
}

func selectDiverse(candidates []Candidate, count int) []Candidate {
	if count <= 0 {
		return nil
	}
	selected := make([]Candidate, 0, count)
	usedDomain := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.FailureDomain != "" && usedDomain[candidate.FailureDomain] {
			continue
		}
		selected = append(selected, candidate)
		usedDomain[candidate.FailureDomain] = candidate.FailureDomain != ""
		if len(selected) == count {
			return selected
		}
	}
	for _, candidate := range candidates {
		if containsCandidate(selected, candidate.NodeID) {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) == count {
			break
		}
	}
	return selected
}

func containsCandidate(items []Candidate, nodeID string) bool {
	for _, item := range items {
		if item.NodeID == nodeID {
			return true
		}
	}
	return false
}
