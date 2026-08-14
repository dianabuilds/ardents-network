package recovery

func verifyChannelEvidence(cell Cell, imageID string, hostScope hostScopeEvidence) Result {
	channel, err := verifyCommonChannelEvidence(cell.ChannelEvidence, cell, hostScope)
	if err != nil {
		return invalid(err.Error())
	}
	switch hostScope.Adapter {
	case "docker-compose-v1":
		if result := verifyCarrierEvidence(cell, imageID); result.Verdict != "pass" {
			return result
		}
		if err := validDockerChannelEvidence(channel, cell, hostScope); err != nil {
			return invalid(err.Error())
		}
	default:
		return invalid("Carrier channel Adapter is unsupported")
	}
	return Result{Verdict: "pass"}
}
