package placement

import (
	"sort"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const candidateFreshness = 15 * time.Minute

const minimumCapacityHeadroom int64 = 64 * 1024

func SelectTargets(request SelectionRequest, candidates []Candidate) SelectionDecision {
	decision := SelectionDecision{}
	eligible := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		reason := candidateDenial(request, candidate)
		if reason != "" {
			decision.Denials = append(decision.Denials, Denial{NodePrincipal: candidate.NodePrincipal, Reason: reason})
			continue
		}
		eligible = append(eligible, candidate)
	}
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].FailureDomain != eligible[j].FailureDomain {
			return eligible[i].FailureDomain < eligible[j].FailureDomain
		}
		return eligible[i].NodePrincipal.String() < eligible[j].NodePrincipal.String()
	})
	decision.Selected = selectDiverse(eligible, request.Count)
	return decision
}

func candidateDenial(request SelectionRequest, candidate Candidate) string {
	switch {
	case candidate.NodePrincipal.String() == "" || candidate.NodePrincipal.Equal(request.OwnerPrincipal):
		return "node_ineligible"
	case request.ExcludedNodes[candidate.NodePrincipal]:
		return ReasonExisting
	case candidate.DenialReason != "":
		return candidate.DenialReason
	case !candidate.Trusted:
		return ReasonUntrusted
	case !candidate.PermissionValid:
		return ReasonPermission
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
	headroom := max((size+19)/20, minimumCapacityHeadroom)
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
		if containsCandidate(selected, candidate.NodePrincipal) {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) == count {
			break
		}
	}
	return selected
}

func containsCandidate(items []Candidate, principal identityprincipal.ID) bool {
	for _, item := range items {
		if item.NodePrincipal.Equal(principal) {
			return true
		}
	}
	return false
}
