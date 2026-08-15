package recovery

import "time"

func verifyReplacementResources(cell replacementCell) Result {
	if len(cell.ResourceSamples) < 3 {
		return invalid("S4.2 one-second samples are incomplete")
	}
	if _, result := verifyResourceSamples(cell.ResourceSamples); result.Verdict != "pass" {
		return result
	}
	first := cell.ResourceSamples[0].AtNanos
	if cell.ResourceStartedAtNanos <= 0 || first < cell.ResourceStartedAtNanos ||
		first-cell.ResourceStartedAtNanos > int64(1500*time.Millisecond) ||
		len(cell.Events) == 0 || cell.ResourceStartedAtNanos > cell.Events[0].LastDeliveryNanos {
		return invalid("S4.2 resource samples do not cover the workload start")
	}
	last := cell.ResourceSamples[len(cell.ResourceSamples)-1].AtNanos
	if last > cell.TerminalNanos || cell.TerminalNanos-last > int64(2*time.Second) {
		return invalid("S4.2 resource samples do not cover the terminal connection lifetime")
	}
	clientTraffic := cell.FinalTraffic.ClientReceived + cell.FinalTraffic.ClientSent
	publisherTraffic := cell.FinalTraffic.PublisherReceived + cell.FinalTraffic.PublisherSent
	if !replacementTrafficBound(clientTraffic, cell.Bytes) || !replacementTrafficBound(publisherTraffic, cell.Bytes) {
		return fail("S4.2 exact Route traffic is outside its workload allowance")
	}
	return Result{Verdict: "pass"}
}

func replacementTrafficBound(value uint64, bytes uint32) bool {
	return value > 0 && value <= uint64(bytes)+recoveryTrafficAllowance
}
