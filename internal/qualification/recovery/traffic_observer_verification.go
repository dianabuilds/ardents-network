package recovery

import "time"

func verifyTrafficObservers(cell Cell, imageID string) Result {
	if !containerID(cell.BaselineClientRoute) || !containerID(cell.BaselinePublisherRoute) ||
		cell.BaselineClientRoute == cell.BaselinePublisherRoute ||
		cell.BaselineClientRoute == cell.InitialRouteContainers["client"] ||
		cell.BaselinePublisherRoute == cell.InitialRouteContainers["publisher"] {
		return invalid("paired baseline Route identity is incomplete")
	}
	pairs := []struct {
		observer ObserverProcess
		route    string
	}{
		{cell.BaselineClientTrafficObserver, cell.BaselineClientRoute},
		{cell.BaselinePublisherTrafficObserver, cell.BaselinePublisherRoute},
		{cell.ClientTrafficObserver, cell.InitialRouteContainers["client"]},
		{cell.PublisherTrafficObserver, cell.InitialRouteContainers["publisher"]},
	}
	identities := map[string]bool{cell.FaultController: true, cell.ReplacementObserver.ContainerID: true}
	for _, process := range []string{cell.ClientProcess, cell.PublisherProcess,
		cell.ClientApplicationProcess, cell.PublisherApplicationProcess} {
		identities[process] = true
	}
	for _, route := range cell.InitialRouteContainers {
		identities[route] = true
	}
	for _, route := range []string{cell.BaselineClientRoute, cell.BaselinePublisherRoute} {
		if identities[route] {
			return invalid("paired baseline Route identity overlaps retained process evidence")
		}
		identities[route] = true
	}
	for _, pair := range pairs {
		observer := pair.observer
		if identities[observer.ContainerID] || !confinedTrafficObserver(observer, imageID, pair.route) {
			return invalid("traffic observer confinement, separation, or cleanup is incomplete")
		}
		identities[observer.ContainerID] = true
	}
	final := cell.FinalTraffic
	if final.AtNanos < cell.TerminalAtNanos || final.AtNanos > cell.TerminalAtNanos+int64(5*time.Second) ||
		final.ClientReceived+final.ClientSent == 0 || final.PublisherReceived+final.PublisherSent == 0 ||
		cell.CarrierForwardBytes != max(final.ClientSent, final.PublisherSent) ||
		cell.CarrierReverseBytes != max(final.ClientReceived, final.PublisherReceived) {
		return invalid("final externally retained traffic snapshot is incomplete")
	}
	baseline := cell.BaselineFinalTraffic
	if cell.BaselineTerminalNanos <= 0 || baseline.AtNanos < cell.BaselineTerminalNanos ||
		baseline.AtNanos > cell.BaselineTerminalNanos+int64(5*time.Second) ||
		baseline.ClientReceived+baseline.ClientSent == 0 ||
		baseline.PublisherReceived+baseline.PublisherSent == 0 ||
		cell.BaselineClientTraffic != baseline.ClientReceived+baseline.ClientSent ||
		cell.BaselinePublisherTraffic != baseline.PublisherReceived+baseline.PublisherSent {
		return invalid("paired baseline final traffic snapshot is incomplete")
	}
	return Result{Verdict: "pass"}
}

func confinedTrafficObserver(value ObserverProcess, imageID, route string) bool {
	return containerID(value.ContainerID) && value.ImageID == imageID && value.NetworkMode == "container:"+route &&
		value.User == "65532:65532" && exactStrings(value.Command,
		[]string{"/usr/local/bin/ardents-qualify", "carrier-fault", "wait"}) &&
		len(value.CapAdd) == 0 && value.PIDMode == "" && value.IPCMode == "private" && !value.Privileged &&
		exactStrings(value.CapDrop, []string{"ALL"}) && exactStrings(value.SecurityOpt, []string{"no-new-privileges"}) &&
		value.ReadOnly && value.Removed && value.MountCount == 0 && value.PidsLimit == 16 &&
		value.MemoryLimit == 32<<20 && value.NanoCPUs == 250_000_000
}

func retainedContainerIdentities(cell Cell) map[string]bool {
	values := []string{cell.ClientProcess, cell.PublisherProcess, cell.ClientApplicationProcess,
		cell.PublisherApplicationProcess, cell.FaultContainer, cell.FaultController,
		cell.ReplacementObserver.ContainerID, cell.BaselineClientRoute, cell.BaselinePublisherRoute,
		cell.BaselineClientTrafficObserver.ContainerID, cell.BaselinePublisherTrafficObserver.ContainerID,
		cell.ClientTrafficObserver.ContainerID, cell.PublisherTrafficObserver.ContainerID}
	for _, identity := range cell.InitialRouteContainers {
		values = append(values, identity)
	}
	result := make(map[string]bool, len(values))
	for _, identity := range values {
		result[identity] = true
	}
	return result
}
