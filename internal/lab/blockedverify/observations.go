package blockedverify

func verifyObservers(observers []observer) (invalid, failures []string) {
	seen := make(map[string]bool, len(observers))
	for _, value := range observers {
		if seen[value.Boundary] {
			invalid = append(invalid, "observer boundary is duplicated")
			continue
		}
		seen[value.Boundary] = true
		if !value.IPv4UDPControl || !value.IPv6UDPControl || !value.IPv4TCPControl ||
			value.Attribution != "exact" || !value.ObservationCompleted {
			invalid = append(invalid, "observer control or attribution is incomplete: "+value.Boundary)
		}
		if value.UnclassifiedPackets != 0 {
			invalid = append(invalid, "observer recorded unclassified packets: "+value.Boundary)
		}
		if value.ForbiddenPackets != 0 && value.Attribution == "exact" && value.ObservationCompleted &&
			value.ForbiddenOwner == "candidate" {
			failures = append(failures, "observer recorded trustworthy forbidden packets: "+value.Boundary)
		} else if value.ForbiddenPackets != 0 {
			invalid = append(invalid, "observer recorded unattributed forbidden packets: "+value.Boundary)
		}
		if value.ForbiddenPackets == 0 && value.ForbiddenOwner != "none" {
			invalid = append(invalid, "observer has a forbidden owner without packets: "+value.Boundary)
		}
	}
	for _, boundary := range requiredBoundaries {
		if !seen[boundary] {
			invalid = append(invalid, "mandatory observer boundary is absent: "+boundary)
		}
	}
	if len(seen) != len(requiredBoundaries) {
		invalid = append(invalid, "observer boundary cardinality is invalid")
	}
	return invalid, failures
}

func verifyCleanup(cleanup cleanupInventory, attributions map[string]attributionFact) (invalid, failures []string) {
	if !cleanup.Complete {
		invalid = append(invalid, "cleanup inventory is incomplete")
	}
	seen := make(map[string]bool, len(cleanup.Items))
	for _, item := range cleanup.Items {
		if seen[item.Kind] || item.Owner != "none" && item.Owner != "candidate" && item.Owner != "harness" {
			invalid = append(invalid, "cleanup inventory identity or owner is ambiguous")
			continue
		}
		seen[item.Kind] = true
		owner, attributed := ownerForCommitment(attributions, item.AttributionEvidence)
		if item.Count > 0 && (!attributed || item.Owner != owner) {
			invalid = append(invalid, "residual ownership is not committed: "+item.Kind)
		} else if item.Count > 0 && owner == "candidate" {
			failures = append(failures, "candidate-owned residual: "+item.Kind)
		} else if item.Count > 0 {
			invalid = append(invalid, "harness-owned or unattributed residual: "+item.Kind)
		} else if item.AttributionEvidence != "" {
			invalid = append(invalid, "zero residual has attribution evidence: "+item.Kind)
		}
	}
	for _, kind := range requiredResiduals {
		if !seen[kind] {
			invalid = append(invalid, "cleanup inventory omits: "+kind)
		}
	}
	if len(seen) != len(requiredResiduals) {
		invalid = append(invalid, "cleanup inventory cardinality is invalid")
	}
	return invalid, failures
}

func ownerForCommitment(attributions map[string]attributionFact, commitment string) (string, bool) {
	for _, fact := range attributions {
		if fact.commitment == commitment {
			return fact.owner, true
		}
	}
	return "", false
}
