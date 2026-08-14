package recovery

import "time"

func verifyReplacementObservers(cell replacementCell, imageID string, identities map[string]bool) Result {
	pairs := []struct {
		observer ObserverProcess
		route    string
	}{
		{cell.BaselineClientTrafficObserver, cell.BaselineClientRoute},
		{cell.BaselinePublisherTrafficObserver, cell.BaselinePublisherRoute},
		{cell.ClientTrafficObserver, cell.ClientRoute},
		{cell.PublisherTrafficObserver, cell.PublisherRoute},
	}
	for _, route := range []string{cell.BaselineClientRoute, cell.BaselinePublisherRoute, cell.ClientRoute, cell.PublisherRoute} {
		if !fullContainerID(route) || identities[route] {
			return invalid("S4.2 traffic Route identity is incomplete or overlaps a candidate")
		}
		identities[route] = true
	}
	for _, pair := range pairs {
		if identities[pair.observer.ContainerID] || !confinedTrafficObserver(pair.observer, imageID, pair.route) {
			return invalid("S4.2 traffic observer confinement or separation is incomplete")
		}
		identities[pair.observer.ContainerID] = true
	}
	final := cell.FinalTraffic
	if final.AtNanos < cell.TerminalNanos || final.AtNanos > cell.TerminalNanos+int64(5*time.Second) ||
		final.ClientReceived+final.ClientSent == 0 || final.PublisherReceived+final.PublisherSent == 0 {
		return invalid("S4.2 terminal traffic snapshot is incomplete")
	}
	baseline := cell.BaselineFinalTraffic
	if cell.BaselineTerminalNanos <= 0 || baseline.AtNanos < cell.BaselineTerminalNanos ||
		baseline.AtNanos > cell.BaselineTerminalNanos+int64(5*time.Second) ||
		cell.BaselineClientTraffic != baseline.ClientReceived+baseline.ClientSent ||
		cell.BaselinePublisherTraffic != baseline.PublisherReceived+baseline.PublisherSent {
		return invalid("S4.2 paired baseline traffic snapshot is incomplete")
	}
	return Result{Verdict: "pass"}
}

func replacementCellIdentities(cell replacementCell) map[string]bool {
	result := make(map[string]bool, len(cell.HostProcesses)+16)
	for _, process := range cell.HostProcesses {
		result[process.Host.Identity] = true
	}
	for _, generation := range cell.Routes {
		for _, process := range generation.Processes {
			result[process.Host.Identity] = true
		}
	}
	for _, proposal := range cell.Proposals {
		for _, process := range proposal.Processes {
			result[process.Host.Identity] = true
		}
	}
	for _, identity := range []string{cell.BaselineClientRoute, cell.BaselinePublisherRoute, cell.ClientRoute, cell.PublisherRoute,
		cell.BaselineClientTrafficObserver.ContainerID, cell.BaselinePublisherTrafficObserver.ContainerID,
		cell.ClientTrafficObserver.ContainerID, cell.PublisherTrafficObserver.ContainerID} {
		result[identity] = true
	}
	return result
}
