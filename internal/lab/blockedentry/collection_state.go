package blockedentry

func pristineObservers() []observer {
	result := make([]observer, 0, len(boundaries))
	for _, boundary := range boundaries {
		result = append(result, observer{Boundary: boundary, IPv4UDPControl: true, IPv6UDPControl: true,
			IPv4TCPControl: true, Attribution: "exact", ForbiddenOwner: "none", ObservationCompleted: true})
	}
	return result
}

func pristineCleanup() cleanupInventory {
	result := cleanupInventory{Complete: true}
	for _, kind := range residualKinds {
		result.Items = append(result.Items, residual{Kind: kind, Owner: "none"})
	}
	return result
}

func mergeObservers(aggregate, cell []observer, owner string, trustworthy bool) {
	if len(cell) != len(aggregate) {
		invalidateObservers(aggregate)
		return
	}
	for index := range aggregate {
		observed := cell[index]
		if observed.Boundary != aggregate[index].Boundary {
			invalidateObservers(aggregate)
			return
		}
		aggregate[index].IPv4UDPControl = aggregate[index].IPv4UDPControl && observed.IPv4UDPControl
		aggregate[index].IPv6UDPControl = aggregate[index].IPv6UDPControl && observed.IPv6UDPControl
		aggregate[index].IPv4TCPControl = aggregate[index].IPv4TCPControl && observed.IPv4TCPControl
		if observed.Attribution != "exact" {
			aggregate[index].Attribution = "ambiguous"
		}
		aggregate[index].ObservationCompleted = aggregate[index].ObservationCompleted && observed.ObservationCompleted
		aggregate[index].ForbiddenPackets += observed.ForbiddenPackets
		if observed.ForbiddenPackets > 0 {
			if !trustworthy {
				aggregate[index].Attribution = "ambiguous"
				aggregate[index].ForbiddenOwner = "harness"
				continue
			}
			if aggregate[index].ForbiddenOwner != "none" && aggregate[index].ForbiddenOwner != owner {
				aggregate[index].Attribution = "ambiguous"
			}
			aggregate[index].ForbiddenOwner = owner
		}
		aggregate[index].UnclassifiedPackets += observed.UnclassifiedPackets
	}
}

func invalidateObservers(values []observer) {
	for index := range values {
		values[index].Attribution = "ambiguous"
		values[index].ObservationCompleted = false
	}
}

func mergeResiduals(aggregate *cleanupInventory, cell []residual, owner, attributionHash string) {
	if len(cell) != len(aggregate.Items) {
		aggregate.Complete = false
		return
	}
	for index := range aggregate.Items {
		observed := cell[index]
		if observed.Kind != aggregate.Items[index].Kind {
			aggregate.Complete = false
			return
		}
		aggregate.Items[index].Count += observed.Count
		if observed.Count == 0 {
			continue
		}
		if aggregate.Items[index].Owner != "none" && aggregate.Items[index].Owner != owner {
			aggregate.Complete = false
			aggregate.Items[index].Owner = "harness"
		} else {
			aggregate.Items[index].Owner = owner
			aggregate.Items[index].AttributionEvidence = attributionHash
		}
	}
}
