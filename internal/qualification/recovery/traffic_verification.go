package recovery

import "time"

func verifyTrafficSamples(cell Cell) Result {
	if !terminalTrafficSample(cell.FinalTraffic, cell.TerminalAtNanos) ||
		cell.CarrierForwardBytes != max(cell.FinalTraffic.ClientSent, cell.FinalTraffic.PublisherSent) ||
		cell.CarrierReverseBytes != max(cell.FinalTraffic.ClientReceived, cell.FinalTraffic.PublisherReceived) {
		return invalid("final exact Route traffic sample is incomplete")
	}
	baseline := cell.BaselineFinalTraffic
	if cell.BaselineTerminalNanos <= 0 || !terminalTrafficSample(baseline, cell.BaselineTerminalNanos) ||
		cell.BaselineClientTraffic != baseline.ClientReceived+baseline.ClientSent ||
		cell.BaselinePublisherTraffic != baseline.PublisherReceived+baseline.PublisherSent {
		return invalid("paired baseline exact Route traffic sample is incomplete")
	}
	return Result{Verdict: "pass"}
}

func verifyReplacementTraffic(cell replacementCell) Result {
	if !terminalTrafficSample(cell.FinalTraffic, cell.TerminalNanos) {
		return invalid("S4.2 terminal exact Route traffic sample is incomplete")
	}
	return Result{Verdict: "pass"}
}

func terminalTrafficSample(value ResourceSample, terminalNanos int64) bool {
	return value.AtNanos > 0 && terminalNanos > 0 &&
		value.AtNanos <= terminalNanos+int64(2*time.Second) &&
		terminalNanos-value.AtNanos <= int64(2*time.Second) &&
		value.ClientReceived+value.ClientSent > 0 && value.PublisherReceived+value.PublisherSent > 0
}

func retainedContainerIdentities(cell Cell) map[string]bool {
	values := []string{cell.ClientProcess, cell.PublisherProcess, cell.ClientApplicationProcess,
		cell.PublisherApplicationProcess, cell.FaultContainer, cell.FaultController,
		cell.ReplacementObserver.ContainerID}
	for _, identity := range cell.InitialRouteContainers {
		values = append(values, identity)
	}
	result := make(map[string]bool, len(values))
	for _, identity := range values {
		result[identity] = true
	}
	return result
}
