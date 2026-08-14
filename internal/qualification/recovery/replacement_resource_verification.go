package recovery

import "time"

func verifyReplacementResources(cell replacementCell) Result {
	if len(cell.ResourceSamples) < 3 || cell.BaselineClientTraffic == 0 || cell.BaselinePublisherTraffic == 0 {
		return invalid("S4.2 paired baseline or one-second samples are incomplete")
	}
	if _, result := verifyResourceSamples(cell.ResourceSamples); result.Verdict != "pass" {
		return result
	}
	last := cell.ResourceSamples[len(cell.ResourceSamples)-1].AtNanos
	if last > cell.TerminalNanos || cell.TerminalNanos-last > int64(2*time.Second) {
		return invalid("S4.2 resource samples do not cover the terminal connection lifetime")
	}
	clientTraffic := cell.FinalTraffic.ClientReceived + cell.FinalTraffic.ClientSent
	publisherTraffic := cell.FinalTraffic.PublisherReceived + cell.FinalTraffic.PublisherSent
	if trafficExcess(clientTraffic, cell.BaselineClientTraffic) > recoveryTrafficAllowance ||
		trafficExcess(publisherTraffic, cell.BaselinePublisherTraffic) > recoveryTrafficAllowance {
		return fail("S4.2 recovery exceeded paired endpoint traffic allowance")
	}
	return Result{Verdict: "pass"}
}
